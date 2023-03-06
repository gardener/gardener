// Copyright (c) 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package gardener

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
)

// ReconcileTopologyAwareRoutingMetadata adds (or removes) the required annotation and label to make a Service topology-aware.
func ReconcileTopologyAwareRoutingMetadata(service *corev1.Service, topologyAwareRoutingEnabled bool) {
	if topologyAwareRoutingEnabled {
		metav1.SetMetaDataAnnotation(&service.ObjectMeta, corev1.AnnotationTopologyAwareHints, "auto")
		metav1.SetMetaDataLabel(&service.ObjectMeta, resourcesv1alpha1.EndpointSliceHintsConsider, "true")
	} else {
		delete(service.Annotations, corev1.AnnotationTopologyAwareHints)
		delete(service.Labels, resourcesv1alpha1.EndpointSliceHintsConsider)
	}
}
