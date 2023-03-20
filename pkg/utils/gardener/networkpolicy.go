// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
)

// InjectNetworkPolicyAnnotationsForScrapeTargets injects the provided ports into the
// `networking.resources.gardener.cloud/from-policy-allowed-ports` annotation of the given service. In addition, it adds
// the well-known annotation for scrape targets of Prometheus in shoot namespaces.
func InjectNetworkPolicyAnnotationsForScrapeTargets(service *corev1.Service, ports ...networkingv1.NetworkPolicyPort) error {
	return injectNetworkPolicyAnnotationsForScrapeTargets(service, v1beta1constants.LabelNetworkPolicyScrapeTargets, ports...)
}

// InjectNetworkPolicyAnnotationsForSeedScrapeTargets injects the provided ports into the
// `networking.resources.gardener.cloud/from-policy-allowed-ports` annotation of the given service. In addition, it adds
// the well-known annotation for scrape targets of the Prometheis in the garden namespace.
func InjectNetworkPolicyAnnotationsForSeedScrapeTargets(service *corev1.Service, ports ...networkingv1.NetworkPolicyPort) error {
	return injectNetworkPolicyAnnotationsForScrapeTargets(service, v1beta1constants.LabelNetworkPolicySeedScrapeTargets, ports...)
}

// InjectNetworkPolicyAnnotationsForScrapeTargets injects the provided ports into the
// `networking.resources.gardener.cloud/from-policy-allowed-ports` annotation of the given service. In addition, it adds
// the annotation for scrape targets based on the provided label selector.
func injectNetworkPolicyAnnotationsForScrapeTargets(service *corev1.Service, podLabelSelector string, ports ...networkingv1.NetworkPolicyPort) error {
	rawPorts, err := json.Marshal(ports)
	if err != nil {
		return err
	}

	metav1.SetMetaDataAnnotation(&service.ObjectMeta, resourcesv1alpha1.NetworkingFromPolicyPodLabelSelector, podLabelSelector)
	metav1.SetMetaDataAnnotation(&service.ObjectMeta, resourcesv1alpha1.NetworkingFromPolicyAllowedPorts, string(rawPorts))
	return nil
}

// InjectNetworkPolicyNamespaceSelectors injects the provided selectors into the
// `networking.resources.gardener.cloud/namespace-selectors` annotation of the given service.
func InjectNetworkPolicyNamespaceSelectors(service *corev1.Service, selectors ...metav1.LabelSelector) error {
	rawSelectors, err := json.Marshal(selectors)
	if err != nil {
		return err
	}

	metav1.SetMetaDataAnnotation(&service.ObjectMeta, resourcesv1alpha1.NetworkingNamespaceSelectors, string(rawSelectors))
	return nil
}

// NetworkPolicyLabel returns the network policy label for a component initiating the connection to a service with the
// given name and TCP port.
func NetworkPolicyLabel(serviceName string, port int) string {
	return fmt.Sprintf("networking.resources.gardener.cloud/to-%s-tcp-%d", serviceName, port)
}
