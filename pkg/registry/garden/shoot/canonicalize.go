// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package shoot

import (
	"net"

	gardencore "github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/apis/garden"
)

// canonicalizeShootCIDRs tries to convert all CIDR address to their canonical form.
// e.g "10.0.4.5/24" -> "10.0.4.0/24"
// Parse errors are ignored and they should be picked by the valdiation.
func canonicalizeShootCIDRs(spec *garden.ShootSpec) {
	if spec == nil {
		return
	}

	if networking := spec.Networking; networking != nil {
		canonicalizeK8SNetworks(networking.K8SNetworks)
	}

	if aws := spec.Cloud.AWS; aws != nil {
		networks := aws.Networks

		canonicalizeK8SNetworks(networks.K8SNetworks)

		canonicalizeCIDRs(networks.Internal)
		canonicalizeCIDR(networks.VPC.CIDR)
		canonicalizeCIDRs(networks.Public)
		canonicalizeCIDRs(networks.Workers)
	}

	if azure := spec.Cloud.Azure; azure != nil {
		networks := azure.Networks

		canonicalizeK8SNetworks(networks.K8SNetworks)

		canonicalizeCIDR(networks.VNet.CIDR)

		workers := &networks.Workers
		canonicalizeCIDR(&networks.Workers)
		spec.Cloud.Azure.Networks.Workers = *workers
	}

	if gcp := spec.Cloud.GCP; gcp != nil {
		networks := gcp.Networks

		canonicalizeK8SNetworks(networks.K8SNetworks)

		canonicalizeCIDR(networks.Internal)
		canonicalizeCIDRs(networks.Workers)
	}

	if openstack := spec.Cloud.OpenStack; openstack != nil {
		networks := openstack.Networks

		canonicalizeK8SNetworks(networks.K8SNetworks)

		canonicalizeCIDRs(networks.Workers)
	}

	if alicloud := spec.Cloud.Alicloud; alicloud != nil {
		networks := alicloud.Networks

		canonicalizeK8SNetworks(networks.K8SNetworks)

		canonicalizeCIDR(networks.VPC.CIDR)
		canonicalizeCIDRs(networks.Workers)
	}

	if packet := spec.Cloud.Packet; packet != nil {
		canonicalizeK8SNetworks(packet.Networks.K8SNetworks)
	}
}

func canonicalizeCIDR(cidr *gardencore.CIDR) {
	if cidr == nil {
		return
	}
	_, ipNet, _ := net.ParseCIDR(string(*cidr))
	if ipNet != nil {
		*cidr = gardencore.CIDR(ipNet.String())
	}
}

func canonicalizeCIDRs(cidrs []gardencore.CIDR) {
	for i, cidr := range cidrs {
		cidr := &cidr
		canonicalizeCIDR(cidr)
		cidrs[i] = *cidr
	}
}

func canonicalizeK8SNetworks(networks gardencore.K8SNetworks) {
	canonicalizeCIDR(networks.Nodes)
	canonicalizeCIDR(networks.Pods)
	canonicalizeCIDR(networks.Services)
}
