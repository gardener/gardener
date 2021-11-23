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
	"net"

	"github.com/gardener/gardener/charts"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	mockkubernetes "github.com/gardener/gardener/pkg/client/kubernetes/mock"
	"github.com/gardener/gardener/pkg/operation"
	. "github.com/gardener/gardener/pkg/operation/botanist"
	shootpkg "github.com/gardener/gardener/pkg/operation/shoot"
	"github.com/gardener/gardener/pkg/utils/imagevector"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
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
		var kubernetesClient *mockkubernetes.MockInterface

		BeforeEach(func() {
			kubernetesClient = mockkubernetes.NewMockInterface(ctrl)
			botanist.K8sSeedClient = kubernetesClient
			botanist.Shoot = &shootpkg.Shoot{
				Networks: &shootpkg.Networks{
					Pods:     &net.IPNet{IP: []byte("192.168.0.0"), Mask: []byte("16")},
					Services: &net.IPNet{IP: []byte("10.0.0.0"), Mask: []byte("24")},
				},
			}
			botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{})
		})

		It("should successfully create a vpnShoot interface for ReversedVPN not enabled case", func() {
			kubernetesClient.EXPECT().Client()
			botanist.ImageVector = imagevector.ImageVector{{Name: charts.ImageNameVpnShoot}}
			botanist.Shoot.ReversedVPNEnabled = false

			vpnShoot, err := botanist.DefaultVPNShoot()
			Expect(vpnShoot).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())
		})

		It("should successfully create a vpnShoot interface for ReversedVPN enabled case", func() {
			kubernetesClient.EXPECT().Client()
			botanist.ImageVector = imagevector.ImageVector{{Name: charts.ImageNameVpnShootClient}}
			botanist.Shoot.ReversedVPNEnabled = true

			vpnShoot, err := botanist.DefaultVPNShoot()
			Expect(vpnShoot).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return an error because the image cannot be found", func() {
			botanist.ImageVector = imagevector.ImageVector{}

			vpnShoot, err := botanist.DefaultVPNShoot()
			Expect(vpnShoot).To(BeNil())
			Expect(err).To(HaveOccurred())
		})
	})
})
