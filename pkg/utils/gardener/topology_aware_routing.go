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
	"github.com/gardener/gardener/pkg/utils/version"
)

// ReconcileTopologyAwareRouting adds (or removes) the required annotation, label and trafficDistribution setting to make a Service topology-aware.
func ReconcileTopologyAwareRouting(service *corev1.Service, topologyAwareRoutingEnabled bool, k8sVersion *semver.Version) {
	// remove settings for topologyAwareRouting, as these settings are controlled via this function.
	delete(service.Annotations, corev1.AnnotationTopologyMode)
	delete(service.Annotations, corev1.DeprecatedAnnotationTopologyAwareHints)
	delete(service.Labels, resourcesv1alpha1.EndpointSliceHintsConsider)
	service.Spec.TrafficDistribution = nil

	// return without topologyAwareRouting settings if disabled
	if !topologyAwareRoutingEnabled {
		return
	}

	// use trafficDistribution feature in kubernetes 1.31.x or above
	if version.ConstraintK8sGreaterEqual131.Check(k8sVersion) {
		service.Spec.TrafficDistribution = ptr.To(corev1.ServiceTrafficDistributionPreferClose)
		return
	}

	// use topology-mode auto and GRM webhook in kubernetes 1.27.x or above
	if version.ConstraintK8sGreaterEqual127.Check(k8sVersion) {
		metav1.SetMetaDataAnnotation(&service.ObjectMeta, corev1.AnnotationTopologyMode, "auto")
		metav1.SetMetaDataLabel(&service.ObjectMeta, resourcesv1alpha1.EndpointSliceHintsConsider, "true")
		return
	}

	// use topology-aware-hints auto and GRM webhook in kubernetes 1.26.x or below
	metav1.SetMetaDataAnnotation(&service.ObjectMeta, corev1.DeprecatedAnnotationTopologyAwareHints, "auto")
	metav1.SetMetaDataLabel(&service.ObjectMeta, resourcesv1alpha1.EndpointSliceHintsConsider, "true")
}

// ReconcileTopologyAwareRoutingMetadata adds (or removes) the required annotation, label and trafficDistribution setting to make a Service topology-aware.
// Deprecated: please use ReconcileTopologyAwareRouting instead.
func ReconcileTopologyAwareRoutingMetadata(service *corev1.Service, topologyAwareRoutingEnabled bool, k8sVersion *semver.Version) {
	ReconcileTopologyAwareRouting(service, topologyAwareRoutingEnabled, k8sVersion)
}
