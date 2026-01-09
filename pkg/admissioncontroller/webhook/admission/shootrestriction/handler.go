// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/gardener/gardener/pkg/admissioncontroller/gardenletidentity"
	shootidentity "github.com/gardener/gardener/pkg/admissioncontroller/gardenletidentity/shoot"
	admissionwebhook "github.com/gardener/gardener/pkg/admissioncontroller/webhook/admission"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	gardenletutils "github.com/gardener/gardener/pkg/utils/gardener/gardenlet"
)

var (
	// Only take v1beta1 for the core.gardener.cloud API group because the Authorize function only checks the resource
	// group and the resource (but it ignores the version).
	backupBucketResource              = gardencorev1beta1.Resource("backupbuckets")
	backupEntryResource               = gardencorev1beta1.Resource("backupentries")
	certificateSigningRequestResource = certificatesv1.Resource("certificatesigningrequests")
	configMapResource                 = corev1.Resource("configmaps")
	gardenletResource                 = seedmanagementv1alpha1.Resource("gardenlets")
	leaseResource                     = coordinationv1.Resource("leases")
	projectResource                   = gardencorev1beta1.Resource("projects")
	secretResource                    = corev1.Resource("secrets")
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
	case certificateSigningRequestResource:
		return h.admitCertificateSigningRequest(gardenletShootInfo, userType, request)

	case gardenletResource:
		return h.admitCreateWithResourcePrefix(gardenletShootInfo, request)

	case leaseResource:
		return h.admitCreateWithResourcePrefix(gardenletShootInfo, request)

	case secretResource:
		return h.admitSecret(ctx, gardenletShootInfo, request)

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

func (h *Handler) admitSecret(ctx context.Context, gardenletShootInfo types.NamespacedName, request admission.Request) admission.Response {
	if request.Operation != admissionv1.Create {
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("unexpected operation: %q", request.Operation))
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

func (h *Handler) admitCreateWithResourcePrefix(gardenletShootInfo types.NamespacedName, request admission.Request) admission.Response {
	if request.Operation != admissionv1.Create {
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("unexpected operation: %q", request.Operation))
	}

	if !strings.HasPrefix(request.Name, gardenletutils.ResourcePrefixSelfHostedShoot) {
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("the resource for self-hosted shoots must be prefixed with %q", gardenletutils.ResourcePrefixSelfHostedShoot))
	}

	return h.admit(gardenletShootInfo, types.NamespacedName{Name: strings.TrimPrefix(request.Name, gardenletutils.ResourcePrefixSelfHostedShoot), Namespace: request.Namespace})
}
