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

	"github.com/go-logr/logr"
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
	logger  logr.Logger
}

// NewHandler returns a new handler.
func NewHandler(logger logr.Logger) admission.Handler {
	return &handler{
		logger: logger,
	}
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

	log := h.logger.WithValues("pod", client.ObjectKeyFromObject(pod))

	// Check conflicting and add required pod affinity terms.
	handlePodAffinity(log, pod)

	// If the concrete zone is already determined by Gardener, let the pod be scheduled only to nodes in that zone.
	if err := handleNodeAffinity(ctx, h.client, log, pod); err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	marshaledPod, err := json.Marshal(pod)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledPod)
}

func handlePodAffinity(log logr.Logger, pod *corev1.Pod) {
	affinityTerm := corev1.PodAffinityTerm{
		// Use empty label selector to match any pods in the shoot control plane namespace.
		LabelSelector: &metav1.LabelSelector{},
		TopologyKey:   corev1.LabelTopologyZone,
	}

	// First remove potentially conflicting pod anti affinity terms that would forbid scheduling into a single zone.
	removeConflictingPodAntiAffinityTerms(log, pod, affinityTerm)

	// Handle required affinity to let the pod be scheduled to a specific zone.
	handleZonePodAffinityTerm(log, pod, affinityTerm)
}

func removeConflictingPodAntiAffinityTerms(log logr.Logger, pod *corev1.Pod, affinityTerm corev1.PodAffinityTerm) {
	if pod.Spec.Affinity == nil || pod.Spec.Affinity.PodAntiAffinity == nil {
		return
	}

	// Filter out anti affinities that match the wanted `affinityTerm.
	pod.Spec.Affinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution = filterAffinityTerms(
		log.WithValues("type", "podAntiAffinity"),
		pod.Spec.Affinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution,
		func(t corev1.PodAffinityTerm) bool {
			return equality.Semantic.DeepEqual(t, affinityTerm)
		},
	)
}

func handleZonePodAffinityTerm(log logr.Logger, pod *corev1.Pod, affinityTerm corev1.PodAffinityTerm) {
	if pod.Spec.Affinity == nil {
		pod.Spec.Affinity = &corev1.Affinity{}
	}
	if pod.Spec.Affinity.PodAffinity == nil {
		pod.Spec.Affinity.PodAffinity = &corev1.PodAffinity{}
	}

	// Filter out any pod affinities based on the zone topology key as they potentially interfere with the wanted `affinityTerm`.
	pod.Spec.Affinity.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution = filterAffinityTerms(
		log.WithValues("type", "podAffinity"),
		pod.Spec.Affinity.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution,
		func(t corev1.PodAffinityTerm) bool {
			return t.TopologyKey == corev1.LabelTopologyZone
		})

	// Add pod affinity for zone if not already available.
	pod.Spec.Affinity.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution = append(pod.Spec.Affinity.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution, affinityTerm)
}

func filterAffinityTerms(log logr.Logger, terms []corev1.PodAffinityTerm, matchFn func(term corev1.PodAffinityTerm) bool) []corev1.PodAffinityTerm {
	filteredAffinityTerms := make([]corev1.PodAffinityTerm, 0, len(terms))
	for _, term := range terms {
		// If there is another term configured on zones, we assume that this will be conflicting.
		if matchFn(term) {
			log.Info("AffinityTerm is removed because of potential conflicts with zone affinity")
			continue
		}

		filteredAffinityTerms = append(filteredAffinityTerms, term)
	}
	return filteredAffinityTerms
}

func handleNodeAffinity(ctx context.Context, cl client.Client, log logr.Logger, pod *corev1.Pod) error {
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

	filteredAntiAffinityTerms := make([]corev1.NodeSelectorTerm, 0, len(pod.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms))

	for _, term := range pod.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms {
		// Check conflicting affinity terms.
		for _, expr := range term.MatchExpressions {
			if expr.Key == corev1.LabelTopologyZone {
				log.Info("NodeSelectorTerm is removed because of potential conflicts with zone affinity", "type", "nodeAffinity")
				continue
			}
			filteredAntiAffinityTerms = append(filteredAntiAffinityTerms, term)
		}
	}

	pod.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms = filteredAntiAffinityTerms

	// Add node affinity for zone if not already available.
	pod.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms = append(pod.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms, *nodeSelector)

	return nil
}

func getZoneSpecificNodeSelector(ctx context.Context, cl client.Client, namespace string) (*corev1.NodeSelectorTerm, error) {
	namespaceObj := &corev1.Namespace{}
	if err := cl.Get(ctx, kutil.Key(namespace), namespaceObj); err != nil {
		return nil, err
	}

	// Check if scheduling to a specific zone is required.
	var nodeSelector *corev1.NodeSelectorTerm
	if zone := namespaceObj.Labels[gardencorev1beta1constants.ShootControlPlaneEnforceZone]; zone != "" {
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
