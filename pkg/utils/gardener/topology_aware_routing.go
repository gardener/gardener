// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardener

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
)

// ReconcileTopologyAwareRoutingMetadata adds (or removes) the required annotation and label to make a Service topology-aware.
func ReconcileTopologyAwareRoutingMetadata(service *corev1.Service, topologyAwareRoutingEnabled bool) {
	if topologyAwareRoutingEnabled {
		metav1.SetMetaDataAnnotation(&service.ObjectMeta, corev1.AnnotationTopologyMode, "auto")
		delete(service.Annotations, corev1.DeprecatedAnnotationTopologyAwareHints)
		metav1.SetMetaDataLabel(&service.ObjectMeta, resourcesv1alpha1.EndpointSliceHintsConsider, "true")
	} else {
		delete(service.Annotations, corev1.AnnotationTopologyMode)
		delete(service.Annotations, corev1.DeprecatedAnnotationTopologyAwareHints)
		delete(service.Labels, resourcesv1alpha1.EndpointSliceHintsConsider)
	}
}
