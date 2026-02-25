// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"

	"github.com/gardener/gardener/pkg/component/mock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	kubernetesmock "github.com/gardener/gardener/pkg/client/kubernetes/mock"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	seedpkg "github.com/gardener/gardener/pkg/gardenlet/operation/seed"
	shootpkg "github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
)

var _ = Describe("ExtAuthzServer", func() {
	var (
		ctrl             *gomock.Controller
		botanist         *Botanist
		kubernetesClient *kubernetesmock.MockInterface

		ctx = context.TODO()
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		kubernetesClient = kubernetesmock.NewMockInterface(ctrl)

		botanist = &Botanist{Operation: &operation.Operation{
			Shoot: &shootpkg.Shoot{
				Components: &shootpkg.Components{
					ControlPlane: &shootpkg.ControlPlane{},
				},
			},
			Seed:          &seedpkg.Seed{},
			SeedClientSet: kubernetesClient,
		}}
		botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{})
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#DefaultExtAuthzServer", func() {
		It("should successfully create a ext-authz-server interface", func() {
			kubernetesClient.EXPECT().Client()

			extAuthzServer, err := botanist.DefaultExtAuthzServer()
			Expect(extAuthzServer).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("#DeployExtAuthzServer", func() {
		var mockExtAuthzServer *mock.MockDeployWaiter

		BeforeEach(func() {
			mockExtAuthzServer = mock.NewMockDeployWaiter(ctrl)
			botanist.Shoot.Components.ControlPlane.ExtAuthzServer = mockExtAuthzServer
		})

		It("should successfully deploy the ext-authz-server", func() {
			mockExtAuthzServer.EXPECT().Deploy(ctx)
			Expect(botanist.DeployExtAuthzServer(ctx)).To(Succeed())
		})

		It("should successfully destroy the ext-authz-server when shoot purpose is testing", func() {
			botanist.Shoot.Purpose = gardencorev1beta1.ShootPurposeTesting
			mockExtAuthzServer.EXPECT().Destroy(ctx)
			Expect(botanist.DeployExtAuthzServer(ctx)).To(Succeed())
		})
	})
})
