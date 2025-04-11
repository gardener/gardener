// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
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
	"k8s.io/utils/ptr"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	kubernetesmock "github.com/gardener/gardener/pkg/client/kubernetes/mock"
	mockvpnshoot "github.com/gardener/gardener/pkg/component/networking/vpn/shoot/mock"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	. "github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
	seedpkg "github.com/gardener/gardener/pkg/gardenlet/operation/seed"
	shootpkg "github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
)

var _ = Describe("VPNShoot", func() {
	var (
		ctrl     *gomock.Controller
		botanist *Botanist
	)
	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		botanist = &Botanist{Operation: &operation.Operation{}}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#DefaultVPNShoot", func() {
		var kubernetesClient *kubernetesmock.MockInterface

		BeforeEach(func() {
			kubernetesClient = kubernetesmock.NewMockInterface(ctrl)
			botanist.SeedClientSet = kubernetesClient
			botanist.Shoot = &shootpkg.Shoot{
				Networks: &shootpkg.Networks{
					Pods:     []net.IPNet{{IP: []byte("192.168.0.0"), Mask: []byte("16")}},
					Services: []net.IPNet{{IP: []byte("10.0.0.0"), Mask: []byte("24")}},
					Nodes:    []net.IPNet{{IP: []byte("10.181.0.0"), Mask: []byte("16")}},
				},
			}
			botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					Kubernetes: gardencorev1beta1.Kubernetes{
						Version: "1.30.1",
					},
					Networking: &gardencorev1beta1.Networking{
						IPFamilies: []gardencorev1beta1.IPFamily{gardencorev1beta1.IPFamilyIPv4},
						Pods:       ptr.To("192.168.0.0/16"),
						Services:   ptr.To("10.0.0.0/24"),
						Nodes:      ptr.To("10.181.0.0/16"),
					},
				},
			})
			botanist.Seed = &seedpkg.Seed{}
			botanist.Seed.SetInfo(&gardencorev1beta1.Seed{})
		})

		It("should successfully create a vpnShoot interface for ReversedVPN", func() {
			kubernetesClient.EXPECT().Client()

			vpnShoot, err := botanist.DefaultVPNShoot()
			Expect(vpnShoot).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("#DeployVPNShoot", func() {
		var (
			vpnShoot *mockvpnshoot.MockInterface
			ctx      = context.TODO()
			fakeErr  = errors.New("fake err")
		)

		BeforeEach(func() {
			vpnShoot = mockvpnshoot.NewMockInterface(ctrl)

			botanist.Shoot = &shootpkg.Shoot{
				Components: &shootpkg.Components{
					SystemComponents: &shootpkg.SystemComponents{
						VPNShoot: vpnShoot,
					},
				},
				Networks: &shootpkg.Networks{
					Pods:     []net.IPNet{{IP: []byte("192.168.0.0"), Mask: []byte("16")}},
					Services: []net.IPNet{{IP: []byte("10.0.0.0"), Mask: []byte("24")}},
					Nodes:    []net.IPNet{{IP: []byte("10.181.0.0"), Mask: []byte("16")}},
				},
			}
		})

		BeforeEach(func() {
			vpnShoot.EXPECT().SetNodeNetworkCIDRs(botanist.Shoot.Networks.Nodes)
			vpnShoot.EXPECT().SetServiceNetworkCIDRs(botanist.Shoot.Networks.Services)
			vpnShoot.EXPECT().SetPodNetworkCIDRs(botanist.Shoot.Networks.Pods)
		})

		It("should set the network ranges and deploy", func() {
			vpnShoot.EXPECT().Deploy(ctx)
			Expect(botanist.DeployVPNShoot(ctx)).To(Succeed())
		})

		It("should fail when the deploy function fails", func() {
			vpnShoot.EXPECT().Deploy(ctx).Return(fakeErr)
			Expect(botanist.DeployVPNShoot(ctx)).To(Equal(fakeErr))
		})
	})
})
