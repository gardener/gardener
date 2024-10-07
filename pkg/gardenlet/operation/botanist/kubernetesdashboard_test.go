// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
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
	mockkubernetesdashboard "github.com/gardener/gardener/pkg/component/kubernetes/dashboard/mock"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	. "github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
	"github.com/gardener/gardener/pkg/gardenlet/operation/garden"
	shootpkg "github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
)

var _ = Describe("Kubernetes Dashboard", func() {
	var (
		ctrl     *gomock.Controller
		botanist *Botanist
		shoot    *gardencorev1beta1.Shoot
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())

		botanist = &Botanist{Operation: &operation.Operation{
			Garden: &garden.Garden{},
			Shoot:  &shootpkg.Shoot{},
		}}
		shoot = &gardencorev1beta1.Shoot{
			Spec: gardencorev1beta1.ShootSpec{
				Addons: &gardencorev1beta1.Addons{
					KubernetesDashboard: &gardencorev1beta1.KubernetesDashboard{},
				},
				Kubernetes: gardencorev1beta1.Kubernetes{
					Version: "1.31.1",
				},
			},
		}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#DefaultKubernetesDashboard", func() {
		var kubernetesClient *kubernetesmock.MockInterface

		BeforeEach(func() {
			kubernetesClient = kubernetesmock.NewMockInterface(ctrl)
			kubernetesClient.EXPECT().Version().AnyTimes()
			kubernetesClient.EXPECT().Client().AnyTimes()

			botanist.SeedClientSet = kubernetesClient
			botanist.Shoot.SetInfo(shoot)
		})

		It("should successfully create a Kubernetes Dashboard interface", func() {
			kubernetesDashboard, err := botanist.DefaultKubernetesDashboard()
			Expect(kubernetesDashboard).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("#DeployKubernetesDashboard", func() {
		var (
			kubernetesdashboard *mockkubernetesdashboard.MockInterface

			ctx     = context.TODO()
			fakeErr = errors.New("fake err")
		)

		BeforeEach(func() {
			kubernetesdashboard = mockkubernetesdashboard.NewMockInterface(ctrl)
			botanist.Shoot.Components = &shootpkg.Components{
				Addons: &shootpkg.Addons{
					KubernetesDashboard: kubernetesdashboard,
				},
			}
		})

		Context("KubernetesDashboard wanted", func() {
			BeforeEach(func() {
				shoot.Spec.Addons.KubernetesDashboard.Addon = gardencorev1beta1.Addon{
					Enabled: true,
				}
				botanist.Shoot.SetInfo(shoot)
			})

			It("should fail when the deploy function fails", func() {
				kubernetesdashboard.EXPECT().Deploy(ctx).Return(fakeErr)

				Expect(botanist.DeployKubernetesDashboard(ctx)).To(MatchError(fakeErr))
			})

			It("should successfully deploy", func() {
				kubernetesdashboard.EXPECT().Deploy(ctx)

				Expect(botanist.DeployKubernetesDashboard(ctx)).To(Succeed())
			})
		})

		Context("KubernetesDashboard not wanted", func() {
			BeforeEach(func() {
				shoot.Spec.Addons.KubernetesDashboard.Addon = gardencorev1beta1.Addon{
					Enabled: false,
				}
				botanist.Shoot.SetInfo(shoot)
			})

			It("should fail when the destroy function fails", func() {
				kubernetesdashboard.EXPECT().Destroy(ctx).Return(fakeErr)

				Expect(botanist.DeployKubernetesDashboard(ctx)).To(MatchError(fakeErr))
			})

			It("should successfully destroy", func() {
				kubernetesdashboard.EXPECT().Destroy(ctx)

				Expect(botanist.DeployKubernetesDashboard(ctx)).To(Succeed())
			})
		})
	})
})
