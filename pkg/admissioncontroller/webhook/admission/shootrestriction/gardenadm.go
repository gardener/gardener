// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shootrestriction

import (
	"context"
	"fmt"
	"net/http"

	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

func (h *Handler) admitGardenadmRequests(_ context.Context, gardenletShootInfo types.NamespacedName, request admission.Request) admission.Response {
	requestResource := schema.GroupResource{Group: request.Resource.Group, Resource: request.Resource.Resource}
	switch requestResource {
	case backupBucketResource:
		if request.Operation != admissionv1.Create {
			return admission.Errored(http.StatusBadRequest, fmt.Errorf("unexpected operation: %q", request.Operation))
		}

		backupBucket := &gardencorev1beta1.BackupBucket{}
		if err := h.Decoder.Decode(request, backupBucket); err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}

		if shootRef := backupBucket.Spec.ShootRef; shootRef != nil &&
			shootRef.Name == gardenletShootInfo.Name && shootRef.Namespace == gardenletShootInfo.Namespace {
			return admission.Allowed("")
		}

		return admission.Errored(http.StatusForbidden, fmt.Errorf("object does not belong to shoot %s", gardenletShootInfo))

	case backupEntryResource:
		if request.Operation != admissionv1.Create {
			return admission.Errored(http.StatusBadRequest, fmt.Errorf("unexpected operation: %q", request.Operation))
		}

		backupEntry := &gardencorev1beta1.BackupEntry{}
		if err := h.Decoder.Decode(request, backupEntry); err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}

		if shootRef := backupEntry.Spec.ShootRef; shootRef != nil &&
			shootRef.Name == gardenletShootInfo.Name && shootRef.Namespace == gardenletShootInfo.Namespace {
			return admission.Allowed("")
		}

		return admission.Errored(http.StatusForbidden, fmt.Errorf("object does not belong to shoot %s", gardenletShootInfo))

	case configMapResource, secretResource:
		if request.Operation != admissionv1.Create {
			return admission.Errored(http.StatusBadRequest, fmt.Errorf("unexpected operation: %q", request.Operation))
		}

		if gardenletShootInfo.Namespace == request.Namespace {
			return admission.Allowed("")
		}

		return admission.Errored(http.StatusForbidden, fmt.Errorf("object does not belong to shoot %s", gardenletShootInfo))

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
