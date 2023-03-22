// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
