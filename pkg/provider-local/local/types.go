// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package local

import (
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// Name is the name of the local provider.
	Name = "provider-local"
	// Type is the type of resources managed by the local actuators.
	Type = "local"

	// FieldOwner is a constant for the owner name in `.metadata.managedFields`.
	FieldOwner = client.FieldOwner("gardener-extension-provider-local")

	// MachineControllerManagerName is a constant for the name of the machine-controller-manager.
	MachineControllerManagerName = "machine-controller-manager"
	// MachineControllerManagerVpaName is the name of the VerticalPodAutoscaler of the machine-controller-manager
	// deployment.
	MachineControllerManagerVpaName = "machine-controller-manager-vpa"
	// MachineControllerManagerMonitoringConfigName is the name of the ConfigMap containing monitoring stack
	// configurations for machine-controller-manager.
	MachineControllerManagerMonitoringConfigName = "machine-controller-manager-monitoring-config"

	// LabelNetworkPolicyToIstioIngressGateway allows Egress from pods labeled with
	// 'networking.gardener.cloud/to-istio-ingressgateway=allowed' to istio-ingressgateway pods running in
	// 'istio-ingress' namespace.
	LabelNetworkPolicyToIstioIngressGateway = "networking.gardener.cloud/to-istio-ingressgateway"
)

var (
	// NodeResourceCPU is the resource that will be used for advertising the node's CPU capacity.
	NodeResourceCPU = resource.MustParse("100")
	// NodeResourceMemory is the resource that will be used for advertising the node's memory capacity.
	NodeResourceMemory = resource.MustParse("100Gi")
)
