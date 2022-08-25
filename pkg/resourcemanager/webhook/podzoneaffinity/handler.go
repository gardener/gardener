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

package podzoneaffinity

import (
	"context"
	"encoding/json"
	"net/http"

	gardencorev1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var podGVK = metav1.GroupVersionKind{Group: "", Kind: "Pod", Version: "v1"}

type handler struct {
	client  client.Client
	decoder *admission.Decoder
}

// NewHandler returns a new handler.
func NewHandler() admission.Handler {
	return &handler{}
}

func (h *handler) InjectClient(cl client.Client) error {
	h.client = cl
	return nil
}

func (h *handler) InjectDecoder(d *admission.Decoder) error {
	h.decoder = d
	return nil
}

func (h *handler) Handle(ctx context.Context, req admission.Request) admission.Response {
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

	// Check conflicting and add required pod affinity terms.
	handlePodAffinity(pod)

	// If the concrete zone is already determined by Gardener, let the pod be scheduled only to nodes in that zone.
	if err := handleNodeAffinity(ctx, h.client, pod); err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	marshaledPod, err := json.Marshal(pod)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledPod)
}

func handlePodAffinity(pod *corev1.Pod) {
	affinityTerm := corev1.PodAffinityTerm{
		// Use empty label selector to match any pods in the shoot control plane namespace.
		LabelSelector: &metav1.LabelSelector{},
		TopologyKey:   corev1.LabelTopologyZone,
	}

	// First remove potentially conflicting pod anti affinity terms that would forbid scheduling into a single zone.
	removeConflictingPodAntiAffinityTerms(pod, affinityTerm)

	// Handle required affinity to let the pod be scheduled to a specific zone.
	handleZonePodAffinityTerm(pod, affinityTerm)
}

func removeConflictingPodAntiAffinityTerms(pod *corev1.Pod, affinityTerm corev1.PodAffinityTerm) {
	if pod.Spec.Affinity == nil || pod.Spec.Affinity.PodAntiAffinity == nil {
		return
	}

	// Remove pod anti-affinity rules with zone topology key.
	remainingAntiAffinityTerms := make([]corev1.PodAffinityTerm, 0, len(pod.Spec.Affinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution))
	for _, term := range pod.Spec.Affinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution {
		if equality.Semantic.DeepEqual(term, affinityTerm) {
			continue
		}
		remainingAntiAffinityTerms = append(remainingAntiAffinityTerms, term)
	}
	pod.Spec.Affinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution = remainingAntiAffinityTerms
}

func handleZonePodAffinityTerm(pod *corev1.Pod, affinityTerm corev1.PodAffinityTerm) {
	if pod.Spec.Affinity == nil {
		pod.Spec.Affinity = &corev1.Affinity{}
	}
	if pod.Spec.Affinity.PodAffinity == nil {
		pod.Spec.Affinity.PodAffinity = &corev1.PodAffinity{}
	}

	var (
		zoneTermExisting      bool
		filteredAffinityTerms = make([]corev1.PodAffinityTerm, 0, len(pod.Spec.Affinity.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution))
	)

	for _, term := range pod.Spec.Affinity.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution {
		if equality.Semantic.DeepEqual(term, affinityTerm) {
			zoneTermExisting = true
		}

		// If there is another affinity configured on zones, we assume that this will be conflicting.
		if term.TopologyKey == corev1.LabelTopologyZone && !zoneTermExisting {
			continue
		}

		filteredAffinityTerms = append(filteredAffinityTerms, term)
	}

	pod.Spec.Affinity.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution = filteredAffinityTerms

	// Add pod affinity for zone if not already available.
	if !zoneTermExisting {
		pod.Spec.Affinity.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution = append(pod.Spec.Affinity.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution, affinityTerm)
	}
}

func handleNodeAffinity(ctx context.Context, cl client.Client, pod *corev1.Pod) error {
	nodeSelector, err := getZoneSpecificNodeSelector(ctx, cl, pod.Namespace)
	if err != nil {
		return err
	}

	if nodeSelector == nil {
		return nil
	}

	if pod.Spec.Affinity.NodeAffinity == nil {
		pod.Spec.Affinity.NodeAffinity = &corev1.NodeAffinity{}
	}

	if pod.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution == nil {
		pod.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution = &corev1.NodeSelector{}
	}

	var (
		zoneTermExisting          bool
		filteredAntiAffinityTerms = make([]corev1.NodeSelectorTerm, 0, len(pod.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms))
	)

	for _, term := range pod.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms {
		// Check if node affinity already exists.
		if equality.Semantic.DeepEqual(term, nodeSelector) {
			zoneTermExisting = true
		}

		// Check conflicting affinity terms.
		for _, expr := range term.MatchExpressions {
			if expr.Key == corev1.LabelTopologyZone && !zoneTermExisting {
				continue
			}
			filteredAntiAffinityTerms = append(filteredAntiAffinityTerms, term)
		}
	}

	pod.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms = filteredAntiAffinityTerms

	// Add node affinity for zone if not already available.
	if !zoneTermExisting {
		pod.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms = append(pod.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms, *nodeSelector)
	}

	return nil
}

func getZoneSpecificNodeSelector(ctx context.Context, cl client.Client, namespace string) (*corev1.NodeSelectorTerm, error) {
	namespaceObj := &corev1.Namespace{}
	if err := cl.Get(ctx, kutil.Key(namespace), namespaceObj); err != nil {
		return nil, err
	}

	// Check if scheduling to a specific zone is required.
	var nodeSelector *corev1.NodeSelectorTerm
	zone := namespaceObj.Labels[gardencorev1beta1constants.ShootZonePinning]
	if zone != "" {
		nodeSelector = &corev1.NodeSelectorTerm{
			MatchExpressions: []corev1.NodeSelectorRequirement{
				{
					Key:      corev1.LabelTopologyZone,
					Operator: corev1.NodeSelectorOpIn,
					Values: []string{
						zone,
					},
				},
			},
		}
	}

	return nodeSelector, nil
}
