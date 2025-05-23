// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	"k8s.io/utils/ptr"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	kubernetesmock "github.com/gardener/gardener/pkg/client/kubernetes/mock"
	mockcomponent "github.com/gardener/gardener/pkg/component/mock"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	shootpkg "github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
)

var _ = Describe("BlackboxExporter", func() {
	var (
		ctrl     *gomock.Controller
		botanist *Botanist

		shootPurposeEvaluation = gardencorev1beta1.ShootPurposeEvaluation
		shootPurposeTesting    = gardencorev1beta1.ShootPurposeTesting
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		botanist = &Botanist{Operation: &operation.Operation{}}
		botanist.Shoot = &shootpkg.Shoot{}
		botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
			Spec: gardencorev1beta1.ShootSpec{
				Kubernetes: gardencorev1beta1.Kubernetes{
					Version: "1.31.1",
				},
				Purpose: &shootPurposeEvaluation,
			},
		})
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#DefaultBlackboxExporterCluster", func() {
		var kubernetesClient *kubernetesmock.MockInterface

		BeforeEach(func() {
			kubernetesClient = kubernetesmock.NewMockInterface(ctrl)

			botanist.SeedClientSet = kubernetesClient
		})

		It("should successfully create a blackbox-exporter interface", func() {
			kubernetesClient.EXPECT().Client()

			blackboxExporter, err := botanist.DefaultBlackboxExporterCluster()
			Expect(blackboxExporter).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("#ReconcileBlackboxExporterCluster", func() {
		var (
			blackboxExporter *mockcomponent.MockDeployWaiter

			ctx     = context.TODO()
			fakeErr = errors.New("fake err")
		)

		BeforeEach(func() {
			blackboxExporter = mockcomponent.NewMockDeployWaiter(ctrl)

			botanist.Shoot.Components = &shootpkg.Components{
				SystemComponents: &shootpkg.SystemComponents{
					BlackboxExporter: blackboxExporter,
				},
			}
		})

		Context("shoot monitoring is enabled in GardenletConfiguration", func() {
			Context("shoot purpose is not testing", func() {
				It("should fail when the deploy function fails", func() {
					blackboxExporter.EXPECT().Deploy(ctx).Return(fakeErr)

					Expect(botanist.ReconcileBlackboxExporterCluster(ctx)).To(MatchError(fakeErr))
				})

				It("should successfully deploy", func() {
					blackboxExporter.EXPECT().Deploy(ctx)

					Expect(botanist.ReconcileBlackboxExporterCluster(ctx)).To(Succeed())
				})
			})

			Context("shoot purpose is testing", func() {
				BeforeEach(func() {
					botanist.Shoot.Purpose = shootPurposeTesting
				})

				It("should fail when the destroy function fails", func() {
					blackboxExporter.EXPECT().Destroy(ctx).Return(fakeErr)

					Expect(botanist.ReconcileBlackboxExporterCluster(ctx)).To(MatchError(fakeErr))
				})

				It("should successfully destroy", func() {
					blackboxExporter.EXPECT().Destroy(ctx)

					Expect(botanist.ReconcileBlackboxExporterCluster(ctx)).To(Succeed())
				})
			})
		})

		Context("shoot monitoring is disabled in GardenletConfiguration", func() {
			BeforeEach(func() {
				botanist.Config = &gardenletconfigv1alpha1.GardenletConfiguration{
					Monitoring: &gardenletconfigv1alpha1.MonitoringConfig{
						Shoot: &gardenletconfigv1alpha1.ShootMonitoringConfig{
							Enabled: ptr.To(false),
						},
					},
				}
			})

			Context("shoot purpose is not testing", func() {
				It("should successfully destroy", func() {
					blackboxExporter.EXPECT().Destroy(ctx)

					Expect(botanist.ReconcileBlackboxExporterCluster(ctx)).To(Succeed())
				})
			})

			Context("shoot purpose is testing", func() {
				BeforeEach(func() {
					botanist.Shoot.Purpose = shootPurposeTesting
				})

				It("should successfully destroy", func() {
					blackboxExporter.EXPECT().Destroy(ctx)

					Expect(botanist.ReconcileBlackboxExporterCluster(ctx)).To(Succeed())
				})
			})
		})
	})
})
