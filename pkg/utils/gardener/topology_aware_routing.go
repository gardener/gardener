// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardener

import (
	"github.com/Masterminds/semver/v3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	versionutils "github.com/gardener/gardener/pkg/utils/version"
)

// ReconcileTopologyAwareRoutingSettings adds or removes the required annotation, label and spec field to make a Service topology-aware.
//
// <k8sVersion> is the runtime cluster's Kubernetes version.
func ReconcileTopologyAwareRoutingSettings(service *corev1.Service, topologyAwareRoutingEnabled bool, k8sVersion *semver.Version) {
	delete(service.Annotations, corev1.AnnotationTopologyMode)
	delete(service.Annotations, corev1.DeprecatedAnnotationTopologyAwareHints)
	delete(service.Labels, resourcesv1alpha1.EndpointSliceHintsConsider)
	service.Spec.TrafficDistribution = nil

	if !topologyAwareRoutingEnabled {
		return
	}

	if versionutils.ConstraintK8sGreaterEqual132.Check(k8sVersion) {
		// For Kubernetes >= 1.32, only use the PreferClose strategy of the ServiceTrafficDistribution feature.
		service.Spec.TrafficDistribution = ptr.To(corev1.ServiceTrafficDistributionPreferClose)
	} else if versionutils.ConstraintK8sEqual131.Check(k8sVersion) {
		// For Kubernetes 1.31, use the PreferClose strategy of the ServiceTrafficDistribution feature in combination with the GRM's endpoints-slice-hints webhook.
		//
		// The webhook is still used to cover the migration case to prevent disabling the topology-aware routing feature during Kubernetes 1.30 -> 1.31 migration
		// for Services until they are reconciled in the next maintenance time window of the Shoot.
		service.Spec.TrafficDistribution = ptr.To(corev1.ServiceTrafficDistributionPreferClose)
		metav1.SetMetaDataLabel(&service.ObjectMeta, resourcesv1alpha1.EndpointSliceHintsConsider, "true")
	} else {
		// For Kubernetes < 1.31, use the TopologyAwareHints feature (with the new annotation key) in combination with the GRM's endpoints-slice-hints webhook.
		metav1.SetMetaDataAnnotation(&service.ObjectMeta, corev1.AnnotationTopologyMode, "auto")
		metav1.SetMetaDataLabel(&service.ObjectMeta, resourcesv1alpha1.EndpointSliceHintsConsider, "true")
	}
}

// ReconcileTopologyAwareRoutingMetadata adds or removes the required annotation, label and spec field to make a Service topology-aware.
//
// TODO(ialidzhikov): Remove this function after Gardener v1.119 has been released.
// Deprecated: Use ReconcileTopologyAwareRoutingSettings instead.
var ReconcileTopologyAwareRoutingMetadata = ReconcileTopologyAwareRoutingSettings
