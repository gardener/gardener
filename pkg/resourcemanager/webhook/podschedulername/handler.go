// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package podschedulername

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// Handler handles admission requests and sets the spec.schedulerName field in Pod resources.
type Handler struct {
	SchedulerName string
}

// Default defaults the scheduler name of the provided pod.
func (h *Handler) Default(_ context.Context, obj runtime.Object) error {
	pod, ok := obj.(*corev1.Pod)
	if !ok {
		return fmt.Errorf("expected *corev1.Pod but got %T", obj)
	}

	// Only overwrite the scheduler name when no custom scheduler name is specified
	if pod.Spec.SchedulerName == "" || pod.Spec.SchedulerName == corev1.DefaultSchedulerName {
		pod.Spec.SchedulerName = h.SchedulerName
	}

	return nil
}
