// SPDX-FileCopyrightText: Contributors to the Gardener project
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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	bootstraptokenapi "k8s.io/cluster-bootstrap/token/api"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/gardener/gardener/pkg/admissioncontroller/gardenletidentity"
	seedidentity "github.com/gardener/gardener/pkg/admissioncontroller/gardenletidentity/seed"
	admissionwebhook "github.com/gardener/gardener/pkg/admissioncontroller/webhook/admission"
	seedmanagementv1alpha1helper "github.com/gardener/gardener/pkg/api/seedmanagement/v1alpha1/helper"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	operationsv1alpha1 "github.com/gardener/gardener/pkg/apis/operations/v1alpha1"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
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
	controllerinstallationResource    = gardencorev1beta1.Resource("controllerinstallations")
	gardenletResource                 = seedmanagementv1alpha1.Resource("gardenlets")
	internalSecretResource            = gardencorev1beta1.Resource("internalsecrets")
	leaseResource                     = coordinationv1.Resource("leases")
	managedSeedResource               = seedmanagementv1alpha1.Resource("managedseeds")
	secretResource                    = corev1.Resource("secrets")
	configMapResource                 = corev1.Resource("configmaps")
	seedResource                      = gardencorev1beta1.Resource("seeds")
	serviceAccountResource            = corev1.Resource("serviceaccounts")
	shootResource                     = gardencorev1beta1.Resource("shoots")
	shootStateResource                = gardencorev1beta1.Resource("shootstates")
)

// Handler restricts requests made by seed gardenlets.
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

	log := h.Logger.WithValues("seedName", seedName, "userType", userType)

	// For CREATE operations, if the object already exists, allow the request since the CREATE must have already been processed by this code previously.
	// This avoids re-validating the object against CREATE-specific logic that may no longer apply to the current state of the object.
	if request.Operation == admissionv1.Create {
		obj := &unstructured.Unstructured{}
		obj.SetGroupVersionKind(schema.GroupVersionKind{Group: request.Kind.Group, Version: request.Kind.Version, Kind: request.Kind.Kind})

		if err := h.Client.Get(ctx, client.ObjectKey{Name: request.Name, Namespace: request.Namespace}, obj); err == nil {
			return admissionwebhook.Allowed("object already exists")
		} else if !apierrors.IsNotFound(err) {
			log.Error(err, "Failed to get object, continuing with normal admission checks", "requestName", request.Name, "requestNamespace", request.Namespace, "requestGroup", request.Kind.Group, "requestVersion", request.Kind.Version, "requestKind", request.Kind.Kind)
		}
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
	case controllerinstallationResource:
		return h.admitControllerInstallation(seedName, request)
	case internalSecretResource:
		return h.admitInternalSecret(ctx, seedName, request)
	case gardenletResource:
		return h.admitGardenlet(ctx, seedName, request)
	case leaseResource:
		return h.admitLease(seedName, userType, request)
	case managedSeedResource:
		return h.admitManagedSeed(seedName, request)
	case secretResource:
		return h.admitSecret(ctx, seedName, request)
	case seedResource:
		return h.admitSeed(ctx, seedName, request)
	case shootResource:
		return h.admitShoot(seedName, request)
	case serviceAccountResource:
		return h.admitServiceAccount(ctx, seedName, userType, request)
	case shootStateResource:
		return h.admitShootState(ctx, seedName, request)
	default:
		log.Info(
			"Unhandled resource request",
			"group", request.Kind.Group,
			"version", request.Kind.Version,
			"resource", request.Resource.Resource,
		)
	}

	return admission.Errored(http.StatusBadRequest, fmt.Errorf("unexpected resource: %q", requestResource))
}

func (h *Handler) admitBackupBucket(ctx context.Context, seedName string, request admission.Request) admission.Response {
	switch request.Operation {
	case admissionv1.Update:
		oldBucket := &gardencorev1beta1.BackupBucket{}
		if err := h.Decoder.DecodeRaw(request.OldObject, oldBucket); err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}
		newBucket := &gardencorev1beta1.BackupBucket{}
		if err := h.Decoder.Decode(request, newBucket); err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}
		if !apiequality.Semantic.DeepEqual(oldBucket.Spec, newBucket.Spec) {
			return admission.Errored(http.StatusForbidden, errors.New("gardenlet must not modify .spec of BackupBucket"))
		}
		return admission.Allowed("")

	case admissionv1.Create:
		// If a gardenlet tries to create a BackupBucket then the request may only be allowed if the used `.spec.seedName`
		// is equal to the gardenlet's seed, and the `.spec` matches the backup configuration of the gardenlet's seed.
		backupBucket := &gardencorev1beta1.BackupBucket{}
		if err := h.Decoder.Decode(request, backupBucket); err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}

		if resp := h.admit(seedName, backupBucket.Spec.SeedName); !resp.Allowed {
			return resp
		}

		seed := &gardencorev1beta1.Seed{ObjectMeta: metav1.ObjectMeta{Name: seedName}}
		if err := h.Client.Get(ctx, client.ObjectKeyFromObject(seed), seed); err != nil {
			return admission.Errored(http.StatusInternalServerError, err)
		}

		if seed.Spec.Backup == nil {
			return admission.Errored(http.StatusForbidden, errors.New("gardenlet's seed has no backup configuration"))
		}

		backup := seed.Spec.Backup
		region := ptr.Deref(backup.Region, seed.Spec.Provider.Region)
		if backupBucket.Spec.Provider.Type != backup.Provider ||
			backupBucket.Spec.Provider.Region != region ||
			!apiequality.Semantic.DeepEqual(backupBucket.Spec.ProviderConfig, backup.ProviderConfig) ||
			!apiequality.Semantic.DeepEqual(backupBucket.Spec.CredentialsRef, backup.CredentialsRef) {
			return admission.Errored(http.StatusForbidden, errors.New("BackupBucket spec does not match the backup configuration of the gardenlet's seed"))
		}

		return admission.Allowed("")

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
	switch request.Operation {
	case admissionv1.Update:
		oldEntry := &gardencorev1beta1.BackupEntry{}
		if err := h.Decoder.DecodeRaw(request.OldObject, oldEntry); err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}
		newEntry := &gardencorev1beta1.BackupEntry{}
		if err := h.Decoder.Decode(request, newEntry); err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}
		if !apiequality.Semantic.DeepEqual(oldEntry.Spec.ShootRef, newEntry.Spec.ShootRef) {
			return admission.Errored(http.StatusForbidden, errors.New("gardenlet must not modify .spec.shootRef of BackupEntry"))
		}
		if oldEntry.Spec.BucketName != newEntry.Spec.BucketName {
			backupBucket := &gardencorev1beta1.BackupBucket{ObjectMeta: metav1.ObjectMeta{Name: newEntry.Spec.BucketName}}
			if err := h.Client.Get(ctx, client.ObjectKeyFromObject(backupBucket), backupBucket); err != nil {
				return admission.Errored(http.StatusInternalServerError, err)
			}
			if resp := h.admit(seedName, backupBucket.Spec.SeedName); !resp.Allowed {
				return resp
			}
		}
		return admission.Allowed("")

	case admissionv1.Create:
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

	return admission.Errored(http.StatusBadRequest, fmt.Errorf("unexpected operation: %q", request.Operation))
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
	switch request.Operation {
	case admissionv1.Update:
		oldBastion := &operationsv1alpha1.Bastion{}
		if err := h.Decoder.DecodeRaw(request.OldObject, oldBastion); err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}
		newBastion := &operationsv1alpha1.Bastion{}
		if err := h.Decoder.Decode(request, newBastion); err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}
		if !apiequality.Semantic.DeepEqual(oldBastion.Spec, newBastion.Spec) {
			return admission.Errored(http.StatusForbidden, errors.New("gardenlet must not modify .spec of Bastion"))
		}
		return admission.Allowed("")

	case admissionv1.Create:
		bastion := &operationsv1alpha1.Bastion{}
		if err := h.Decoder.Decode(request, bastion); err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}
		return h.admit(seedName, bastion.Spec.SeedName)
	}

	return admission.Errored(http.StatusBadRequest, fmt.Errorf("unexpected operation: %q", request.Operation))
}

func (h *Handler) admitControllerInstallation(_ string, request admission.Request) admission.Response {
	if request.Operation != admissionv1.Update {
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("unexpected operation: %q", request.Operation))
	}
	oldCI := &gardencorev1beta1.ControllerInstallation{}
	if err := h.Decoder.DecodeRaw(request.OldObject, oldCI); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}
	newCI := &gardencorev1beta1.ControllerInstallation{}
	if err := h.Decoder.Decode(request, newCI); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}
	if !apiequality.Semantic.DeepEqual(oldCI.Spec, newCI.Spec) {
		return admission.Errored(http.StatusForbidden, errors.New("gardenlet must not modify .spec of ControllerInstallation"))
	}
	return admission.Allowed("")
}

func (h *Handler) admitManagedSeed(_ string, request admission.Request) admission.Response {
	if request.Operation != admissionv1.Update {
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("unexpected operation: %q", request.Operation))
	}
	oldManagedSeed := &seedmanagementv1alpha1.ManagedSeed{}
	if err := h.Decoder.DecodeRaw(request.OldObject, oldManagedSeed); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}
	newManagedSeed := &seedmanagementv1alpha1.ManagedSeed{}
	if err := h.Decoder.Decode(request, newManagedSeed); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}
	if !apiequality.Semantic.DeepEqual(oldManagedSeed.Spec, newManagedSeed.Spec) {
		return admission.Errored(http.StatusForbidden, errors.New("gardenlet must not modify .spec of ManagedSeed"))
	}
	return admission.Allowed("")
}

func (h *Handler) admitCertificateSigningRequest(seedName string, userType gardenletidentity.UserType, request admission.Request) admission.Response {
	if request.Operation != admissionv1.Create {
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("unexpected operation: %q", request.Operation))
	}

	if userType == gardenletidentity.UserTypeExtension {
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

func (h *Handler) admitClusterRoleBinding(ctx context.Context, seedName string, userType gardenletidentity.UserType, request admission.Request) admission.Response {
	if request.Operation != admissionv1.Create {
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("unexpected operation: %q", request.Operation))
	}

	if userType == gardenletidentity.UserTypeExtension {
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

func (h *Handler) admitGardenlet(ctx context.Context, seedName string, request admission.Request) admission.Response {
	switch request.Operation {
	case admissionv1.Update:
		oldGardenlet := &seedmanagementv1alpha1.Gardenlet{}
		if err := h.Decoder.DecodeRaw(request.OldObject, oldGardenlet); err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}
		newGardenlet := &seedmanagementv1alpha1.Gardenlet{}
		if err := h.Decoder.Decode(request, newGardenlet); err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}
		if !apiequality.Semantic.DeepEqual(oldGardenlet.Spec, newGardenlet.Spec) {
			return admission.Errored(http.StatusForbidden, errors.New("gardenlet must not modify .spec of Gardenlet"))
		}
		return admission.Allowed("")

	case admissionv1.Create:
		if request.Namespace != v1beta1constants.GardenNamespace {
			return admission.Errored(http.StatusBadRequest, fmt.Errorf("object must be in namespace: %q", v1beta1constants.GardenNamespace))
		}
		if resp := h.admit(seedName, &request.Name); !resp.Allowed {
			return resp
		}
		managedSeed := &seedmanagementv1alpha1.ManagedSeed{ObjectMeta: metav1.ObjectMeta{Namespace: v1beta1constants.GardenNamespace, Name: seedName}}
		if err := h.Client.Get(ctx, client.ObjectKeyFromObject(managedSeed), managedSeed); err == nil {
			return admission.Errored(http.StatusForbidden, errors.New("managed-seed gardenlet must not create Gardenlet resources"))
		} else if !apierrors.IsNotFound(err) {
			return admission.Errored(http.StatusInternalServerError, err)
		}
		gardenlet := &seedmanagementv1alpha1.Gardenlet{}
		if err := h.Decoder.Decode(request, gardenlet); err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}
		if gardenlet.Spec.KubeconfigSecretRef != nil {
			return admission.Errored(http.StatusForbidden, errors.New("gardenlet must not set .spec.kubeconfigSecretRef"))
		}
		return admission.Allowed("")
	}

	return admission.Errored(http.StatusBadRequest, fmt.Errorf("unexpected operation: %q", request.Operation))
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

func (h *Handler) admitLease(seedName string, userType gardenletidentity.UserType, request admission.Request) admission.Response {
	if request.Operation != admissionv1.Create {
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("unexpected operation: %q", request.Operation))
	}

	// extension clients may only work with leases in the seed namespace
	if userType == gardenletidentity.UserTypeExtension {
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
		if publicKeyType, ok := secret.Labels[v1beta1constants.LabelDiscoveryPublic]; !ok {
			return admission.Errored(http.StatusUnprocessableEntity, fmt.Errorf("label %q is missing", v1beta1constants.LabelDiscoveryPublic))
		} else if publicKeyType != v1beta1constants.LabelPublicKeysServiceAccount {
			return admission.Errored(http.StatusUnprocessableEntity, fmt.Errorf("label %q value must be set to %q", v1beta1constants.LabelDiscoveryPublic, v1beta1constants.LabelPublicKeysServiceAccount))
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
			return h.allowIfGardenletIsNotYetBootstrapped(ctx, seedName, namespace, name)
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
		shoot := &gardencorev1beta1.Shoot{ObjectMeta: metav1.ObjectMeta{Namespace: managedSeed.Namespace, Name: managedSeed.Spec.Shoot.Name}}
		if err := h.Client.Get(ctx, client.ObjectKeyFromObject(shoot), shoot); err != nil {
			return admission.Errored(http.StatusInternalServerError, err)
		}

		if !h.admit(seedName, shoot.Spec.SeedName).Allowed {
			continue
		}

		seedTemplate, _, err := seedmanagementv1alpha1helper.ExtractSeedTemplateAndGardenletConfig(managedSeed.Name, &managedSeed.Spec.Gardenlet.Config)
		if err != nil {
			return admission.Errored(http.StatusInternalServerError, err)
		}
		if seedTemplate == nil {
			return admission.Errored(http.StatusInternalServerError, fmt.Errorf("seed template is nil for ManagedSeed %q", managedSeed.Name))
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
	switch request.Operation {
	case admissionv1.Update:
		if resp := h.admit(seedName, &request.Name); !resp.Allowed {
			return resp
		}
		oldSeed := &gardencorev1beta1.Seed{}
		if err := h.Decoder.DecodeRaw(request.OldObject, oldSeed); err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}
		newSeed := &gardencorev1beta1.Seed{}
		if err := h.Decoder.Decode(request, newSeed); err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}
		if apiequality.Semantic.DeepEqual(oldSeed.Spec, newSeed.Spec) {
			return admission.Allowed("")
		}

		// Spec changed: validate that the new spec matches the authoritative spec in the ManagedSeed or
		// Gardenlet resource. The gardenlet is the canonical writer of Seed.spec, but only within the
		// bounds set by the operator via those resources.
		managedSeed := &seedmanagementv1alpha1.ManagedSeed{ObjectMeta: metav1.ObjectMeta{Namespace: v1beta1constants.GardenNamespace, Name: request.Name}}
		if err := h.Client.Get(ctx, client.ObjectKeyFromObject(managedSeed), managedSeed); err != nil && !apierrors.IsNotFound(err) {
			return admission.Errored(http.StatusInternalServerError, err)
		} else if err == nil {
			seedTemplate, _, err := seedmanagementv1alpha1helper.ExtractSeedTemplateAndGardenletConfig(managedSeed.Name, &managedSeed.Spec.Gardenlet.Config)
			if err != nil {
				return admission.Errored(http.StatusInternalServerError, err)
			}
			if seedTemplate != nil && !apiequality.Semantic.DeepEqual(newSeed.Spec, seedTemplate.Spec) {
				return admission.Errored(http.StatusForbidden, errors.New("gardenlet must not set .spec of Seed to a value different from the .spec in the ManagedSeed"))
			}
			return admission.Allowed("")
		}

		gardenlet := &seedmanagementv1alpha1.Gardenlet{ObjectMeta: metav1.ObjectMeta{Namespace: v1beta1constants.GardenNamespace, Name: request.Name}}
		if err := h.Client.Get(ctx, client.ObjectKeyFromObject(gardenlet), gardenlet); err != nil && !apierrors.IsNotFound(err) {
			return admission.Errored(http.StatusInternalServerError, err)
		} else if err == nil {
			seedTemplate, _, err := seedmanagementv1alpha1helper.ExtractSeedTemplateAndGardenletConfig(gardenlet.Name, &gardenlet.Spec.Config)
			if err != nil {
				return admission.Errored(http.StatusInternalServerError, err)
			}
			if seedTemplate != nil && !apiequality.Semantic.DeepEqual(newSeed.Spec, seedTemplate.Spec) {
				return admission.Errored(http.StatusForbidden, errors.New("gardenlet must not set .spec of Seed to a value different from the .spec in the Gardenlet"))
			}
			return admission.Allowed("")
		}

		// TODO(rfranzke): Once the Gardenlet resource is always present for all seeds, deny spec changes that have no
		// authoritative source to validate against. Until then, allow them to avoid breaking existing deployments.
		return admission.Allowed("")

	case admissionv1.Create:
		return h.admit(seedName, &request.Name)

	case admissionv1.Delete:
		// A gardenlet must not delete its own Seed — deletion is always triggered externally;
		// the gardenlet only removes the finalizer as part of its delete flow.
		if request.Name == seedName {
			return admission.Errored(http.StatusForbidden, errors.New("gardenlet must not delete its own Seed"))
		}

		// The deletion request might be submitted by the "parent gardenlet", i.e. the gardenlet/seed
		// which is responsible for the ManagedSeed in question.
		managedSeed := &seedmanagementv1alpha1.ManagedSeed{ObjectMeta: metav1.ObjectMeta{Namespace: v1beta1constants.GardenNamespace, Name: request.Name}}
		if err := h.Client.Get(ctx, client.ObjectKeyFromObject(managedSeed), managedSeed); err != nil {
			if apierrors.IsNotFound(err) {
				return admission.Errored(http.StatusForbidden, fmt.Errorf("object does not belong to seed %q", seedName))
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
			return admission.Errored(http.StatusForbidden, fmt.Errorf("object does not belong to seed %q", seedName))
		}

		// Check if the `.spec.seedName` of the Shoot referenced in the `.spec.shoot.name` field of the ManagedSeed matches
		// the seed name of the requesting gardenlet.
		shoot := &gardencorev1beta1.Shoot{ObjectMeta: metav1.ObjectMeta{Namespace: managedSeed.Namespace, Name: managedSeed.Spec.Shoot.Name}}
		if err := h.Client.Get(ctx, client.ObjectKeyFromObject(shoot), shoot); err != nil {
			return admission.Errored(http.StatusInternalServerError, err)
		}

		return h.admit(seedName, shoot.Spec.SeedName)
	}

	return admission.Errored(http.StatusBadRequest, fmt.Errorf("unexpected operation: %q", request.Operation))
}

func (h *Handler) admitServiceAccount(ctx context.Context, seedName string, userType gardenletidentity.UserType, request admission.Request) admission.Response {
	if request.Operation != admissionv1.Create {
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("unexpected operation: %q", request.Operation))
	}

	if userType == gardenletidentity.UserTypeExtension {
		return admission.Errored(http.StatusForbidden, fmt.Errorf("extension client may not create ServiceAccounts"))
	}

	// Allow gardenlet to create service accounts which can be used to bootstrap other gardenlets deployed as part of
	// the ManagedSeed reconciliation.
	if after, ok := strings.CutPrefix(request.Name, gardenletbootstraputil.ServiceAccountNamePrefix); ok {
		return h.allowIfManagedSeedIsNotYetBootstrapped(ctx, seedName, request.Namespace, after)
	}

	// Allow all verbs for service accounts in gardenlets' seed-<name> namespaces.
	if request.Namespace == gardenerutils.ComputeGardenNamespace(seedName) {
		return admission.Allowed("")
	}

	return admission.Errored(http.StatusForbidden, fmt.Errorf("object does not belong to seed %q", seedName))
}

func (h *Handler) admitShoot(_ string, request admission.Request) admission.Response {
	if request.Operation != admissionv1.Update {
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("unexpected operation: %q", request.Operation))
	}
	oldShoot := &gardencorev1beta1.Shoot{}
	if err := h.Decoder.DecodeRaw(request.OldObject, oldShoot); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}
	newShoot := &gardencorev1beta1.Shoot{}
	if err := h.Decoder.Decode(request, newShoot); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}
	if apiequality.Semantic.DeepEqual(oldShoot.Spec, newShoot.Spec) {
		return admission.Allowed("")
	}
	// Allow setting .spec.networking.nodes from nil — gardenlet writes this after node CIDR discovery.
	var oldNodes *string
	if oldShoot.Spec.Networking != nil {
		oldNodes = oldShoot.Spec.Networking.Nodes
	}
	var newNodes *string
	if newShoot.Spec.Networking != nil {
		newNodes = newShoot.Spec.Networking.Nodes
	}
	if oldNodes == nil && newNodes != nil {
		// Check that .spec.networking.nodes is the only change by temporarily patching old spec and re-comparing.
		oldSpecForComparison := *oldShoot.Spec.DeepCopy()
		if oldSpecForComparison.Networking == nil {
			oldSpecForComparison.Networking = &gardencorev1beta1.Networking{}
		}
		oldSpecForComparison.Networking.Nodes = newNodes
		if apiequality.Semantic.DeepEqual(oldSpecForComparison, newShoot.Spec) {
			return admission.Allowed("")
		}
	}
	return admission.Errored(http.StatusForbidden, errors.New("gardenlet must not modify .spec of Shoot (only .spec.networking.nodes may be set from nil)"))
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

func (h *Handler) allowIfGardenletIsNotYetBootstrapped(ctx context.Context, seedName, gardenletNamespace, gardenletName string) admission.Response {
	if gardenletName != seedName || gardenletNamespace != v1beta1constants.GardenNamespace {
		return admission.Errored(http.StatusForbidden, fmt.Errorf("gardenlet %s/%s does not belong to seed %q", gardenletNamespace, gardenletName, seedName))
	}

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
