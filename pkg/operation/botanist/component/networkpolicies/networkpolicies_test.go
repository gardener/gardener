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

package networkpolicies_test

import (
	"context"
	"fmt"

	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	. "github.com/gardener/gardener/pkg/operation/botanist/component/networkpolicies"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Networkpolicies", func() {
	var (
		ctx       = context.TODO()
		fakeErr   = fmt.Errorf("fake error")
		namespace = "shoot--foo--bar"

		ctrl     *gomock.Controller
		c        *mockclient.MockClient
		deployer component.Deployer
		values   Values
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		c = mockclient.NewMockClient(ctrl)
		deployer = New(c, namespace, values)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#Deploy", func() {
		It("should fail if any call fails", func() {
			allowToAggregatePrometheus := constructNPAllowToAggregatePrometheus(namespace)

			c.EXPECT().Get(ctx, kutil.Key(allowToAggregatePrometheus.Namespace, allowToAggregatePrometheus.Name), gomock.AssignableToTypeOf(&networkingv1.NetworkPolicy{})).Return(fakeErr)

			Expect(deployer.Deploy(ctx)).To(MatchError(fakeErr))
		})

		It("w/o any special configuration", func() {
			expectGetUpdate(ctx, c, constructNPAllowToAggregatePrometheus(namespace))
			expectGetUpdate(ctx, c, constructNPAllowToSeedPrometheus(namespace))
			expectGetUpdate(ctx, c, constructNPAllowToAllShootAPIServers(namespace, values.SNIEnabled))
			expectGetUpdate(ctx, c, constructNPAllowToBlockedCIDRs(namespace, values.BlockedAddresses))
			expectGetUpdate(ctx, c, constructNPAllowToDNS(namespace, values.DNSServerAddress, values.NodeLocalIPVSAddress))
			expectGetUpdate(ctx, c, constructNPDenyAll(namespace, values.DenyAllTraffic))
			expectGetUpdate(ctx, c, constructNPAllowToPrivateNetworks(namespace, values.PrivateNetworkPeers))
			expectGetUpdate(ctx, c, constructNPAllowToPublicNetworks(namespace, values.BlockedAddresses))
			expectGetUpdate(ctx, c, constructNPAllowToShootNetworks(namespace, values.ShootNetworkPeers))

			Expect(deployer.Deploy(ctx)).To(Succeed())
		})

		It("w/ SNI enabled, w/ blocked addresses, w/ deny all, w/ private network peers, w/ shoot network peers", func() {
			values = Values{
				ShootNetworkPeers: []networkingv1.NetworkPolicyPeer{
					{IPBlock: &networkingv1.IPBlock{CIDR: "1.2.3.4/5"}},
					{PodSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"shoot": "peers"}}},
				},
				GlobalValues: GlobalValues{
					SNIEnabled:       true,
					BlockedAddresses: []string{"foo", "bar"},
					PrivateNetworkPeers: []networkingv1.NetworkPolicyPeer{
						{PodSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"private": "peers"}}},
						{IPBlock: &networkingv1.IPBlock{CIDR: "6.7.8.9/10"}},
					},
					DenyAllTraffic:       true,
					NodeLocalIPVSAddress: pointer.String("node-local-ipvs-address"),
					DNSServerAddress:     pointer.String("dns-server-address"),
				},
			}
			deployer = New(c, namespace, values)

			expectGetUpdate(ctx, c, constructNPAllowToAggregatePrometheus(namespace))
			expectGetUpdate(ctx, c, constructNPAllowToSeedPrometheus(namespace))
			expectGetUpdate(ctx, c, constructNPAllowToAllShootAPIServers(namespace, values.SNIEnabled))
			expectGetUpdate(ctx, c, constructNPAllowToBlockedCIDRs(namespace, values.BlockedAddresses))
			expectGetUpdate(ctx, c, constructNPAllowToDNS(namespace, values.DNSServerAddress, values.NodeLocalIPVSAddress))
			expectGetUpdate(ctx, c, constructNPDenyAll(namespace, values.DenyAllTraffic))
			expectGetUpdate(ctx, c, constructNPAllowToPrivateNetworks(namespace, values.PrivateNetworkPeers))
			expectGetUpdate(ctx, c, constructNPAllowToPublicNetworks(namespace, values.BlockedAddresses))
			expectGetUpdate(ctx, c, constructNPAllowToShootNetworks(namespace, values.ShootNetworkPeers))

			Expect(deployer.Deploy(ctx)).To(Succeed())
		})
	})

	Describe("#Destroy", func() {
		It("should fail if an object fails to delete", func() {
			gomock.InOrder(
				c.EXPECT().Delete(ctx, &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "allow-to-aggregate-prometheus", Namespace: namespace}}),
				c.EXPECT().Delete(ctx, &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "allow-to-seed-prometheus", Namespace: namespace}}),
				c.EXPECT().Delete(ctx, &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "allow-to-all-shoot-apiservers", Namespace: namespace}}),
				c.EXPECT().Delete(ctx, &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "allow-to-blocked-cidrs", Namespace: namespace}}).Return(fakeErr),
			)

			Expect(deployer.Destroy(ctx)).To(MatchError(fakeErr))
		})

		It("should successfully destroy all objects", func() {
			gomock.InOrder(
				c.EXPECT().Delete(ctx, &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "allow-to-aggregate-prometheus", Namespace: namespace}}),
				c.EXPECT().Delete(ctx, &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "allow-to-seed-prometheus", Namespace: namespace}}),
				c.EXPECT().Delete(ctx, &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "allow-to-all-shoot-apiservers", Namespace: namespace}}),
				c.EXPECT().Delete(ctx, &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "allow-to-blocked-cidrs", Namespace: namespace}}),
				c.EXPECT().Delete(ctx, &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "allow-to-dns", Namespace: namespace}}),
				c.EXPECT().Delete(ctx, &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "deny-all", Namespace: namespace}}),
				c.EXPECT().Delete(ctx, &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "allow-to-private-networks", Namespace: namespace}}),
				c.EXPECT().Delete(ctx, &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "allow-to-public-networks", Namespace: namespace}}),
				c.EXPECT().Delete(ctx, &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "allow-to-shoot-networks", Namespace: namespace}}),
			)

			Expect(deployer.Destroy(ctx)).To(Succeed())
		})
	})
})

func constructNPAllowToShootNetworks(namespace string, peers []networkingv1.NetworkPolicyPeer) *networkingv1.NetworkPolicy {
	obj := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "allow-to-shoot-networks",
			Namespace:   namespace,
			Annotations: map[string]string{"gardener.cloud/description": "Allows Egress from pods labeled with 'networking.gardener.cloud/to-shoot-networks=allowed' to IPv4 blocks belonging to the Shoot network. In practice, this should be used by components which use 'vpn-seed' to communicate to Pods in the Shoot cluster."},
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{"networking.gardener.cloud/to-shoot-networks": "allowed"},
			},
			Egress:      []networkingv1.NetworkPolicyEgressRule{{}},
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
		},
	}

	if peers != nil {
		obj.Spec.Egress[0].To = peers
	}

	return obj
}

func expectGetUpdate(ctx context.Context, c *mockclient.MockClient, expected *networkingv1.NetworkPolicy) {
	c.EXPECT().Get(ctx, kutil.Key(expected.Namespace, expected.Name), gomock.AssignableToTypeOf(&networkingv1.NetworkPolicy{}))
	c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&networkingv1.NetworkPolicy{}), gomock.Any()).
		Do(func(ctx context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
			Expect(obj).To(DeepEqual(expected))
		})
}
