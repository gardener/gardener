// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package prometheus

import (
	pvcautoscalerv1alpha1 "github.com/gardener/pvc-autoscaler/api/autoscaling/v1alpha1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/utils"
)

func (p *prometheus) pvca(maxCapacity resource.Quantity) *pvcautoscalerv1alpha1.PersistentVolumeClaimAutoscaler {
	obj := &pvcautoscalerv1alpha1.PersistentVolumeClaimAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      p.name(),
			Namespace: p.namespace,
			Labels: utils.MergeStringMaps(p.getLabels(), map[string]string{
				v1beta1constants.LabelObservabilityApplication: p.name(),
			}),
		},
		Spec: pvcautoscalerv1alpha1.PersistentVolumeClaimAutoscalerSpec{
			TargetRef: autoscalingv1.CrossVersionObjectReference{
				APIVersion: "monitoring.coreos.com/v1",
				Kind:       "Prometheus",
				Name:       p.values.Name,
			},
			VolumePolicies: []pvcautoscalerv1alpha1.VolumePolicy{
				{
					MaxCapacity: maxCapacity,
				},
			},
		},
	}

	return obj
}
