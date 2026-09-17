// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package shootrestriction

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-logr/logr"
	admissionv1 "k8s.io/api/admission/v1"
	certificatesv1 "k8s.io/api/certificates/v1"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	bootstraptokenapi "k8s.io/cluster-bootstrap/token/api"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/gardener/gardener/pkg/admissioncontroller/gardenletidentity"
	shootidentity "github.com/gardener/gardener/pkg/admissioncontroller/gardenletidentity/shoot"
	admissionwebhook "github.com/gardener/gardener/pkg/admissioncontroller/webhook/admission"
	v1beta1helper "github.com/gardener/gardener/pkg/api/core/v1beta1/helper"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	operationsv1alpha1 "github.com/gardener/gardener/pkg/apis/operations/v1alpha1"
	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	gardenletbootstraputil "github.com/gardener/gardener/pkg/gardenlet/bootstrap/util"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	gardenletutils "github.com/gardener/gardener/pkg/utils/gardener/gardenlet"
)

var (
	// Only take v1beta1 for the core.gardener.cloud API group because the Authorize function only checks the resource
	// group and the resource (but it ignores the version).
	backupBucketResource              = gardencorev1beta1.Resource("backupbuckets")
	backupEntryResource               = gardencorev1beta1.Resource("backupentries")
	bastionResource                   = operationsv1alpha1.Resource("bastions")
	certificateSigningRequestResource = certificatesv1.Resource("certificatesigningrequests")
	configMapResource                 = corev1.Resource("configmaps")
	controllerInstallationResource    = gardencorev1beta1.Resource("controllerinstallations")
	gardenletResource                 = seedmanagementv1alpha1.Resource("gardenlets")
	internalSecretResource            = gardencorev1beta1.Resource("internalsecrets")
	leaseResource                     = coordinationv1.Resource("leases")
	managedSeedResource               = seedmanagementv1alpha1.Resource("managedseeds")
	projectResource                   = gardencorev1beta1.Resource("projects")
	secretResource                    = corev1.Resource("secrets")
	serviceAccountResource            = corev1.Resource("serviceaccounts")
	shootResource                     = gardencorev1beta1.Resource("shoots")
	shootStateResource                = gardencorev1beta1.Resource("shootstates")
	workloadIdentityResource          = securityv1alpha1.Resource("workloadidentities")
)

// Handler restricts requests made by shoot gardenlets.
type Handler struct {
	Logger  logr.Logger
	Client  client.Reader
	Decoder admission.Decoder
}

// Handle restricts requests made by gardenlets.
func (h *Handler) Handle(ctx context.Context, request admission.Request) admission.Response {
	shootNamespace, shootName, isSelfHostedShoot, userType := shootidentity.FromAuthenticationV1UserInfo(request.UserInfo)
	if !isSelfHostedShoot {
		return admissionwebhook.Allowed("")
	}

	var (
		log                = h.Logger.WithValues("shootNamespace", shootNamespace, "shootName", shootName, "userType", userType)
		gardenletShootInfo = types.NamespacedName{Name: shootName, Namespace: shootNamespace}
	)

	if userType == gardenletidentity.UserTypeGardenadm {
		return h.admitGardenadmRequests(ctx, gardenletShootInfo, request)
	}

	requestResource := schema.GroupResource{Group: request.Resource.Group, Resource: request.Resource.Resource}
	switch requestResource {
	case backupBucketResource:
		return h.admitBackupBucket(ctx, gardenletShootInfo, request)

	case backupEntryResource:
		return h.admitBackupEntry(ctx, gardenletShootInfo, request)

	case bastionResource:
		return h.admitBastion(gardenletShootInfo, request)

	case certificateSigningRequestResource:
		return h.admitCertificateSigningRequest(gardenletShootInfo, userType, request)

	case configMapResource:
		return h.admitConfigMap(gardenletShootInfo, request)

	case controllerInstallationResource:
		return h.admitControllerInstallation(gardenletShootInfo, request)

	case gardenletResource:
		return h.admitGardenlet(gardenletShootInfo, request)

	case shootResource:
		return h.admitShoot(gardenletShootInfo, request)

	case internalSecretResource:
		return h.admitInternalSecret(gardenletShootInfo, request)

	case leaseResource:
		return h.admitLease(gardenletShootInfo, userType, request)

	case managedSeedResource:
		return h.admitManagedSeed(ctx, gardenletShootInfo, request)

	case secretResource:
		return h.admitSecret(ctx, gardenletShootInfo, request)

	case serviceAccountResource:
		return h.admitServiceAccount(gardenletShootInfo, userType, request)

	case shootStateResource:
		return h.admitShootState(gardenletShootInfo, request)

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

func (h *Handler) admitBackupBucket(ctx context.Context, gardenletShootInfo types.NamespacedName, request admission.Request) admission.Response {
	switch request.Operation {
	case admissionv1.Update:
		oldBB := &gardencorev1beta1.BackupBucket{}
		if err := h.Decoder.DecodeRaw(request.OldObject, oldBB); err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}
		newBB := &gardencorev1beta1.BackupBucket{}
		if err := h.Decoder.Decode(request, newBB); err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}
		if !apiequality.Semantic.DeepEqual(oldBB.Spec, newBB.Spec) {
			return admission.Errored(http.StatusForbidden, errors.New("gardenlet must not modify .spec of BackupBucket"))
		}
		return admission.Allowed("")

	case admissionv1.Create:
		backupBucket := &gardencorev1beta1.BackupBucket{}
		if err := h.Decoder.Decode(request, backupBucket); err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}
		if backupBucket.Spec.ShootRef == nil {
			return admission.Errored(http.StatusForbidden, fmt.Errorf("object does not belong to shoot %s", gardenletShootInfo))
		}
		if resp := h.admit(gardenletShootInfo, types.NamespacedName{Name: backupBucket.Spec.ShootRef.Name, Namespace: backupBucket.Spec.ShootRef.Namespace}); !resp.Allowed {
			return resp
		}

		shoot := &gardencorev1beta1.Shoot{ObjectMeta: metav1.ObjectMeta{Name: gardenletShootInfo.Name, Namespace: gardenletShootInfo.Namespace}}
		if err := h.Client.Get(ctx, client.ObjectKeyFromObject(shoot), shoot); err != nil {
			return admission.Errored(http.StatusInternalServerError, err)
		}

		cpWorker := v1beta1helper.ControlPlaneWorkerPoolForShoot(shoot.Spec.Provider.Workers)
		if cpWorker == nil {
			return admission.Errored(http.StatusForbidden, errors.New("shoot has no control-plane worker pool"))
		}

		if cpWorker.ControlPlane.Backup == nil {
			return admission.Errored(http.StatusForbidden, errors.New("shoot's control-plane worker pool has no backup configuration"))
		}

		backup := cpWorker.ControlPlane.Backup
		region := ptr.Deref(backup.Region, shoot.Spec.Region)
		if backupBucket.Spec.Provider.Type != backup.Provider ||
			backupBucket.Spec.Provider.Region != region ||
			!apiequality.Semantic.DeepEqual(backupBucket.Spec.ProviderConfig, backup.ProviderConfig) ||
			!apiequality.Semantic.DeepEqual(backupBucket.Spec.CredentialsRef, backup.CredentialsRef) {
			return admission.Errored(http.StatusForbidden, errors.New("BackupBucket spec does not match the backup configuration of the shoot's control-plane worker pool"))
		}

		return admission.Allowed("")

	default:
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("unexpected operation: %q", request.Operation))
	}
}

func (h *Handler) admitBackupEntry(ctx context.Context, gardenletShootInfo types.NamespacedName, request admission.Request) admission.Response {
	switch request.Operation {
	case admissionv1.Update:
		oldBE := &gardencorev1beta1.BackupEntry{}
		if err := h.Decoder.DecodeRaw(request.OldObject, oldBE); err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}
		newBE := &gardencorev1beta1.BackupEntry{}
		if err := h.Decoder.Decode(request, newBE); err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}
		if !apiequality.Semantic.DeepEqual(oldBE.Spec.ShootRef, newBE.Spec.ShootRef) {
			return admission.Errored(http.StatusForbidden, errors.New("gardenlet must not modify .spec.shootRef of BackupEntry"))
		}
		if oldBE.Spec.BucketName != newBE.Spec.BucketName {
			backupBucket := &gardencorev1beta1.BackupBucket{ObjectMeta: metav1.ObjectMeta{Name: newBE.Spec.BucketName}}
			if err := h.Client.Get(ctx, client.ObjectKeyFromObject(backupBucket), backupBucket); err != nil {
				if apierrors.IsNotFound(err) {
					return admission.Errored(http.StatusForbidden, err)
				}
				return admission.Errored(http.StatusInternalServerError, err)
			}
			if backupBucket.Spec.ShootRef == nil {
				return admission.Errored(http.StatusForbidden, fmt.Errorf("bucket does not belong to shoot %s", gardenletShootInfo))
			}
			return h.admit(gardenletShootInfo, types.NamespacedName{Name: backupBucket.Spec.ShootRef.Name, Namespace: backupBucket.Spec.ShootRef.Namespace})
		}
		return admission.Allowed("")

	case admissionv1.Create:
		backupEntry := &gardencorev1beta1.BackupEntry{}
		if err := h.Decoder.Decode(request, backupEntry); err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}
		if backupEntry.Spec.ShootRef == nil {
			return admission.Errored(http.StatusForbidden, fmt.Errorf("object does not belong to shoot %s", gardenletShootInfo))
		}
		if resp := h.admit(gardenletShootInfo, types.NamespacedName{Name: backupEntry.Spec.ShootRef.Name, Namespace: backupEntry.Spec.ShootRef.Namespace}); !resp.Allowed {
			return resp
		}
		backupBucket := &gardencorev1beta1.BackupBucket{ObjectMeta: metav1.ObjectMeta{Name: backupEntry.Spec.BucketName}}
		if err := h.Client.Get(ctx, client.ObjectKeyFromObject(backupBucket), backupBucket); err != nil {
			if apierrors.IsNotFound(err) {
				return admission.Errored(http.StatusForbidden, err)
			}
			return admission.Errored(http.StatusInternalServerError, err)
		}
		if backupBucket.Spec.ShootRef == nil {
			return admission.Errored(http.StatusForbidden, fmt.Errorf("object does not belong to shoot %s", gardenletShootInfo))
		}
		return h.admit(gardenletShootInfo, types.NamespacedName{Name: backupBucket.Spec.ShootRef.Name, Namespace: backupBucket.Spec.ShootRef.Namespace})

	default:
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("unexpected operation: %q", request.Operation))
	}
}

func (h *Handler) admitBastion(_ types.NamespacedName, request admission.Request) admission.Response {
	if request.Operation != admissionv1.Update {
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("unexpected operation: %q", request.Operation))
	}
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
}

func (h *Handler) admitControllerInstallation(_ types.NamespacedName, request admission.Request) admission.Response {
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

func (h *Handler) admitCertificateSigningRequest(gardenletShootInfo types.NamespacedName, userType gardenletidentity.UserType, request admission.Request) admission.Response {
	if request.Operation != admissionv1.Create {
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("unexpected operation: %q", request.Operation))
	}

	if userType != gardenletidentity.UserTypeGardenlet {
		return admission.Errored(http.StatusForbidden, errors.New("only gardenlet clients may create CertificateSigningRequests"))
	}

	csr := &certificatesv1.CertificateSigningRequest{}
	if err := h.Decoder.Decode(request, csr); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	x509cr, err := utils.DecodeCertificateRequest(csr.Spec.Request)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	if ok, reason := gardenerutils.IsShootClientCert(x509cr, csr.Spec.Usages); !ok {
		return admission.Errored(http.StatusForbidden, fmt.Errorf("can only create CSRs for shoot clusters: %s", reason))
	}

	namespace, name, _, _ := shootidentity.FromCertificateSigningRequest(x509cr)
	return h.admit(gardenletShootInfo, types.NamespacedName{Name: name, Namespace: namespace})
}

func (h *Handler) admitConfigMap(gardenletShootInfo types.NamespacedName, request admission.Request) admission.Response {
	if request.Operation != admissionv1.Create {
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("unexpected operation: %q", request.Operation))
	}

	// Check if the config map is related to a Shoot assigned to the seed the gardenlet is responsible for.
	if shootName, ok := gardenerutils.IsShootProjectConfigMap(request.Name); ok {
		return h.admit(gardenletShootInfo, types.NamespacedName{Name: shootName, Namespace: request.Namespace})
	}

	return admission.Errored(http.StatusForbidden, fmt.Errorf("object does not belong to shoot %s", gardenletShootInfo))
}

func (h *Handler) admitInternalSecret(gardenletShootInfo types.NamespacedName, request admission.Request) admission.Response {
	if request.Operation != admissionv1.Create {
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("unexpected operation: %q", request.Operation))
	}

	// Check if the internal secret is related to a Shoot assigned to the seed the gardenlet is responsible for.
	if shootName, ok := gardenerutils.IsShootProjectInternalSecret(request.Name); ok {
		return h.admit(gardenletShootInfo, types.NamespacedName{Name: shootName, Namespace: request.Namespace})
	}

	return admission.Errored(http.StatusForbidden, fmt.Errorf("object does not belong to shoot %s", gardenletShootInfo))
}

func (h *Handler) admitManagedSeed(_ context.Context, gardenletShootInfo types.NamespacedName, request admission.Request) admission.Response {
	if request.Operation != admissionv1.Update {
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("unexpected operation: %q", request.Operation))
	}

	oldMS := &seedmanagementv1alpha1.ManagedSeed{}
	if err := h.Decoder.DecodeRaw(request.OldObject, oldMS); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}
	newMS := &seedmanagementv1alpha1.ManagedSeed{}
	if err := h.Decoder.Decode(request, newMS); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	if !apiequality.Semantic.DeepEqual(oldMS.Spec, newMS.Spec) {
		return admission.Errored(http.StatusForbidden, errors.New("gardenlet must not modify .spec of ManagedSeed"))
	}

	if oldMS.Spec.Shoot == nil {
		return admission.Errored(http.StatusForbidden, fmt.Errorf("object does not belong to shoot %s", gardenletShootInfo))
	}

	return h.admit(gardenletShootInfo, types.NamespacedName{Name: oldMS.Spec.Shoot.Name, Namespace: request.Namespace})
}

func (h *Handler) admitSecret(ctx context.Context, gardenletShootInfo types.NamespacedName, request admission.Request) admission.Response {
	if request.Operation != admissionv1.Create {
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("unexpected operation: %q", request.Operation))
	}

	// Check if the secret is related to a Shoot assigned to the seed the gardenlet is responsible for.
	if shootName, ok := gardenerutils.IsShootProjectSecret(request.Name); ok {
		return h.admit(gardenletShootInfo, types.NamespacedName{Name: shootName, Namespace: request.Namespace})
	}

	// Check if the secret is related to a BackupBucket assigned to the Shoot the gardenlet is responsible for.
	if strings.HasPrefix(request.Name, v1beta1constants.SecretPrefixGeneratedBackupBucket) {
		backupBucket := &gardencorev1beta1.BackupBucket{}
		if err := h.Client.Get(ctx, client.ObjectKey{Name: strings.TrimPrefix(request.Name, v1beta1constants.SecretPrefixGeneratedBackupBucket)}, backupBucket); err != nil {
			if apierrors.IsNotFound(err) {
				return admission.Errored(http.StatusForbidden, err)
			}
			return admission.Errored(http.StatusInternalServerError, err)
		}

		if backupBucket.Spec.ShootRef == nil {
			return admission.Errored(http.StatusForbidden, fmt.Errorf(".spec.shootRef must be set in the BackupBucket resource %q belonging to this Secret", backupBucket.Name))
		}

		return h.admit(gardenletShootInfo, types.NamespacedName{Name: backupBucket.Spec.ShootRef.Name, Namespace: backupBucket.Spec.ShootRef.Namespace})
	}

	// Check if the secret is a bootstrap token for a ManagedSeed referencing the gardenlet's shoot.
	if request.Namespace == metav1.NamespaceSystem && strings.HasPrefix(request.Name, bootstraptokenapi.BootstrapTokenSecretPrefix) {
		secret := &corev1.Secret{}
		if err := h.Decoder.Decode(request, secret); err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}

		kind, namespace, name := gardenletbootstraputil.MetadataFromDescription(string(secret.Data[bootstraptokenapi.BootstrapTokenDescriptionKey]))
		switch kind {
		case gardenletbootstraputil.KindManagedSeed:
			managedSeed := &seedmanagementv1alpha1.ManagedSeed{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace}}
			if err := h.Client.Get(ctx, client.ObjectKeyFromObject(managedSeed), managedSeed); err != nil {
				if apierrors.IsNotFound(err) {
					return admission.Errored(http.StatusForbidden, err)
				}
				return admission.Errored(http.StatusInternalServerError, err)
			}

			return h.admit(gardenletShootInfo, types.NamespacedName{Name: managedSeed.Spec.Shoot.Name, Namespace: managedSeed.Namespace})

		case gardenletbootstraputil.KindGardenlet:
			// The Gardenlet resource name carries the `self-hosted-shoot-` prefix; strip it to recover the shoot name.
			return h.admit(gardenletShootInfo, types.NamespacedName{Name: strings.TrimPrefix(name, gardenletutils.ResourcePrefixSelfHostedShoot), Namespace: namespace})
		}
	}

	return admission.Errored(http.StatusForbidden, fmt.Errorf("object does not belong to shoot %s", gardenletShootInfo))
}

func (h *Handler) admitShootState(gardenletShootInfo types.NamespacedName, request admission.Request) admission.Response {
	if request.Operation != admissionv1.Create {
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("unexpected operation: %q", request.Operation))
	}

	return h.admit(gardenletShootInfo, types.NamespacedName{Name: request.Name, Namespace: request.Namespace})
}

func (h *Handler) admit(gardenletShootInfo, objectShootInfo types.NamespacedName) admission.Response {
	// Allow request if the shoot the gardenlet is responsible for matches with the shoot related to the object.
	if gardenletShootInfo.Name == objectShootInfo.Name && gardenletShootInfo.Namespace == objectShootInfo.Namespace {
		return admission.Allowed("")
	}

	return admission.Errored(http.StatusForbidden, fmt.Errorf("object does not belong to shoot %s", gardenletShootInfo))
}

func (h *Handler) admitLease(gardenletShootInfo types.NamespacedName, userType gardenletidentity.UserType, request admission.Request) admission.Response {
	if request.Operation != admissionv1.Create {
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("unexpected operation: %q", request.Operation))
	}

	// Extension clients may only create leases in the shoot namespace and whose name is prefixed with
	// the shoot name to avoid tampering with leases belonging to other shoots in the same project namespace.
	if userType == gardenletidentity.UserTypeExtension {
		if request.Namespace != gardenletShootInfo.Namespace {
			return admission.Errored(http.StatusForbidden, fmt.Errorf("extension client can only create leases in the namespace for shoot %q", gardenletShootInfo))
		}
		if !strings.HasPrefix(request.Name, gardenletShootInfo.Name+"--") {
			return admission.Errored(http.StatusForbidden, fmt.Errorf("extension client can only create leases with the shoot name %q as prefix", gardenletShootInfo.Name))
		}
		return admission.Allowed("")
	}

	return h.admitCreateWithResourcePrefix(gardenletShootInfo, request)
}

func (h *Handler) admitServiceAccount(gardenletShootInfo types.NamespacedName, userType gardenletidentity.UserType, request admission.Request) admission.Response {
	if request.Operation != admissionv1.Create {
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("unexpected operation: %q", request.Operation))
	}

	if userType == gardenletidentity.UserTypeExtension {
		return admission.Errored(http.StatusForbidden, fmt.Errorf("extension client may not create ServiceAccounts"))
	}

	// Allow gardenlet to create service accounts for extensions in the shoot's project namespace.
	// The SA name must be prefixed with extension-shoot--<shootName>-- to scope to this shoot.
	if request.Namespace == gardenletShootInfo.Namespace &&
		strings.HasPrefix(request.Name, v1beta1constants.ExtensionShootServiceAccountPrefix+gardenletShootInfo.Name+"--") {
		return admission.Allowed("")
	}

	return admission.Errored(http.StatusForbidden, fmt.Errorf("object does not belong to shoot %s", gardenletShootInfo))
}

func (h *Handler) admitCreateWithResourcePrefix(gardenletShootInfo types.NamespacedName, request admission.Request) admission.Response {
	if request.Operation != admissionv1.Create {
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("unexpected operation: %q", request.Operation))
	}

	if !strings.HasPrefix(request.Name, gardenletutils.ResourcePrefixSelfHostedShoot) {
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("the resource for self-hosted shoots must be prefixed with %q", gardenletutils.ResourcePrefixSelfHostedShoot))
	}

	return h.admit(gardenletShootInfo, types.NamespacedName{Name: strings.TrimPrefix(request.Name, gardenletutils.ResourcePrefixSelfHostedShoot), Namespace: request.Namespace})
}

func (h *Handler) admitGardenlet(gardenletShootInfo types.NamespacedName, request admission.Request) admission.Response {
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
		return h.admitCreateWithResourcePrefix(gardenletShootInfo, request)
	}

	return admission.Errored(http.StatusBadRequest, fmt.Errorf("unexpected operation: %q", request.Operation))
}

func (h *Handler) admitShoot(_ types.NamespacedName, request admission.Request) admission.Response {
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
