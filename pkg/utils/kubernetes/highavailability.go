// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package kubernetes

import (
	"slices"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

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
		return ptr.To[int32](1)
	}

	return ptr.To[int32](2)
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
	enforceSpreadAcrossHosts bool,
) []corev1.TopologySpreadConstraint {
	if replicas <= 1 {
		return nil
	}

	var (
		// Enforcing a spread over zones is required when there are:
		// - multiple zones
		// - AND
		// - the failure tolerance type is 'nil' (seed/shoot system component case) or 'zone' (shoot control-plane case)
		zoneSpreadRequired = numberOfZones > 1 && (failureToleranceType == nil || *failureToleranceType == gardencorev1beta1.FailureToleranceTypeZone)

		// Enforcing a spread over hosts is required when:
		// - it is explicitly requested via 'enforceSpreadAcrossHosts' argument
		// - OR
		// - a failure tolerance type is set
		hostSpreadRequired = enforceSpreadAcrossHosts || ptr.Deref(failureToleranceType, "") != ""

		minDomainsHosts   *int32
		whenUnsatisfiable = corev1.ScheduleAnyway
	)

	if hostSpreadRequired {
		whenUnsatisfiable = corev1.DoNotSchedule
		minDomainsHosts = calculateMinDomains(3, maxReplicas)
	}

	topologySpreadConstraints := []corev1.TopologySpreadConstraint{{
		TopologyKey:       corev1.LabelHostname,
		MinDomains:        minDomainsHosts,
		MaxSkew:           1,
		WhenUnsatisfiable: whenUnsatisfiable,
		LabelSelector:     &labelSelector,
	}}

	if zoneSpreadRequired {
		topologySpreadConstraints = append(topologySpreadConstraints, corev1.TopologySpreadConstraint{
			TopologyKey:       corev1.LabelTopologyZone,
			MinDomains:        calculateMinDomains(numberOfZones, maxReplicas),
			MaxSkew:           1,
			WhenUnsatisfiable: corev1.DoNotSchedule,
			LabelSelector:     &labelSelector,
		})
	}

	return topologySpreadConstraints
}

func calculateMinDomains(numDomains, maxReplicas int32) *int32 {
	// If the maximum replica count is lower than the number of domains (e.g. zone or node count), then we only need to set 'minDomains' to
	// the number of replicas because there is no benefit of enforcing a further zone spread for additional replicas,
	// e.g. when a rolling update is performed.
	if maxReplicas < numDomains {
		return ptr.To(maxReplicas)
	}
	// Return the number of zones otherwise because it's not possible to spread pods over more zones than there are available.
	return ptr.To(numDomains)
}

// MutateMatchLabelKeys mutates the matchLabelKeys of the given topologySpreadConstraint by adding the "pod-template-hash" label key if it is not already present and removing it from the label selector match labels.
func MutateMatchLabelKeys(topologySpreadConstraints []corev1.TopologySpreadConstraint) {
	for i := range topologySpreadConstraints {
		if !slices.Contains(topologySpreadConstraints[i].MatchLabelKeys, appsv1.DefaultDeploymentUniqueLabelKey) {
			topologySpreadConstraints[i].MatchLabelKeys = append(topologySpreadConstraints[i].MatchLabelKeys, appsv1.DefaultDeploymentUniqueLabelKey)
		}
	}
}
