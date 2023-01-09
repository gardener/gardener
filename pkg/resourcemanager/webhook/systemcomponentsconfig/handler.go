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

package systemcomponentsconfig

import (
	"context"
	"fmt"
	"strings"

	"github.com/gardener/gardener/pkg/utils"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilsets "k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// Handler contains required nodeSelector and tolerations information.
type Handler struct {
	Logger       logr.Logger
	TargetClient client.Client

	// NodeSelector is the selector used to retrieve nodes considered when calculating the tolerations.
	NodeSelector map[string]string
	// PodNodeSelector are the key-value pairs that should be added to each pod.
	PodNodeSelector map[string]string
	// PodTolerations are the tolerations that should be added to each pod.
	PodTolerations []corev1.Toleration
}

// Default sets the spec.nodeSelector and spec.Tolerations fields the podTemplate.
func (h *Handler) Default(ctx context.Context, obj runtime.Object) error {
	pod, ok := obj.(*corev1.Pod)
	if !ok {
		return fmt.Errorf("expected *corev1.Pod but got %T", obj)
	}

	req, err := admission.RequestFromContext(ctx)
	if err != nil {
		return err
	}

	log := h.Logger.WithValues("pod", kubernetesutils.ObjectKeyForCreateWebhooks(pod, req))

	if kubernetesutils.PodManagedByDaemonSet(pod) {
		log.Info("Pod is managed by DaemonSet, skipping further handling")
		return nil
	}

	log.Info("Handle node selector and system component tolerations")

	// Add configured node selectors to pod.
	h.handleNodeSelector(pod)

	// Add tolerations for workers which allow system components.
	return h.handleTolerations(ctx, log, pod)
}

func (h *Handler) handleNodeSelector(pod *corev1.Pod) {
	pod.Spec.NodeSelector = utils.MergeStringMaps(pod.Spec.NodeSelector, h.PodNodeSelector)
}

func (h *Handler) handleTolerations(ctx context.Context, log logr.Logger, pod *corev1.Pod) error {
	nodeList := &corev1.NodeList{}
	if err := h.TargetClient.List(ctx, nodeList, client.MatchingLabels(h.NodeSelector)); err != nil {
		return err
	}

	var (
		tolerations = utilsets.New[corev1.Toleration]()

		// We need to use semantically equal tolerations, i.e. equality of underlying values of pointers,
		// before they are added to the tolerations set.
		comparableTolerations = &kubernetesutils.ComparableTolerations{}
	)

	// Add existing tolerations from pod to map.
	for _, existingToleration := range pod.Spec.Tolerations {
		tolerations = tolerations.Insert(comparableTolerations.Transform(existingToleration))
	}

	// Add tolerations from webhook configuration to map.
	for _, toleration := range h.PodTolerations {
		tolerations = tolerations.Insert(comparableTolerations.Transform(toleration))
	}

	// Add required tolerations for existing system component nodes.
	for _, node := range nodeList.Items {
		for _, taint := range node.Spec.Taints {
			// Kubernetes reserved taints must not be added which would circumvent taint based evictions, see https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/#taint-based-evictions.
			if kubernetesReservedTaint(taint) {
				log.Info("Kubernetes reserved taint is skipped for toleration calculation", "node", client.ObjectKeyFromObject(&node), "taint", taint.Key)
				continue
			}
			toleration := kubernetesutils.TolerationForTaint(taint)
			tolerations = tolerations.Insert(comparableTolerations.Transform(toleration))
		}
	}

	pod.Spec.Tolerations = tolerations.UnsortedList()

	return nil
}

func kubernetesReservedTaint(taint corev1.Taint) bool {
	return strings.Contains(taint.Key, "kubernetes.io/")
}
