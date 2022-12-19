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

package kubernetes

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
)

// GetReplicaCount returns the replica count based on the criteria, failure tolerance type, and component type.
func GetReplicaCount(failureToleranceType *gardencorev1beta1.FailureToleranceType, componentType string) *int32 {
	if len(componentType) == 0 {
		return nil
	}

	if failureToleranceType != nil &&
		*failureToleranceType == "" &&
		componentType == resourcesv1alpha1.HighAvailabilityConfigTypeController {
		return pointer.Int32(1)
	}

	return pointer.Int32(2)
}

// GetNodeSelectorRequirementForZones returns a node selector requirement to ensure all pods are scheduled only on
// nodes in the provided zones. If no zones are provided then nothing is done.
// Note that the returned requirement should be added to all existing node selector terms in the
// spec.affinity.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms field of pods because
// the various node selector terms are evaluated with the OR operator.
func GetNodeSelectorRequirementForZones(isZonePinningEnabled bool, zones []string) *corev1.NodeSelectorRequirement {
	if len(zones) == 0 || !isZonePinningEnabled {
		return nil
	}

	return &corev1.NodeSelectorRequirement{
		Key:      corev1.LabelTopologyZone,
		Operator: corev1.NodeSelectorOpIn,
		Values:   zones,
	}
}

// GetTopologySpreadConstraints adds topology spread constraints based on the passed `failureToleranceType`. This is
// only done when the number of replicas is greater than 1 (otherwise, it doesn't make sense to add spread constraints).
func GetTopologySpreadConstraints(
	replicas int32,
	maxReplicas int32,
	labelSelector metav1.LabelSelector,
	numberOfZones int32,
	failureToleranceType *gardencorev1beta1.FailureToleranceType,
) []corev1.TopologySpreadConstraint {
	if replicas <= 1 {
		return nil
	}

	whenUnsatisfiable := corev1.ScheduleAnyway
	if failureToleranceType != nil && *failureToleranceType != "" {
		whenUnsatisfiable = corev1.DoNotSchedule
	}

	topologySpreadConstraints := []corev1.TopologySpreadConstraint{{
		TopologyKey:       corev1.LabelHostname,
		MaxSkew:           1,
		WhenUnsatisfiable: whenUnsatisfiable,
		LabelSelector:     &labelSelector,
	}}

	// We only want to enforce a spread over zones when there are:
	// - multiple zones
	// - AND
	// - the failure tolerance type is 'nil' (seed/shoot system component case) or 'zone' (shoot control-plane case)
	if numberOfZones > 1 && (failureToleranceType == nil || *failureToleranceType == gardencorev1beta1.FailureToleranceTypeZone) {
		maxSkew := int32(1)
		// Increase maxSkew if there are >= 2*numberOfZones maxReplicas, see https://github.com/kubernetes/kubernetes/issues/109364.
		if maxReplicas >= 2*numberOfZones {
			maxSkew = 2
		}

		topologySpreadConstraints = append(topologySpreadConstraints, corev1.TopologySpreadConstraint{
			TopologyKey:       corev1.LabelTopologyZone,
			MaxSkew:           maxSkew,
			WhenUnsatisfiable: corev1.DoNotSchedule,
			LabelSelector:     &labelSelector,
		})
	}

	return topologySpreadConstraints
}
