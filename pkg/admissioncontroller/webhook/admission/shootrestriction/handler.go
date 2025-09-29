// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shootrestriction

import (
	"context"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	shootidentity "github.com/gardener/gardener/pkg/admissioncontroller/gardenletidentity/shoot"
	admissionwebhook "github.com/gardener/gardener/pkg/admissioncontroller/webhook/admission"
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

	log := h.Logger.WithValues("shootNamespace", shootNamespace, "shootName", shootName, "userType", userType)

	requestResource := schema.GroupResource{Group: request.Resource.Group, Resource: request.Resource.Resource}
	switch requestResource {
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
