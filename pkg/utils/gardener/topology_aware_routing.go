// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
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
	if versionutils.ConstraintK8sGreaterEqual134.Check(k8sVersion) {
		// For Kubernetes >= 1.34, only use the PreferSameZone strategy of the ServiceTrafficDistribution feature.
		// PreferClose is deprecated. PreferSameZone is a new alias for PreferClose (https://kubernetes.io/blog/2025/08/27/kubernetes-v1-34-release/#preferclose-traffic-distribution-is-deprecated).
		service.Spec.TrafficDistribution = ptr.To(corev1.ServiceTrafficDistributionPreferSameZone)
	} else if versionutils.ConstraintK8sGreaterEqual132.Check(k8sVersion) {
		// For Kubernetes >= 1.32, only use the PreferClose strategy of the ServiceTrafficDistribution feature.
		service.Spec.TrafficDistribution = ptr.To(corev1.ServiceTrafficDistributionPreferClose)
	} else {
		// For Kubernetes 1.31, use the PreferClose strategy of the ServiceTrafficDistribution feature in combination with the GRM's endpoints-slice-hints webhook.
		service.Spec.TrafficDistribution = ptr.To(corev1.ServiceTrafficDistributionPreferClose)
		metav1.SetMetaDataLabel(&service.ObjectMeta, resourcesv1alpha1.EndpointSliceHintsConsider, "true")
	}
}
