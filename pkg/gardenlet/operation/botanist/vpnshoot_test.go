// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist_test

import (
	"net"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	kubernetesmock "github.com/gardener/gardener/pkg/client/kubernetes/mock"
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
				},
			}
			botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					Kubernetes: gardencorev1beta1.Kubernetes{
						Version: "1.30.1",
					},
					Networking: &gardencorev1beta1.Networking{
						IPFamilies: []gardencorev1beta1.IPFamily{gardencorev1beta1.IPFamilyIPv4},
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
})
