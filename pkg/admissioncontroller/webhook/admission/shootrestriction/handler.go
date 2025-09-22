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
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/gardener/gardener/pkg/admissioncontroller/gardenletidentity"
	shootidentity "github.com/gardener/gardener/pkg/admissioncontroller/gardenletidentity/shoot"
	admissionwebhook "github.com/gardener/gardener/pkg/admissioncontroller/webhook/admission"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	gardenletutils "github.com/gardener/gardener/pkg/utils/gardener/gardenlet"
)

var (
	// Only take v1beta1 for the core.gardener.cloud API group because the Authorize function only checks the resource
	// group and the resource (but it ignores the version).
	certificateSigningRequestResource = certificatesv1.Resource("certificatesigningrequests")
	gardenletResource                 = seedmanagementv1alpha1.Resource("gardenlets")
)

// Handler restricts requests made by shoot gardenlets.
type Handler struct {
	Logger  logr.Logger
	Client  client.Reader
	Decoder admission.Decoder
}

// Handle restricts requests made by gardenlets.
func (h *Handler) Handle(_ context.Context, request admission.Request) admission.Response {
	shootNamespace, shootName, isAutonomousShoot, userType := shootidentity.FromAuthenticationV1UserInfo(request.UserInfo)
	if !isAutonomousShoot {
		return admissionwebhook.Allowed("")
	}

	var (
		log                = h.Logger.WithValues("shootNamespace", shootNamespace, "shootName", shootName, "userType", userType)
		gardenletShootInfo = types.NamespacedName{Name: shootName, Namespace: shootNamespace}
	)

	requestResource := schema.GroupResource{Group: request.Resource.Group, Resource: request.Resource.Resource}
	switch requestResource {
	case certificateSigningRequestResource:
		return h.admitCertificateSigningRequest(gardenletShootInfo, userType, request)

	case gardenletResource:
		return h.admitGardenlet(gardenletShootInfo, request)

	default:
		log.Info(
			"Unhandled resource request",
			"group", request.Kind.Group,
			"version", request.Kind.Version,
			"resource", request.Resource.Resource,
		)
	}

	return admissionwebhook.Allowed("")
}

func (h *Handler) admitCertificateSigningRequest(gardenletShootInfo types.NamespacedName, userType gardenletidentity.UserType, request admission.Request) admission.Response {
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

	if ok, reason := gardenerutils.IsShootClientCert(x509cr, csr.Spec.Usages); !ok {
		return admission.Errored(http.StatusForbidden, fmt.Errorf("can only create CSRs for shoot clusters: %s", reason))
	}

	namespace, name, _, _ := shootidentity.FromCertificateSigningRequest(x509cr)
	return h.admit(gardenletShootInfo, types.NamespacedName{Name: name, Namespace: namespace})
}

func (h *Handler) admitGardenlet(gardenletShootInfo types.NamespacedName, request admission.Request) admission.Response {
	if request.Operation != admissionv1.Create {
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("unexpected operation: %q", request.Operation))
	}

	if !strings.HasPrefix(request.Name, gardenletutils.ResourcePrefixAutonomousShoot) {
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("the Gardenlet resources for autonomous shoots must be prefixed with %q", gardenletutils.ResourcePrefixAutonomousShoot))
	}

	return h.admit(gardenletShootInfo, types.NamespacedName{Name: strings.TrimPrefix(request.Name, gardenletutils.ResourcePrefixAutonomousShoot), Namespace: request.Namespace})
}

func (h *Handler) admit(gardenletShootInfo, objectShootInfo types.NamespacedName) admission.Response {
	// Allow request if the shoot the gardenlet is responsible for matches with the shoot related to the object.
	if gardenletShootInfo.Name == objectShootInfo.Name && gardenletShootInfo.Namespace == objectShootInfo.Namespace {
		return admission.Allowed("")
	}

	return admission.Errored(http.StatusForbidden, fmt.Errorf("object does not belong to shoot %s", gardenletShootInfo))
}
