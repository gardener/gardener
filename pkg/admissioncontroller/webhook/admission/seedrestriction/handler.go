// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package seedrestriction

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-logr/logr"
	admissionv1 "k8s.io/api/admission/v1"
	certificatesv1 "k8s.io/api/certificates/v1"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	bootstraptokenapi "k8s.io/cluster-bootstrap/token/api"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/gardener/gardener/pkg/admissioncontroller/seedidentity"
	admissionwebhook "github.com/gardener/gardener/pkg/admissioncontroller/webhook/admission"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	operationsv1alpha1 "github.com/gardener/gardener/pkg/apis/operations/v1alpha1"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	seedmanagementv1alpha1helper "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1/helper"
	gardenletbootstraputil "github.com/gardener/gardener/pkg/gardenlet/bootstrap/util"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

var (
	// Only take v1beta1 for the core.gardener.cloud API group because the Authorize function only checks the resource
	// group and the resource (but it ignores the version).
	backupBucketResource              = gardencorev1beta1.Resource("backupbuckets")
	backupEntryResource               = gardencorev1beta1.Resource("backupentries")
	bastionResource                   = operationsv1alpha1.Resource("bastions")
	certificateSigningRequestResource = certificatesv1.Resource("certificatesigningrequests")
	clusterRoleBindingResource        = rbacv1.Resource("clusterrolebindings")
	gardenletResource                 = seedmanagementv1alpha1.Resource("gardenlets")
	internalSecretResource            = gardencorev1beta1.Resource("internalsecrets")
	leaseResource                     = coordinationv1.Resource("leases")
	secretResource                    = corev1.Resource("secrets")
	configMapResource                 = corev1.Resource("configmaps")
	seedResource                      = gardencorev1beta1.Resource("seeds")
	serviceAccountResource            = corev1.Resource("serviceaccounts")
	shootStateResource                = gardencorev1beta1.Resource("shootstates")
)

// Handler restricts requests made by gardenlets.
type Handler struct {
	Logger  logr.Logger
	Client  client.Reader
	Decoder admission.Decoder
}

// Handle restricts requests made by gardenlets.
func (h *Handler) Handle(ctx context.Context, request admission.Request) admission.Response {
	seedName, isSeed, userType := seedidentity.FromAuthenticationV1UserInfo(request.UserInfo)
	if !isSeed {
		return admissionwebhook.Allowed("")
	}

	requestResource := schema.GroupResource{Group: request.Resource.Group, Resource: request.Resource.Resource}
	switch requestResource {
	case backupBucketResource:
		return h.admitBackupBucket(ctx, seedName, request)
	case backupEntryResource:
		return h.admitBackupEntry(ctx, seedName, request)
	case bastionResource:
		return h.admitBastion(seedName, request)
	case certificateSigningRequestResource:
		return h.admitCertificateSigningRequest(seedName, userType, request)
	case clusterRoleBindingResource:
		return h.admitClusterRoleBinding(ctx, seedName, userType, request)
	case configMapResource:
		return h.admitConfigMap(ctx, seedName, request)
	case internalSecretResource:
		return h.admitInternalSecret(ctx, seedName, request)
	case gardenletResource:
		return h.admitGardenlet(seedName, request)
	case leaseResource:
		return h.admitLease(seedName, userType, request)
	case secretResource:
		return h.admitSecret(ctx, seedName, request)
	case seedResource:
		return h.admitSeed(ctx, seedName, request)
	case serviceAccountResource:
		return h.admitServiceAccount(ctx, seedName, userType, request)
	case shootStateResource:
		return h.admitShootState(ctx, seedName, request)
	}

	return admissionwebhook.Allowed("")
}

func (h *Handler) admitBackupBucket(ctx context.Context, seedName string, request admission.Request) admission.Response {
	switch request.Operation {
	case admissionv1.Create:
		// If a gardenlet tries to create a BackupBucket then the request may only be allowed if the used `.spec.seedName`
		// is equal to the gardenlet's seed.
		backupBucket := &gardencorev1beta1.BackupBucket{}
		if err := h.Decoder.Decode(request, backupBucket); err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}
		return h.admit(seedName, backupBucket.Spec.SeedName)

	case admissionv1.Delete:
		// If a gardenlet tries to delete a BackupBucket then it may only be allowed if the name is equal to the UID of
		// the gardenlet's seed.
		seed := &gardencorev1beta1.Seed{}
		if err := h.Client.Get(ctx, client.ObjectKey{Name: seedName}, seed); err != nil {
			return admission.Errored(http.StatusInternalServerError, err)
		}
		if string(seed.UID) != request.Name {
			return admission.Errored(http.StatusForbidden, errors.New("cannot delete unrelated BackupBucket"))
		}
		return admission.Allowed("")
	}

	return admission.Errored(http.StatusBadRequest, fmt.Errorf("unexpected operation: %q", request.Operation))
}

func (h *Handler) admitBackupEntry(ctx context.Context, seedName string, request admission.Request) admission.Response {
	if request.Operation != admissionv1.Create {
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("unexpected operation: %q", request.Operation))
	}

	backupEntry := &gardencorev1beta1.BackupEntry{}
	if err := h.Decoder.Decode(request, backupEntry); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	if resp := h.admit(seedName, backupEntry.Spec.SeedName); !resp.Allowed {
		return resp
	}

	if strings.HasPrefix(backupEntry.Name, v1beta1constants.BackupSourcePrefix) {
		return h.admitSourceBackupEntry(ctx, backupEntry)
	}

	backupBucket := &gardencorev1beta1.BackupBucket{}
	if err := h.Client.Get(ctx, client.ObjectKey{Name: backupEntry.Spec.BucketName}, backupBucket); err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	return h.admit(seedName, backupBucket.Spec.SeedName)
}

func (h *Handler) admitSourceBackupEntry(ctx context.Context, backupEntry *gardencorev1beta1.BackupEntry) admission.Response {
	// The source BackupEntry is created during the restore phase of control plane migration
	// so allow creations only if the shoot that owns the BackupEntry is currently being restored.
	shootName := gardenerutils.GetShootNameFromOwnerReferences(backupEntry)
	shoot := &gardencorev1beta1.Shoot{}
	if err := h.Client.Get(ctx, client.ObjectKey{Namespace: backupEntry.Namespace, Name: shootName}, shoot); err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	if shoot.Status.LastOperation == nil || shoot.Status.LastOperation.Type != gardencorev1beta1.LastOperationTypeRestore ||
		shoot.Status.LastOperation.State != gardencorev1beta1.LastOperationStateProcessing {
		return admission.Errored(http.StatusForbidden, fmt.Errorf("creation of source BackupEntry is only allowed during shoot Restore operation (shoot: %s)", shootName))
	}

	// When the source BackupEntry is created it's spec is the same as that of the shoot's original BackupEntry.
	// The original BackupEntry is modified after the source BackupEntry has been deployed and successfully reconciled.
	shootBackupEntryName := strings.TrimPrefix(backupEntry.Name, fmt.Sprintf("%s-", v1beta1constants.BackupSourcePrefix))
	shootBackupEntry := &gardencorev1beta1.BackupEntry{}
	if err := h.Client.Get(ctx, client.ObjectKey{Namespace: backupEntry.Namespace, Name: shootBackupEntryName}, shootBackupEntry); err != nil {
		if apierrors.IsNotFound(err) {
			return admission.Errored(http.StatusForbidden, fmt.Errorf("could not find original BackupEntry %s: %w", shootBackupEntryName, err))
		}
		return admission.Errored(http.StatusInternalServerError, err)
	}

	if !apiequality.Semantic.DeepEqual(backupEntry.Spec, shootBackupEntry.Spec) {
		return admission.Errored(http.StatusForbidden, fmt.Errorf("specification of source BackupEntry must equal specification of original BackupEntry %s", shootBackupEntryName))
	}

	return admission.Allowed("")
}

func (h *Handler) admitBastion(seedName string, request admission.Request) admission.Response {
	if request.Operation != admissionv1.Create {
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("unexpected operation: %q", request.Operation))
	}

	bastion := &operationsv1alpha1.Bastion{}
	if err := h.Decoder.Decode(request, bastion); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	return h.admit(seedName, bastion.Spec.SeedName)
}

func (h *Handler) admitCertificateSigningRequest(seedName string, userType seedidentity.UserType, request admission.Request) admission.Response {
	if request.Operation != admissionv1.Create {
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("unexpected operation: %q", request.Operation))
	}

	if userType == seedidentity.UserTypeExtension {
		return admission.Errored(http.StatusForbidden, errors.New("extension client may not create CertificateSigningRequests"))
	}

	csr := &certificatesv1.CertificateSigningRequest{}
	if err := h.Decoder.Decode(request, csr); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	x509cr, err := utils.DecodeCertificateRequest(csr.Spec.Request)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	if ok, reason := gardenerutils.IsSeedClientCert(x509cr, csr.Spec.Usages); !ok {
		return admission.Errored(http.StatusForbidden, fmt.Errorf("can only create CSRs for seed clusters: %s", reason))
	}

	seedNameInCSR, _, _ := seedidentity.FromCertificateSigningRequest(x509cr)
	return h.admit(seedName, &seedNameInCSR)
}

func (h *Handler) admitClusterRoleBinding(ctx context.Context, seedName string, userType seedidentity.UserType, request admission.Request) admission.Response {
	if request.Operation != admissionv1.Create {
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("unexpected operation: %q", request.Operation))
	}

	if userType == seedidentity.UserTypeExtension {
		return admission.Errored(http.StatusForbidden, fmt.Errorf("extension client may not create ClusterRoleBindings"))
	}

	// Allow gardenlet to create cluster role bindings referencing service accounts which can be used to bootstrap other
	// gardenlets deployed as part of the ManagedSeed reconciliation.
	if strings.HasPrefix(request.Name, gardenletbootstraputil.ClusterRoleBindingNamePrefix) {
		clusterRoleBinding := &rbacv1.ClusterRoleBinding{}
		if err := h.Decoder.Decode(request, clusterRoleBinding); err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}

		if clusterRoleBinding.RoleRef.APIGroup != rbacv1.GroupName ||
			clusterRoleBinding.RoleRef.Kind != "ClusterRole" ||
			clusterRoleBinding.RoleRef.Name != gardenletbootstraputil.GardenerSeedBootstrapper {
			return admission.Errored(http.StatusForbidden, fmt.Errorf("can only bindings referring to the bootstrapper role"))
		}

		managedSeedNamespace, managedSeedName := gardenletbootstraputil.ManagedSeedInfoFromClusterRoleBindingName(request.Name)
		return h.allowIfManagedSeedIsNotYetBootstrapped(ctx, seedName, managedSeedNamespace, managedSeedName)
	}

	return admission.Errored(http.StatusForbidden, fmt.Errorf("object does not belong to seed %q", seedName))
}

func (h *Handler) admitGardenlet(seedName string, request admission.Request) admission.Response {
	if request.Operation != admissionv1.Create {
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("unexpected operation: %q", request.Operation))
	}

	if request.Namespace != v1beta1constants.GardenNamespace {
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("object must be in namespace: %q", v1beta1constants.GardenNamespace))
	}

	return h.admit(seedName, &request.Name)
}

func (h *Handler) admitInternalSecret(ctx context.Context, seedName string, request admission.Request) admission.Response {
	if request.Operation != admissionv1.Create {
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("unexpected operation: %q", request.Operation))
	}

	// Check if the internal secret is related to a Shoot assigned to the seed the gardenlet is responsible for.
	if shootName, ok := gardenerutils.IsShootProjectInternalSecret(request.Name); ok {
		shoot := &gardencorev1beta1.Shoot{}
		if err := h.Client.Get(ctx, client.ObjectKey{Namespace: request.Namespace, Name: shootName}, shoot); err != nil {
			if apierrors.IsNotFound(err) {
				return admission.Errored(http.StatusForbidden, err)
			}
			return admission.Errored(http.StatusInternalServerError, err)
		}

		return h.admit(seedName, shoot.Spec.SeedName)
	}

	return admission.Errored(http.StatusForbidden, fmt.Errorf("object does not belong to seed %q", seedName))
}

func (h *Handler) admitLease(seedName string, userType seedidentity.UserType, request admission.Request) admission.Response {
	if request.Operation != admissionv1.Create {
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("unexpected operation: %q", request.Operation))
	}

	// extension clients may only work with leases in the seed namespace
	if userType == seedidentity.UserTypeExtension {
		if request.Namespace == gardenerutils.ComputeGardenNamespace(seedName) {
			return admission.Allowed("")
		}
		return admission.Errored(http.StatusForbidden, fmt.Errorf("extension client can only create leases in the namespace for seed %q", seedName))
	}

	// This allows the gardenlet to create a Lease for leader election (if the garden cluster is a seed as well).
	if request.Name == "gardenlet-leader-election" {
		return admission.Allowed("")
	}

	// Each gardenlet creates a Lease with the name of its own seed in the `gardener-system-seed-lease` namespace.
	if request.Namespace == gardencorev1beta1.GardenerSeedLeaseNamespace {
		return h.admit(seedName, &request.Name)
	}

	return admission.Errored(http.StatusForbidden, fmt.Errorf("object does not belong to seed %q", seedName))
}

func (h *Handler) admitSecret(ctx context.Context, seedName string, request admission.Request) admission.Response {
	if request.Operation != admissionv1.Create {
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("unexpected operation: %q", request.Operation))
	}

	// Check if the secret is related to a BackupBucket assigned to the seed the gardenlet is responsible for.
	if strings.HasPrefix(request.Name, v1beta1constants.SecretPrefixGeneratedBackupBucket) {
		backupBucket := &gardencorev1beta1.BackupBucket{}
		if err := h.Client.Get(ctx, client.ObjectKey{Name: strings.TrimPrefix(request.Name, v1beta1constants.SecretPrefixGeneratedBackupBucket)}, backupBucket); err != nil {
			if apierrors.IsNotFound(err) {
				return admission.Errored(http.StatusForbidden, err)
			}
			return admission.Errored(http.StatusInternalServerError, err)
		}

		return h.admit(seedName, backupBucket.Spec.SeedName)
	}

	// Check if the secret is related to a Shoot assigned to the seed the gardenlet is responsible for.
	if shootName, ok := gardenerutils.IsShootProjectSecret(request.Name); ok {
		shoot := &gardencorev1beta1.Shoot{}
		if err := h.Client.Get(ctx, client.ObjectKey{Namespace: request.Namespace, Name: shootName}, shoot); err != nil {
			if apierrors.IsNotFound(err) {
				return admission.Errored(http.StatusForbidden, err)
			}
			return admission.Errored(http.StatusInternalServerError, err)
		}

		return h.admit(seedName, shoot.Spec.SeedName)
	}

	// Gardenlets can create secrets that contain the public info of a shoot's
	// service account issuer in the gardener-system-shoot-issuer namespace.
	if request.Namespace == gardencorev1beta1.GardenerShootIssuerNamespace {
		secret := &corev1.Secret{}
		if err := h.Decoder.Decode(request, secret); err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}

		var (
			shootName      string
			shootNamespace string
			ok             bool
		)
		if shootName, ok = secret.Labels[v1beta1constants.LabelShootName]; !ok {
			return admission.Errored(http.StatusUnprocessableEntity, fmt.Errorf("label %q is missing", v1beta1constants.LabelShootName))
		}
		if shootNamespace, ok = secret.Labels[v1beta1constants.LabelShootNamespace]; !ok {
			return admission.Errored(http.StatusUnprocessableEntity, fmt.Errorf("label %q is missing", v1beta1constants.LabelShootNamespace))
		}
		if publicKeyType, ok := secret.Labels[v1beta1constants.LabelPublicKeys]; !ok {
			return admission.Errored(http.StatusUnprocessableEntity, fmt.Errorf("label %q is missing", v1beta1constants.LabelPublicKeys))
		} else if publicKeyType != v1beta1constants.LabelPublicKeysServiceAccount {
			return admission.Errored(http.StatusUnprocessableEntity, fmt.Errorf("label %q value must be set to %q", v1beta1constants.LabelPublicKeys, v1beta1constants.LabelPublicKeysServiceAccount))
		}

		shoot := &gardencorev1beta1.Shoot{ObjectMeta: metav1.ObjectMeta{Name: shootName, Namespace: shootNamespace}}
		if err := h.Client.Get(ctx, client.ObjectKeyFromObject(shoot), shoot); err != nil {
			if apierrors.IsNotFound(err) {
				return admission.Errored(http.StatusForbidden, err)
			}
			return admission.Errored(http.StatusInternalServerError, err)
		}

		return h.admit(seedName, shoot.Spec.SeedName)
	}

	// Check if the secret is a bootstrap token for a ManagedSeed or a Gardenlet.
	if strings.HasPrefix(request.Name, bootstraptokenapi.BootstrapTokenSecretPrefix) && request.Namespace == metav1.NamespaceSystem {
		secret := &corev1.Secret{}
		if err := h.Decoder.Decode(request, secret); err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}

		if secret.Type != corev1.SecretTypeBootstrapToken {
			return admission.Errored(http.StatusUnprocessableEntity, fmt.Errorf("unexpected secret type: %q", secret.Type))
		}
		if string(secret.Data[bootstraptokenapi.BootstrapTokenUsageAuthentication]) != "true" {
			return admission.Errored(http.StatusUnprocessableEntity, fmt.Errorf("%q must be set to 'true'", bootstraptokenapi.BootstrapTokenUsageAuthentication))
		}
		if string(secret.Data[bootstraptokenapi.BootstrapTokenUsageSigningKey]) != "true" {
			return admission.Errored(http.StatusUnprocessableEntity, fmt.Errorf("%q must be set to 'true'", bootstraptokenapi.BootstrapTokenUsageSigningKey))
		}
		if _, ok := secret.Data[bootstraptokenapi.BootstrapTokenExtraGroupsKey]; ok {
			return admission.Errored(http.StatusUnprocessableEntity, fmt.Errorf("%q must not be set", bootstraptokenapi.BootstrapTokenExtraGroupsKey))
		}

		kind, namespace, name := gardenletbootstraputil.MetadataFromDescription(string(secret.Data[bootstraptokenapi.BootstrapTokenDescriptionKey]))
		switch kind {
		case gardenletbootstraputil.KindManagedSeed:
			return h.allowIfManagedSeedIsNotYetBootstrapped(ctx, seedName, namespace, name)
		case gardenletbootstraputil.KindGardenlet:
			return h.allowIfGardenletIsNotYetBootstrapped(ctx, namespace, name)
		default:
			return admission.Errored(http.StatusBadRequest, fmt.Errorf("unknown kind %q found in bootstrap token secret description %q", kind, secret.Data[bootstraptokenapi.BootstrapTokenDescriptionKey]))
		}
	}

	// Check if the secret is related to a ManagedSeed assigned to the seed the gardenlet is responsible for.
	managedSeedList := &seedmanagementv1alpha1.ManagedSeedList{}
	if err := h.Client.List(ctx, managedSeedList); err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	for _, managedSeed := range managedSeedList.Items {
		shoot := &gardencorev1beta1.Shoot{}
		if err := h.Client.Get(ctx, client.ObjectKey{Namespace: managedSeed.Namespace, Name: managedSeed.Spec.Shoot.Name}, shoot); err != nil {
			return admission.Errored(http.StatusInternalServerError, err)
		}

		if !h.admit(seedName, shoot.Spec.SeedName).Allowed {
			continue
		}

		seedTemplate, _, err := seedmanagementv1alpha1helper.ExtractSeedTemplateAndGardenletConfig(managedSeed.Name, &managedSeed.Spec.Gardenlet.Config)
		if err != nil {
			return admission.Errored(http.StatusInternalServerError, err)
		}

		if seedTemplate.Spec.Backup != nil &&
			seedTemplate.Spec.Backup.CredentialsRef.APIVersion == "v1" &&
			seedTemplate.Spec.Backup.CredentialsRef.Kind == "Secret" &&
			seedTemplate.Spec.Backup.CredentialsRef.Namespace == request.Namespace &&
			seedTemplate.Spec.Backup.CredentialsRef.Name == request.Name {
			return admission.Allowed("")
		}
	}

	return admission.Errored(http.StatusForbidden, fmt.Errorf("object does not belong to seed %q", seedName))
}

func (h *Handler) admitConfigMap(ctx context.Context, seedName string, request admission.Request) admission.Response {
	if request.Operation != admissionv1.Create {
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("unexpected operation: %q", request.Operation))
	}

	// Check if the config map is related to a Shoot assigned to the seed the gardenlet is responsible for.
	if shootName, ok := gardenerutils.IsShootProjectConfigMap(request.Name); ok {
		shoot := &gardencorev1beta1.Shoot{}
		if err := h.Client.Get(ctx, client.ObjectKey{Namespace: request.Namespace, Name: shootName}, shoot); err != nil {
			if apierrors.IsNotFound(err) {
				return admission.Errored(http.StatusForbidden, err)
			}
			return admission.Errored(http.StatusInternalServerError, err)
		}

		return h.admit(seedName, shoot.Spec.SeedName)
	}

	return admission.Errored(http.StatusForbidden, fmt.Errorf("object does not belong to seed %q", seedName))
}

func (h *Handler) admitSeed(ctx context.Context, seedName string, request admission.Request) admission.Response {
	response := h.admit(seedName, &request.Name)
	if request.Operation == admissionv1.Delete && !response.Allowed {
		// If the deletion request is not allowed, then it might be submitted by the "parent gardenlet".
		// This is the gardenlet/seed which is responsible for the `managedseed` in question.
		managedSeed := &seedmanagementv1alpha1.ManagedSeed{}
		if err := h.Client.Get(ctx, client.ObjectKey{Namespace: v1beta1constants.GardenNamespace, Name: request.Name}, managedSeed); err != nil {
			if apierrors.IsNotFound(err) {
				return response
			}
			return admission.Errored(http.StatusInternalServerError, err)
		}

		// If a gardenlet tries to delete a Seed belonging to a ManagedSeed then the request may only be considered
		// further if the `.spec.deletionTimestamp` is set (gardenlets themselves are not allowed to delete ManagedSeeds,
		// so it's safe to only continue if somebody else has set this deletion timestamp).
		if managedSeed.DeletionTimestamp == nil {
			return admission.Errored(http.StatusForbidden, fmt.Errorf("object can only be deleted if corresponding ManagedSeed has a deletion timestamp"))
		}

		// If for whatever reason the `.spec.shoot` is nil then we exit early.
		if managedSeed.Spec.Shoot == nil {
			return response
		}

		// Check if the `.spec.seedName` of the Shoot referenced in the `.spec.shoot.name` field of the ManagedSeed matches
		// the seed name of the requesting gardenlet.
		shoot := &gardencorev1beta1.Shoot{}
		if err := h.Client.Get(ctx, client.ObjectKey{Namespace: managedSeed.Namespace, Name: managedSeed.Spec.Shoot.Name}, shoot); err != nil {
			return admission.Errored(http.StatusInternalServerError, err)
		}

		return h.admit(seedName, shoot.Spec.SeedName)
	}

	return response
}

func (h *Handler) admitServiceAccount(ctx context.Context, seedName string, userType seedidentity.UserType, request admission.Request) admission.Response {
	if request.Operation != admissionv1.Create {
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("unexpected operation: %q", request.Operation))
	}

	if userType == seedidentity.UserTypeExtension {
		return admission.Errored(http.StatusForbidden, fmt.Errorf("extension client may not create ServiceAccounts"))
	}

	// Allow gardenlet to create service accounts which can be used to bootstrap other gardenlets deployed as part of
	// the ManagedSeed reconciliation.
	if strings.HasPrefix(request.Name, gardenletbootstraputil.ServiceAccountNamePrefix) {
		return h.allowIfManagedSeedIsNotYetBootstrapped(ctx, seedName, request.Namespace, strings.TrimPrefix(request.Name, gardenletbootstraputil.ServiceAccountNamePrefix))
	}

	// Allow all verbs for service accounts in gardenlets' seed-<name> namespaces.
	if request.Namespace == gardenerutils.ComputeGardenNamespace(seedName) {
		return admission.Allowed("")
	}

	return admission.Errored(http.StatusForbidden, fmt.Errorf("object does not belong to seed %q", seedName))
}

func (h *Handler) admitShootState(ctx context.Context, seedName string, request admission.Request) admission.Response {
	if request.Operation != admissionv1.Create {
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("unexpected operation: %q", request.Operation))
	}

	shoot := &gardencorev1beta1.Shoot{}
	if err := h.Client.Get(ctx, client.ObjectKey{Namespace: request.Namespace, Name: request.Name}, shoot); err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	return h.admit(seedName, shoot.Spec.SeedName, shoot.Status.SeedName)
}

func (h *Handler) admit(seedName string, seedNamesForObject ...*string) admission.Response {
	// Allow request if one of the seed names for the object matches the seed name of the requesting user.
	for _, seedNameForObject := range seedNamesForObject {
		if seedNameForObject != nil && *seedNameForObject == seedName {
			return admission.Allowed("")
		}
	}

	return admission.Errored(http.StatusForbidden, fmt.Errorf("object does not belong to seed %q", seedName))
}

func (h *Handler) allowIfManagedSeedIsNotYetBootstrapped(ctx context.Context, seedName, managedSeedNamespace, managedSeedName string) admission.Response {
	managedSeed := &seedmanagementv1alpha1.ManagedSeed{}
	if err := h.Client.Get(ctx, client.ObjectKey{Namespace: managedSeedNamespace, Name: managedSeedName}, managedSeed); err != nil {
		if apierrors.IsNotFound(err) {
			return admission.Errored(http.StatusForbidden, err)
		}
		return admission.Errored(http.StatusInternalServerError, err)
	}

	shoot := &gardencorev1beta1.Shoot{}
	if err := h.Client.Get(ctx, client.ObjectKey{Namespace: managedSeed.Namespace, Name: managedSeed.Spec.Shoot.Name}, shoot); err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	if response := h.admit(seedName, shoot.Spec.SeedName); !response.Allowed {
		return response
	}

	seed := &gardencorev1beta1.Seed{}
	if err := h.Client.Get(ctx, client.ObjectKey{Name: managedSeedName}, seed); err != nil {
		if !apierrors.IsNotFound(err) {
			return admission.Errored(http.StatusInternalServerError, err)
		}
		return admission.Allowed("")
	} else if seed.Status.ClientCertificateExpirationTimestamp != nil && seed.Status.ClientCertificateExpirationTimestamp.UTC().Before(time.Now().UTC()) {
		return admission.Allowed("")
	} else if managedSeed.Annotations[v1beta1constants.GardenerOperation] == v1beta1constants.GardenerOperationRenewKubeconfig {
		return admission.Allowed("")
	}

	return admission.Errored(http.StatusBadRequest, fmt.Errorf("managed seed %s/%s is already bootstrapped", managedSeed.Namespace, managedSeed.Name))
}

func (h *Handler) allowIfGardenletIsNotYetBootstrapped(ctx context.Context, gardenletNamespace, gardenletName string) admission.Response {
	gardenlet := &seedmanagementv1alpha1.Gardenlet{}
	if err := h.Client.Get(ctx, client.ObjectKey{Namespace: gardenletNamespace, Name: gardenletName}, gardenlet); err != nil {
		if apierrors.IsNotFound(err) {
			return admission.Errored(http.StatusForbidden, err)
		}
		return admission.Errored(http.StatusInternalServerError, err)
	}

	seed := &gardencorev1beta1.Seed{}
	if err := h.Client.Get(ctx, client.ObjectKey{Name: gardenletName}, seed); err != nil {
		if !apierrors.IsNotFound(err) {
			return admission.Errored(http.StatusInternalServerError, err)
		}
		return admission.Errored(http.StatusForbidden, err)
	} else if seed.Status.ClientCertificateExpirationTimestamp != nil && seed.Status.ClientCertificateExpirationTimestamp.UTC().Before(time.Now().UTC()) {
		return admission.Allowed("")
	} else if gardenlet.Annotations[v1beta1constants.GardenerOperation] == v1beta1constants.GardenerOperationRenewKubeconfig {
		return admission.Allowed("")
	}

	return admission.Errored(http.StatusBadRequest, fmt.Errorf("gardenlet %s/%s is already bootstrapped", gardenlet.Namespace, gardenlet.Name))
}
