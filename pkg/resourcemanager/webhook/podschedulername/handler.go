// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"net/http"

	"gomodules.xyz/jsonpatch/v2"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var podGVK = metav1.GroupVersionKind{Group: "", Kind: "Pod", Version: "v1"}

type handler struct {
	decoder       *admission.Decoder
	schedulerName string
}

// NewHandler returns a new handler.
func NewHandler(schedulerName string) admission.Handler {
	return &handler{
		schedulerName: schedulerName,
	}
}

func (h *handler) InjectDecoder(d *admission.Decoder) error {
	h.decoder = d
	return nil
}

func (h *handler) Handle(_ context.Context, req admission.Request) admission.Response {
	if req.Operation != admissionv1.Create {
		return admission.Allowed("only 'create' operation is handled")
	}

	if req.Kind != podGVK {
		return admission.Allowed("resource is not corev1.Pod")
	}

	if req.SubResource != "" {
		return admission.Allowed("subresources on pods are not supported")
	}

	pod := &corev1.Pod{}
	if err := h.decoder.Decode(req, pod); err != nil {
		return admission.Errored(http.StatusUnprocessableEntity, err)
	}

	// Do not overwrite the scheduler name when a custom scheduler name is specified
	if pod.Spec.SchedulerName != "" && pod.Spec.SchedulerName != corev1.DefaultSchedulerName {
		return admission.Allowed("custom scheduler is specified")
	}

	return admission.Patched(
		fmt.Sprintf("scheduler '%s' is configured", h.schedulerName),
		jsonpatch.NewOperation("replace", "/spec/schedulerName", h.schedulerName),
	)
}
