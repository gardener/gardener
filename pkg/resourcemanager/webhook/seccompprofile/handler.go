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

package seccompprofile

import (
	"context"
	"encoding/json"
	"net/http"

	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	"github.com/go-logr/logr"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var podGVK = metav1.GroupVersionKind{Group: "", Kind: "Pod", Version: "v1"}

// Handler is capable of handling admission requests.
type Handler struct {
	logger  logr.Logger
	decoder *admission.Decoder
}

// NewHandler returns a new handler.
func NewHandler(logger logr.Logger) Handler {
	return Handler{logger: logger}
}

// InjectDecoder injects a decoder into the handler.
func (h *Handler) InjectDecoder(d *admission.Decoder) error {
	h.decoder = d
	return nil
}

// Handle returns a response to an AdmissionRequest.
func (h *Handler) Handle(_ context.Context, req admission.Request) admission.Response {
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

	// Do not overwrite the seccomp profile if it is already specified
	if pod.Spec.SecurityContext != nil && pod.Spec.SecurityContext.SeccompProfile != nil && pod.Spec.SecurityContext.SeccompProfile.Type != "" {
		return admission.Allowed("seccomp profile is explicitly specified")
	}

	if pod.Spec.SecurityContext == nil {
		pod.Spec.SecurityContext = &corev1.PodSecurityContext{}
	}

	pod.Spec.SecurityContext.SeccompProfile = &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault}

	marshaledPod, err := json.Marshal(pod)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	log := h.logger.WithValues("pod", kutil.ObjectKeyForCreateWebhooks(pod))
	log.Info("Mutating pod with default seccomp profile")
	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledPod)
}
