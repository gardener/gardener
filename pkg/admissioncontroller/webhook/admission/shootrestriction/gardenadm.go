// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shootrestriction

import (
	"fmt"
	"net/http"

	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

func (h *Handler) admitGardenadmRequests(gardenletShootInfo types.NamespacedName, request admission.Request) admission.Response {
	requestResource := schema.GroupResource{Group: request.Resource.Group, Resource: request.Resource.Resource}
	switch requestResource {
	case projectResource:
		if request.Operation != admissionv1.Create {
			return admission.Errored(http.StatusBadRequest, fmt.Errorf("unexpected operation: %q", request.Operation))
		}

		project := &gardencorev1beta1.Project{}
		if err := h.Decoder.Decode(request, project); err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}

		if gardenletShootInfo.Namespace == ptr.Deref(project.Spec.Namespace, "") {
			return admission.Allowed("")
		}

		return admission.Errored(http.StatusForbidden, fmt.Errorf("object does not belong to shoot %s", gardenletShootInfo))

	case shootResource:
		if request.Operation != admissionv1.Create {
			return admission.Errored(http.StatusBadRequest, fmt.Errorf("unexpected operation: %q", request.Operation))
		}

		shoot := &gardencorev1beta1.Shoot{}
		if err := h.Decoder.Decode(request, shoot); err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}

		if gardenletShootInfo.Namespace == shoot.Namespace && gardenletShootInfo.Name == shoot.Name {
			return admission.Allowed("")
		}

		return admission.Errored(http.StatusForbidden, fmt.Errorf("object does not belong to shoot %s", gardenletShootInfo))
	}

	return admission.Errored(http.StatusBadRequest, fmt.Errorf("unexpected resource: %q", requestResource))
}
