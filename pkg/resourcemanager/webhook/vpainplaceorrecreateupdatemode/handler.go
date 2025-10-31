// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package vpainplaceorrecreateupdatemode

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/ptr"
)

// Handler handles admission requests and sets the vpa.spec.updatePolicy.updateMode field in VerticalPodAutoscaler resources.
type Handler struct {
	Logger logr.Logger
}

// Default defaults the update mode of the provided VerticalPodAutoscaler.
func (h *Handler) Default(_ context.Context, obj runtime.Object) error {
	vpa, ok := obj.(*vpaautoscalingv1.VerticalPodAutoscaler)
	if !ok {
		return fmt.Errorf("expected *vpaautoscalingv1.VerticalPodAutoscaler but got %T", obj)
	}

	log := h.Logger.WithValues("vpa", vpa.GetName(), "namespace", vpa.GetNamespace())

	if vpa.Spec.UpdatePolicy == nil {
		vpa.Spec.UpdatePolicy = &vpaautoscalingv1.PodUpdatePolicy{}
	}

	updateMode := ptr.Deref(vpa.Spec.UpdatePolicy.UpdateMode, vpaautoscalingv1.UpdateModeAuto)
	if updateMode == vpaautoscalingv1.UpdateModeAuto || updateMode == vpaautoscalingv1.UpdateModeRecreate {
		log.Info("Mutating VerticalPodAutoscaler with InPlaceOrRecreate update mode")
		vpa.Spec.UpdatePolicy.UpdateMode = ptr.To(vpaautoscalingv1.UpdateModeInPlaceOrRecreate)
	}

	return nil
}
