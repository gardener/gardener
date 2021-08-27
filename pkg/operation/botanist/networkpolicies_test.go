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

package botanist_test

import (
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	mockkubernetes "github.com/gardener/gardener/pkg/client/kubernetes/mock"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	"github.com/gardener/gardener/pkg/operation"
	. "github.com/gardener/gardener/pkg/operation/botanist"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/operation/botanist/component/networkpolicies"
	seedpkg "github.com/gardener/gardener/pkg/operation/seed"
	shootpkg "github.com/gardener/gardener/pkg/operation/shoot"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	networkingv1 "k8s.io/api/networking/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Networkpolicies", func() {
	var (
		ctrl            *gomock.Controller
		clientInterface *mockkubernetes.MockInterface
		c               *mockclient.MockClient
		botanist        *Botanist

		seedNamespace = "shoot--foo--bar"

		podCIDRShoot     = "100.96.0.0/13"
		serviceCIDRShoot = "172.18.0.0/14"
		nodeCIDRShoot    = "10.250.0.0/16"

		podCIDRSeed     = "10.222.0.0/16"
		serviceCIDRSeed = "192.168.0.1/24"
		nodeCIDRSeed    = "10.224.0.0/16"
		blockCIDRs      = []string{"10.250.10.250/32"}

		defaultExpectedShootNetworkPeers = []interface{}{
			networkingv1.NetworkPolicyPeer{IPBlock: &networkingv1.IPBlock{
				CIDR:   "10.0.0.0/8",
				Except: []string{podCIDRSeed},
			}},
			networkingv1.NetworkPolicyPeer{IPBlock: &networkingv1.IPBlock{CIDR: "172.16.0.0/12"}},
			networkingv1.NetworkPolicyPeer{IPBlock: &networkingv1.IPBlock{CIDR: "192.168.0.0/16"}},
			networkingv1.NetworkPolicyPeer{IPBlock: &networkingv1.IPBlock{CIDR: "100.64.0.0/10"}},
		}
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		clientInterface = mockkubernetes.NewMockInterface(ctrl)
		c = mockclient.NewMockClient(ctrl)
		botanist = &Botanist{
			Operation: &operation.Operation{
				K8sSeedClient: clientInterface,
				Seed:          &seedpkg.Seed{},
				Shoot: &shootpkg.Shoot{
					Networks: &shootpkg.Networks{
						CoreDNS: []byte{20, 0, 0, 10},
					},
					SeedNamespace: seedNamespace,
				},
			},
		}
		botanist.Seed.SetInfo(&gardencorev1beta1.Seed{
			Spec: gardencorev1beta1.SeedSpec{
				Networks: gardencorev1beta1.SeedNetworks{
					Pods:     podCIDRSeed,
					Services: serviceCIDRSeed,
				},
			},
		})
		botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{})
	})

	DescribeTable("#DefaultNetworkPolicies",
		func(sniPhase component.Phase, prepTestValues func(), expectations func(client.Client, string, networkpolicies.Values)) {
			prepTestValues()
			validator := &newNetworkPoliciesFuncValidator{expectations: expectations}

			oldNewNetworkPoliciesDeployerFn := NewNetworkPoliciesDeployer
			defer func() { NewNetworkPoliciesDeployer = oldNewNetworkPoliciesDeployerFn }()
			NewNetworkPoliciesDeployer = validator.new

			clientInterface.EXPECT().Client().Return(c)

			_, err := botanist.DefaultNetworkPolicies(sniPhase)
			Expect(err).NotTo(HaveOccurred())
		},

		Entry(
			"w/o networks",
			component.PhaseUnknown,
			func() {},
			func(client client.Client, namespace string, values networkpolicies.Values) {
				Expect(client).To(Equal(c))
				Expect(namespace).To(Equal(seedNamespace))
				Expect(values.SNIEnabled).To(BeFalse())
				Expect(values.BlockedAddresses).To(BeEmpty())
				Expect(values.DenyAllTraffic).To(BeTrue())
				Expect(values.ShootNetworkPeers).To(BeEmpty())
				Expect(values.PrivateNetworkPeers).To(ConsistOf(defaultExpectedShootNetworkPeers...))
				Expect(values.NodeLocalIPVSAddress).To(PointTo(Equal("169.254.20.10")))
				Expect(values.DNSServerAddress).To(PointTo(Equal("192.168.0.10")))
			},
		),

		Entry(
			"w/ network CIDRs",
			component.PhaseUnknown,
			func() {
				botanist.Shoot.GetInfo().Spec.Networking.Pods = &podCIDRShoot
				botanist.Shoot.GetInfo().Spec.Networking.Services = &serviceCIDRShoot
				botanist.Shoot.GetInfo().Spec.Networking.Nodes = &nodeCIDRShoot
				botanist.Seed.GetInfo().Spec.Networks.Nodes = &nodeCIDRSeed
				botanist.Seed.GetInfo().Spec.Networks.BlockCIDRs = blockCIDRs
			},
			func(client client.Client, namespace string, values networkpolicies.Values) {
				Expect(client).To(Equal(c))
				Expect(namespace).To(Equal(seedNamespace))
				Expect(values.SNIEnabled).To(BeFalse())
				Expect(values.BlockedAddresses).To(Equal(blockCIDRs))
				Expect(values.DenyAllTraffic).To(BeTrue())
				Expect(values.ShootNetworkPeers).To(ConsistOf(
					networkingv1.NetworkPolicyPeer{IPBlock: &networkingv1.IPBlock{CIDR: nodeCIDRShoot, Except: blockCIDRs}},
					networkingv1.NetworkPolicyPeer{IPBlock: &networkingv1.IPBlock{CIDR: podCIDRShoot}},
					networkingv1.NetworkPolicyPeer{IPBlock: &networkingv1.IPBlock{CIDR: "172.16.0.0/14"}},
				))
				Expect(values.PrivateNetworkPeers).To(ConsistOf(
					networkingv1.NetworkPolicyPeer{IPBlock: &networkingv1.IPBlock{
						CIDR:   "10.0.0.0/8",
						Except: append([]string{podCIDRSeed, nodeCIDRSeed, nodeCIDRShoot}, blockCIDRs...),
					}},
					networkingv1.NetworkPolicyPeer{IPBlock: &networkingv1.IPBlock{CIDR: "172.16.0.0/12"}},
					networkingv1.NetworkPolicyPeer{IPBlock: &networkingv1.IPBlock{CIDR: "192.168.0.0/16"}},
					networkingv1.NetworkPolicyPeer{IPBlock: &networkingv1.IPBlock{
						CIDR:   "100.64.0.0/10",
						Except: []string{podCIDRShoot},
					}},
				))
				Expect(values.NodeLocalIPVSAddress).To(PointTo(Equal("169.254.20.10")))
				Expect(values.DNSServerAddress).To(PointTo(Equal("192.168.0.10")))
			},
		),

		Entry(
			"SNI phase enabled",
			component.PhaseEnabled,
			func() {},
			func(client client.Client, namespace string, values networkpolicies.Values) {
				Expect(client).To(Equal(c))
				Expect(namespace).To(Equal(seedNamespace))
				Expect(values.SNIEnabled).To(BeTrue())
				Expect(values.BlockedAddresses).To(BeEmpty())
				Expect(values.DenyAllTraffic).To(BeTrue())
				Expect(values.ShootNetworkPeers).To(BeEmpty())
				Expect(values.PrivateNetworkPeers).To(ConsistOf(defaultExpectedShootNetworkPeers...))
				Expect(values.NodeLocalIPVSAddress).To(PointTo(Equal("169.254.20.10")))
				Expect(values.DNSServerAddress).To(PointTo(Equal("192.168.0.10")))
			},
		),

		Entry(
			"SNI phase enabling",
			component.PhaseEnabling,
			func() {},
			func(client client.Client, namespace string, values networkpolicies.Values) {
				Expect(client).To(Equal(c))
				Expect(namespace).To(Equal(seedNamespace))
				Expect(values.SNIEnabled).To(BeTrue())
				Expect(values.BlockedAddresses).To(BeEmpty())
				Expect(values.DenyAllTraffic).To(BeTrue())
				Expect(values.ShootNetworkPeers).To(BeEmpty())
				Expect(values.PrivateNetworkPeers).To(ConsistOf(defaultExpectedShootNetworkPeers...))
				Expect(values.NodeLocalIPVSAddress).To(PointTo(Equal("169.254.20.10")))
				Expect(values.DNSServerAddress).To(PointTo(Equal("192.168.0.10")))
			},
		),

		Entry(
			"SNI phase disabling",
			component.PhaseDisabling,
			func() {},
			func(client client.Client, namespace string, values networkpolicies.Values) {
				Expect(client).To(Equal(c))
				Expect(namespace).To(Equal(seedNamespace))
				Expect(values.SNIEnabled).To(BeTrue())
				Expect(values.BlockedAddresses).To(BeEmpty())
				Expect(values.DenyAllTraffic).To(BeTrue())
				Expect(values.ShootNetworkPeers).To(BeEmpty())
				Expect(values.PrivateNetworkPeers).To(ConsistOf(defaultExpectedShootNetworkPeers...))
				Expect(values.NodeLocalIPVSAddress).To(PointTo(Equal("169.254.20.10")))
				Expect(values.DNSServerAddress).To(PointTo(Equal("192.168.0.10")))
			},
		),

		Entry(
			"SNI phase disabled",
			component.PhaseDisabled,
			func() {},
			func(client client.Client, namespace string, values networkpolicies.Values) {
				Expect(client).To(Equal(c))
				Expect(namespace).To(Equal(seedNamespace))
				Expect(values.SNIEnabled).To(BeFalse())
				Expect(values.BlockedAddresses).To(BeEmpty())
				Expect(values.DenyAllTraffic).To(BeTrue())
				Expect(values.ShootNetworkPeers).To(BeEmpty())
				Expect(values.PrivateNetworkPeers).To(ConsistOf(defaultExpectedShootNetworkPeers...))
				Expect(values.NodeLocalIPVSAddress).To(PointTo(Equal("169.254.20.10")))
				Expect(values.DNSServerAddress).To(PointTo(Equal("192.168.0.10")))
			},
		),
	)
})

type newNetworkPoliciesFuncValidator struct {
	expectations func(client.Client, string, networkpolicies.Values)
}

func (n *newNetworkPoliciesFuncValidator) new(client client.Client, namespace string, values networkpolicies.Values) component.Deployer {
	n.expectations(client, namespace, values)
	return nil
}
