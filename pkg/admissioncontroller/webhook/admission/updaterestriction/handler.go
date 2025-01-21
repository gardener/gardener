// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package updaterestriction

import (
	"context"
	"fmt"
	"slices"

	admissionv1 "k8s.io/api/admission/v1"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
)

// Handler handles resources that should not be modified
// from users other than the gardenlet.
type Handler struct{}

// Handle checks if the request is issued by a gardenlet
// and rejects the request if that is not the case.
func (h *Handler) Handle(_ context.Context, req admission.Request) admission.Response {
	// Allow the garbage-collector-controller to delete resources.
	// This is required as KCM is used to GC resources
	// that might be considered stale as their owner object is already gone.
	if req.UserInfo.Username == "system:serviceaccount:kube-system:generic-garbage-collector" {
		if req.Operation == admissionv1.Delete {
			return admission.Allowed("")
		}
		return admission.Denied(fmt.Sprintf("user %q is not allowed to %s system %s", req.UserInfo.Username, req.Operation, req.Resource.Resource))
	}

	if !slices.Contains(req.UserInfo.Groups, v1beta1constants.SeedsGroup) {
		return admission.Denied(fmt.Sprintf("user %q is not allowed to modify system %s", req.UserInfo.Username, req.Resource.Resource))
	}

	return admission.Allowed("")
}
