// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package podtopologyspreadconstraints

import (
	"context"
	"fmt"
	"slices"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// Handler handles admission requests and sets the spec.topologySpreadConstraints field in Pod resources.
type Handler struct {
	Logger logr.Logger
}

// Default defaults the topology spread constraints of the provided pod.
func (h *Handler) Default(ctx context.Context, obj runtime.Object) error {
	pod, ok := obj.(*corev1.Pod)
	if !ok {
		return fmt.Errorf("expected *corev1.Pod but got %T", obj)
	}

	templateHash, ok := pod.Labels[appsv1.DefaultDeploymentUniqueLabelKey]
	if !ok {
		return nil
	}

	if len(pod.Spec.TopologySpreadConstraints) == 0 {
		return nil
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

		pod.Spec.TopologySpreadConstraints[i].MatchLabelKeys = slices.DeleteFunc(pod.Spec.TopologySpreadConstraints[i].MatchLabelKeys, func(key string) bool {
			return key == appsv1.DefaultDeploymentUniqueLabelKey
		})
	}

	req, err := admission.RequestFromContext(ctx)
	if err != nil {
		return err
	}

	log := h.Logger.WithValues("pod", kubernetesutils.ObjectKeyForCreateWebhooks(pod, req))
	log.Info("Mutating topology spread constraint label selector")
	return nil
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
