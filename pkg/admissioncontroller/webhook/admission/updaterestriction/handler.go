// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package updaterestriction

import (
	"context"
	"fmt"
	"slices"

	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
)

// Handler handles resources that should not be modified
// from users other than the gardenlet.
type Handler struct{}

// Handle checks if the request is issued by a gardenlet
// and rejects the request if that is not the case.
func (h *Handler) Handle(_ context.Context, req admission.Request) admission.Response {
	if !slices.Contains(req.UserInfo.Groups, v1beta1constants.SeedsGroup) {
		return admission.Denied(fmt.Sprintf("user %q is not allowed to modify system %s", req.UserInfo.Username, req.Resource.Resource))
	}

	return admission.Allowed("")
}
