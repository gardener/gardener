// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist_test

import (
	"context"
	"errors"

	"github.com/Masterminds/semver/v3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	kubernetesmock "github.com/gardener/gardener/pkg/client/kubernetes/mock"
	mockvpa "github.com/gardener/gardener/pkg/component/autoscaling/vpa/mock"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	. "github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
	seedpkg "github.com/gardener/gardener/pkg/gardenlet/operation/seed"
	shootpkg "github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
)

var _ = Describe("VerticalPodAutoscaler", func() {
	var (
		ctrl     *gomock.Controller
		botanist *Botanist
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		botanist = &Botanist{Operation: &operation.Operation{}}
		botanist.Shoot = &shootpkg.Shoot{
			WantsVerticalPodAutoscaler: true,
		}
		botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{})
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#DefaultVerticalPodAutoscaler", func() {
		var kubernetesClient *kubernetesmock.MockInterface

		BeforeEach(func() {
			kubernetesClient = kubernetesmock.NewMockInterface(ctrl)
			kubernetesClient.EXPECT().Version().AnyTimes()
			kubernetesClient.EXPECT().Client().AnyTimes()

			botanist.SeedClientSet = kubernetesClient
			botanist.Seed = &seedpkg.Seed{}
			botanist.Seed.KubernetesVersion = semver.MustParse("v1.31.1")
			botanist.Shoot = &shootpkg.Shoot{}
			botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{})
		})

		It("should successfully create a VPA component", func() {
			vpa, err := botanist.DefaultVerticalPodAutoscaler()
			Expect(vpa).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("#DeployVerticalPodAutoscaler", func() {
		var (
			vpa *mockvpa.MockInterface

			ctx     = context.TODO()
			fakeErr = errors.New("fake err")
		)

		BeforeEach(func() {
			vpa = mockvpa.NewMockInterface(ctrl)

			botanist.Shoot.Components = &shootpkg.Components{
				ControlPlane: &shootpkg.ControlPlane{
					VerticalPodAutoscaler: vpa,
				},
			}
		})

		Context("VPA wanted", func() {
			It("should fail when the deploy function fails", func() {
				vpa.EXPECT().Deploy(ctx).Return(fakeErr)

				Expect(botanist.DeployVerticalPodAutoscaler(ctx)).To(MatchError(fakeErr))
			})

			It("should successfully deploy", func() {
				vpa.EXPECT().Deploy(ctx)

				Expect(botanist.DeployVerticalPodAutoscaler(ctx)).To(Succeed())
			})
		})

		Context("VPA not wanted", func() {
			BeforeEach(func() {
				botanist.Shoot.WantsVerticalPodAutoscaler = false
			})

			It("should fail when the destroy function fails", func() {
				vpa.EXPECT().Destroy(ctx).Return(fakeErr)

				Expect(botanist.DeployVerticalPodAutoscaler(ctx)).To(MatchError(fakeErr))
			})

			It("should successfully destroy", func() {
				vpa.EXPECT().Destroy(ctx)

				Expect(botanist.DeployVerticalPodAutoscaler(ctx)).To(Succeed())
			})
		})
	})
})
