// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package shootserviceaccounts

import (
	"context"
	"fmt"
	"net/http"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/gardener/gardener/pkg/admissioncontroller/gardenletidentity"
	shootidentity "github.com/gardener/gardener/pkg/admissioncontroller/gardenletidentity/shoot"
	admissionwebhook "github.com/gardener/gardener/pkg/admissioncontroller/webhook/admission"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

// Handler enforces that the `extension-shoot--` ServiceAccount name prefix is reserved for self-hosted shoot gardenlets
// in project namespaces.
type Handler struct {
	Logger logr.Logger
}

// gardenerInternalServiceAccountUsername is the username of the `kube-system/gardener-internal` ServiceAccount. The
// gardener-operator uses the corresponding gardener-internal access secret to communicate with the garden cluster.
const gardenerInternalServiceAccountUsername = "system:serviceaccount:" + metav1.NamespaceSystem + ":" + v1beta1constants.SecretNameGardenerInternal

// Handle denies the request and logs the caller when the name prefix is reserved but the caller is not the owning
// shoot gardenlet.
func (h *Handler) Handle(_ context.Context, request admission.Request) admission.Response {
	requestResource := schema.GroupResource{Group: request.Resource.Group, Resource: request.Resource.Resource}
	if requestResource != corev1.Resource("serviceaccounts") {
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("unexpected resource: %q", requestResource))
	}

	var (
		shootName, isExtensionServiceAccount                                = gardenerutils.ParseExtensionShootServiceAccountName(request.Name)
		gardenletNamespace, gardenletShootName, isSelfHostedShoot, userType = shootidentity.FromAuthenticationV1UserInfo(request.UserInfo)

		isOwningGardenlet = isExtensionServiceAccount && isSelfHostedShoot && userType == gardenletidentity.UserTypeGardenlet &&
			gardenletNamespace == request.Namespace && gardenletShootName == shootName
		isGardenerOperator = request.UserInfo.Username == gardenerInternalServiceAccountUsername
	)

	if !isExtensionServiceAccount || (!isOwningGardenlet && !isGardenerOperator) {
		h.Logger.Info("Denied request for reserved ServiceAccount name prefix",
			"name", request.Name,
			"namespace", request.Namespace,
			"username", request.UserInfo.Username,
		)
		return admission.Errored(http.StatusForbidden, fmt.Errorf(
			"the %q prefix is reserved for the self-hosted shoot gardenlet responsible for %q (or the gardener-operator) and may not be used by other clients",
			v1beta1constants.ExtensionShootServiceAccountPrefix+shootName, shootName,
		))
	}

	return admissionwebhook.Allowed("")
}
