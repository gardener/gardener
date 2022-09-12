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

package podtopologyspreadconstraints

import (
	"context"
	"encoding/json"
	"net/http"

	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/go-logr/logr"

	appsv1 "k8s.io/api/apps/v1"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var podGVK = metav1.GroupVersionKind{Group: "", Kind: "Pod", Version: "v1"}

type handler struct {
	decoder *admission.Decoder
	logger  logr.Logger
}

// NewHandler returns a new handler.
func NewHandler(logger logr.Logger) admission.Handler {
	return &handler{
		logger: logger,
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

	templateHash, ok := pod.Labels[appsv1.DefaultDeploymentUniqueLabelKey]
	if !ok {
		return admission.Allowed("no pod-template-hash label available")
	}

	if len(pod.Spec.TopologySpreadConstraints) == 0 {
		return admission.Allowed("no topology spread constraints defined")
	}

	for i, constraint := range pod.Spec.TopologySpreadConstraints {
		if hasPodTemplateHashSelector(constraint.LabelSelector) {
			continue
		}
		if pod.Spec.TopologySpreadConstraints[i].LabelSelector == nil {
			pod.Spec.TopologySpreadConstraints[i].LabelSelector = &metav1.LabelSelector{}
		}

		if pod.Spec.TopologySpreadConstraints[i].LabelSelector.MatchLabels == nil {
			pod.Spec.TopologySpreadConstraints[i].LabelSelector.MatchLabels = map[string]string{}
		}

		// This selector mimics the `matchLabelKeys` (alpha in `v1.25`) on `pod-template-hash` which is required to consider
		// TSC configuration for rolling updates.
		// See https://kubernetes.io/docs/concepts/scheduling-eviction/topology-spread-constraints/#spread-constraint-definition,
		// https://github.com/kubernetes/kubernetes/issues/98215
		pod.Spec.TopologySpreadConstraints[i].LabelSelector.MatchLabels[appsv1.DefaultDeploymentUniqueLabelKey] = templateHash
	}

	marshaledPod, err := json.Marshal(pod)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	log := h.logger.WithValues("pod", kutil.ObjectKeyForCreateWebhooks(pod))
	log.Info("Mutating topology spread constraint label selector")
	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledPod)
}

func hasPodTemplateHashSelector(selector *metav1.LabelSelector) bool {
	if selector == nil {
		return false
	}
	if _, ok := selector.MatchLabels[appsv1.DefaultDeploymentUniqueLabelKey]; ok {
		return true
	}
	for _, expression := range selector.MatchExpressions {
		if expression.Operator != metav1.LabelSelectorOpIn {
			continue
		}
		if expression.Key == appsv1.DefaultDeploymentUniqueLabelKey {
			return true
		}
	}
	return false
}
