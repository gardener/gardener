// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist_test

import (
	"context"
	"errors"
	"net"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakediscovery "k8s.io/client-go/discovery/fake"

	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/client/kubernetes/test"
	mockshootsystem "github.com/gardener/gardener/pkg/component/shoot/system/mock"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	. "github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
	shootpkg "github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
)

var _ = Describe("ShootSystem", func() {
	var (
		ctrl        *gomock.Controller
		botanist    *Botanist
		nodeCIDR    = "10.0.0.0/16"
		serviceCIDR = "2001:db8:1::/64"
		podCIDR1    = "2001:db8:2::/64"
		podCIDR2    = "2001:db8:3::/64"
		egressCIDR1 = "100.64.0.1/32"
		egressCIDR2 = "100.64.0.2/32"
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		botanist = &Botanist{Operation: &operation.Operation{}}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#DeployShootSystem", func() {
		var (
			shootSystem *mockshootsystem.MockInterface

			ctx     = context.TODO()
			fakeErr = errors.New("fake err")

			apiResourceList = []*metav1.APIResourceList{
				{
					GroupVersion: "foo/v1",
					APIResources: []metav1.APIResource{
						{Name: "bar", Verbs: metav1.Verbs{"create", "delete"}},
						{Name: "baz", Verbs: metav1.Verbs{"get", "list", "watch"}},
					},
				},
				{
					GroupVersion: "bar/v1beta1",
					APIResources: []metav1.APIResource{
						{Name: "foo", Verbs: metav1.Verbs{"get", "list", "watch"}},
						{Name: "baz", Verbs: metav1.Verbs{"get", "list", "watch"}},
					},
				},
			}
		)

		BeforeEach(func() {
			shootSystem = mockshootsystem.NewMockInterface(ctrl)

			fakeDiscoveryClient := &fakeDiscoveryWithServerPreferredResources{apiResourceList: apiResourceList}
			fakeKubernetes := test.NewClientSetWithDiscovery(nil, fakeDiscoveryClient)
			botanist.ShootClientSet = fakekubernetes.NewClientSetBuilder().WithKubernetes(fakeKubernetes).Build()

			_, nodes, err := net.ParseCIDR(nodeCIDR)
			Expect(err).ToNot(HaveOccurred())
			_, services, err := net.ParseCIDR(serviceCIDR)
			Expect(err).ToNot(HaveOccurred())
			_, pods1, err := net.ParseCIDR(podCIDR1)
			Expect(err).ToNot(HaveOccurred())
			_, pods2, err := net.ParseCIDR(podCIDR2)
			Expect(err).ToNot(HaveOccurred())
			_, egress1, err := net.ParseCIDR(egressCIDR1)
			Expect(err).ToNot(HaveOccurred())
			_, egress2, err := net.ParseCIDR(egressCIDR2)
			Expect(err).ToNot(HaveOccurred())

			botanist.Shoot = &shootpkg.Shoot{
				Components: &shootpkg.Components{
					SystemComponents: &shootpkg.SystemComponents{
						Resources: shootSystem,
					},
				},
				Networks: &shootpkg.Networks{
					Nodes:       []net.IPNet{*nodes},
					Pods:        []net.IPNet{*pods1, *pods2},
					Services:    []net.IPNet{*services},
					EgressCIDRs: []net.IPNet{*egress1, *egress2},
				},
			}

			shootSystem.EXPECT().SetAPIResourceList(apiResourceList)
			shootSystem.EXPECT().SetNodeNetworkCIDRs(botanist.Shoot.Networks.Nodes)
			shootSystem.EXPECT().SetServiceNetworkCIDRs(botanist.Shoot.Networks.Services)
			shootSystem.EXPECT().SetPodNetworkCIDRs(botanist.Shoot.Networks.Pods)
			shootSystem.EXPECT().SetEgressCIDRs(botanist.Shoot.Networks.EgressCIDRs)
		})

		It("should discover the API and deploy", func() {
			shootSystem.EXPECT().Deploy(ctx)
			Expect(botanist.DeployShootSystem(ctx)).To(Succeed())
		})

		It("should fail when the deploy function fails", func() {
			shootSystem.EXPECT().Deploy(ctx).Return(fakeErr)
			Expect(botanist.DeployShootSystem(ctx)).To(Equal(fakeErr))
		})
	})
})

type fakeDiscoveryWithServerPreferredResources struct {
	*fakediscovery.FakeDiscovery

	apiResourceList []*metav1.APIResourceList
}

func (f *fakeDiscoveryWithServerPreferredResources) ServerPreferredResources() ([]*metav1.APIResourceList, error) {
	return f.apiResourceList, nil
}
