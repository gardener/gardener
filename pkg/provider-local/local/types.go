// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
