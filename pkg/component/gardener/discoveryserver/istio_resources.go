// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package discoveryserver

import (
	"fmt"

	istionetworkingv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/utils/istio"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

func (g *gardenerDiscoveryServer) istioResources() ([]client.Object, error) {
	var exportTo = []string{operatorv1alpha1.VirtualGardenNamePrefix + v1beta1constants.DefaultSNIIngressNamespace}

	gateway := &istionetworkingv1beta1.Gateway{ObjectMeta: metav1.ObjectMeta{Name: deploymentName, Namespace: g.namespace}}
	if err := istio.GatewayWithTLSPassthrough(
		gateway,
		labels(),
		g.values.IstioIngressGatewayLabels,
		[]string{g.values.Domain},
	)(); err != nil {
		return nil, fmt.Errorf("failed to create gateway resource: %w", err)
	}

	destinationHost := kubernetesutils.FQDNForService(ServiceName, g.namespace)
	virtualService := &istionetworkingv1beta1.VirtualService{ObjectMeta: metav1.ObjectMeta{Name: deploymentName, Namespace: g.namespace}}
	if err := istio.VirtualServiceWithSNIMatch(
		virtualService,
		labels(),
		exportTo,
		[]string{g.values.Domain},
		deploymentName,
		portServer,
		destinationHost,
	)(); err != nil {
		return nil, fmt.Errorf("failed to create virtual service resource: %w", err)
	}

	destinationRule := &istionetworkingv1beta1.DestinationRule{ObjectMeta: metav1.ObjectMeta{Name: deploymentName, Namespace: g.namespace}}
	if err := istio.DestinationRuleWithLocalityPreference(destinationRule, labels(), exportTo, destinationHost)(); err != nil {
		return nil, fmt.Errorf("failed to create destination rule resource: %w", err)
	}

	return []client.Object{gateway, virtualService, destinationRule}, nil
}
