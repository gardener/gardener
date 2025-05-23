// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist_test

import (
	"context"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	kubernetesmock "github.com/gardener/gardener/pkg/client/kubernetes/mock"
	mockcomponent "github.com/gardener/gardener/pkg/component/mock"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	. "github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
	shootpkg "github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
)

var _ = Describe("KubeStateMetrics", func() {
	var (
		ctrl     *gomock.Controller
		botanist *Botanist
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		botanist = &Botanist{Operation: &operation.Operation{}}
		botanist.Shoot = &shootpkg.Shoot{
			Purpose: gardencorev1beta1.ShootPurposeProduction,
		}
		botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{})
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#DefaultKubeStateMetrics", func() {
		var kubernetesClient *kubernetesmock.MockInterface

		BeforeEach(func() {
			kubernetesClient = kubernetesmock.NewMockInterface(ctrl)
			kubernetesClient.EXPECT().Version().AnyTimes()
			kubernetesClient.EXPECT().Client().AnyTimes()

			botanist.SeedClientSet = kubernetesClient
			botanist.Shoot = &shootpkg.Shoot{}
			botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{})
		})

		It("should successfully create a kube-state-metrics component", func() {
			ksm, err := botanist.DefaultKubeStateMetrics()
			Expect(ksm).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("#DeployKubeStateMetrics", func() {
		var (
			kubeStateMetrics *mockcomponent.MockDeployWaiter

			ctx     = context.TODO()
			fakeErr = errors.New("fake err")
		)

		BeforeEach(func() {
			kubeStateMetrics = mockcomponent.NewMockDeployWaiter(ctrl)

			botanist.Shoot.Components = &shootpkg.Components{
				ControlPlane: &shootpkg.ControlPlane{
					KubeStateMetrics: kubeStateMetrics,
				},
			}
		})

		Context("shoot purpose != testing", func() {
			It("should fail when the deploy function fails", func() {
				kubeStateMetrics.EXPECT().Deploy(ctx).Return(fakeErr)

				Expect(botanist.DeployKubeStateMetrics(ctx)).To(MatchError(fakeErr))
			})

			It("should successfully deploy", func() {
				kubeStateMetrics.EXPECT().Deploy(ctx)

				Expect(botanist.DeployKubeStateMetrics(ctx)).To(Succeed())
			})
		})

		Context("shoot purpose = testing", func() {
			BeforeEach(func() {
				botanist.Shoot.Purpose = gardencorev1beta1.ShootPurposeTesting
			})

			It("should fail when the destroy function fails", func() {
				kubeStateMetrics.EXPECT().Destroy(ctx).Return(fakeErr)

				Expect(botanist.DeployKubeStateMetrics(ctx)).To(MatchError(fakeErr))
			})

			It("should successfully destroy", func() {
				kubeStateMetrics.EXPECT().Destroy(ctx)

				Expect(botanist.DeployKubeStateMetrics(ctx)).To(Succeed())
			})
		})
	})
})
