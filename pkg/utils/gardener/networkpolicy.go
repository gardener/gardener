// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardener

import (
	"encoding/json"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/utils"
)

// InjectNetworkPolicyAnnotationsForScrapeTargets injects the provided ports into the
// `networking.resources.gardener.cloud/from-all-scrape-targets-allowed-ports` annotation of the given service.
func InjectNetworkPolicyAnnotationsForScrapeTargets(service *corev1.Service, ports ...networkingv1.NetworkPolicyPort) error {
	return injectNetworkPolicyAnnotationsForScrapeTargets(service, v1beta1constants.LabelNetworkPolicyScrapeTargets, ports...)
}

// InjectNetworkPolicyAnnotationsForGardenScrapeTargets injects the provided ports into the
// `networking.resources.gardener.cloud/from-all-garden-scrape-targets-allowed-ports` annotation of the given service.
func InjectNetworkPolicyAnnotationsForGardenScrapeTargets(service *corev1.Service, ports ...networkingv1.NetworkPolicyPort) error {
	return injectNetworkPolicyAnnotationsForScrapeTargets(service, v1beta1constants.LabelNetworkPolicyGardenScrapeTargets, ports...)
}

// InjectNetworkPolicyAnnotationsForSeedScrapeTargets injects the provided ports into the
// `networking.resources.gardener.cloud/from-all-seed-scrape-targets-allowed-ports` annotation of the given service.
func InjectNetworkPolicyAnnotationsForSeedScrapeTargets(service *corev1.Service, ports ...networkingv1.NetworkPolicyPort) error {
	return injectNetworkPolicyAnnotationsForScrapeTargets(service, v1beta1constants.LabelNetworkPolicySeedScrapeTargets, ports...)
}

// InjectNetworkPolicyAnnotationsForWebhookTargets injects the provided ports into the
// `networking.resources.gardener.cloud/from-all-webhook-targets-allowed-ports` annotation of the given service.
func InjectNetworkPolicyAnnotationsForWebhookTargets(service *corev1.Service, ports ...networkingv1.NetworkPolicyPort) error {
	return injectNetworkPolicyAnnotationsForScrapeTargets(service, v1beta1constants.LabelNetworkPolicyWebhookTargets, ports...)
}

// InjectNetworkPolicyAnnotationsForScrapeTargets injects the provided ports into the
// `networking.resources.gardener.cloud/from-<podLabelSelector>-allowed-ports` annotation of the given service.
func injectNetworkPolicyAnnotationsForScrapeTargets(service *corev1.Service, podLabelSelector string, ports ...networkingv1.NetworkPolicyPort) error {
	rawPorts, err := json.Marshal(ports)
	if err != nil {
		return err
	}

	metav1.SetMetaDataAnnotation(&service.ObjectMeta, resourcesv1alpha1.NetworkPolicyFromPolicyAnnotationPrefix+podLabelSelector+resourcesv1alpha1.NetworkPolicyFromPolicyAnnotationSuffix, string(rawPorts))
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
func NetworkPolicyLabel(serviceName string, port int32) string {
	labelKey, _ := ShortenNetworkPolicyLabelKeyIfTooLong(fmt.Sprintf("%sto-%s-tcp-%d", resourcesv1alpha1.NetworkPolicyLabelKeyPrefix, serviceName, port))
	return labelKey
}

// ShortenNetworkPolicyLabelKeyIfTooLong shortens the given label key if it exceeds the maximum length for Kubernetes label keys.
func ShortenNetworkPolicyLabelKeyIfTooLong(labelKey string) (string, bool) {
	const maxLabelKeyLength = 63

	if keyWithoutPrefix := strings.TrimPrefix(labelKey, resourcesv1alpha1.NetworkPolicyLabelKeyPrefix); len(keyWithoutPrefix) > maxLabelKeyLength {
		newKey := resourcesv1alpha1.NetworkPolicyLabelKeyPrefix + keyWithoutPrefix[:maxLabelKeyLength-6] + "-" + utils.ComputeSHA256Hex([]byte(keyWithoutPrefix))[:5]

		return newKey, true
	}

	return labelKey, false
}
