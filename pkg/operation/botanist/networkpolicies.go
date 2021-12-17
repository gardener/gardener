// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package botanist

import (
	"fmt"
	"net"

	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/operation/botanist/component/networkpolicies"
	"github.com/gardener/gardener/pkg/operation/botanist/component/nodelocaldns"
	"github.com/gardener/gardener/pkg/operation/common"

	"k8s.io/utils/pointer"
)

// NewNetworkPoliciesDeployer is an alias for networkpolicies.New. Exposed for testing.
var NewNetworkPoliciesDeployer = networkpolicies.New

// DefaultNetworkPolicies returns a deployer for the network policies that deny all traffic and allow certain components
// to use annotations to declare their desire to transmit/receive traffic to/from other Pods/IP addresses.
func (b *Botanist) DefaultNetworkPolicies(sniPhase component.Phase) (component.Deployer, error) {
	var shootCIDRNetworks []string
	if v := b.Shoot.GetInfo().Spec.Networking.Nodes; v != nil {
		shootCIDRNetworks = append(shootCIDRNetworks, *v)
	}
	if v := b.Shoot.GetInfo().Spec.Networking.Pods; v != nil {
		shootCIDRNetworks = append(shootCIDRNetworks, *v)
	}
	if v := b.Shoot.GetInfo().Spec.Networking.Services; v != nil {
		shootCIDRNetworks = append(shootCIDRNetworks, *v)
	}

	shootNetworkPeers, err := networkpolicies.NetworkPolicyPeersWithExceptions(shootCIDRNetworks, b.Seed.GetInfo().Spec.Networks.BlockCIDRs...)
	if err != nil {
		return nil, err
	}

	seedCIDRNetworks := []string{b.Seed.GetInfo().Spec.Networks.Pods, b.Seed.GetInfo().Spec.Networks.Services}
	if v := b.Seed.GetInfo().Spec.Networks.Nodes; v != nil {
		seedCIDRNetworks = append(seedCIDRNetworks, *v)
	}

	allCIDRNetworks := append(seedCIDRNetworks, shootCIDRNetworks...)
	allCIDRNetworks = append(allCIDRNetworks, b.Seed.GetInfo().Spec.Networks.BlockCIDRs...)

	privateNetworkPeers, err := networkpolicies.ToNetworkPolicyPeersWithExceptions(networkpolicies.AllPrivateNetworkBlocks(), allCIDRNetworks...)
	if err != nil {
		return nil, err
	}

	_, seedServiceCIDR, err := net.ParseCIDR(b.Seed.GetInfo().Spec.Networks.Services)
	if err != nil {
		return nil, err
	}
	seedDNSServerAddress, err := common.ComputeOffsetIP(seedServiceCIDR, 10)
	if err != nil {
		return nil, fmt.Errorf("cannot calculate CoreDNS ClusterIP: %w", err)
	}

	return NewNetworkPoliciesDeployer(
		b.K8sSeedClient.Client(),
		b.Shoot.SeedNamespace,
		networkpolicies.Values{
			ShootNetworkPeers: shootNetworkPeers,
			GlobalValues: networkpolicies.GlobalValues{
				// Enable network policies for SNI
				// When disabling SNI (previously enabled), the control plane is transitioning between states, thus
				// it needs to be ensured that the traffic from old clients can still reach the API server.
				SNIEnabled:           sniPhase == component.PhaseEnabled || sniPhase == component.PhaseEnabling || sniPhase == component.PhaseDisabling,
				BlockedAddresses:     b.Seed.GetInfo().Spec.Networks.BlockCIDRs,
				PrivateNetworkPeers:  privateNetworkPeers,
				DenyAllTraffic:       true,
				NodeLocalIPVSAddress: pointer.String(nodelocaldns.IPVSAddress),
				DNSServerAddress:     pointer.String(seedDNSServerAddress.String()),
			},
		},
	), nil
}
