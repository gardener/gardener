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
	"k8s.io/utils/ptr"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	kubernetesmock "github.com/gardener/gardener/pkg/client/kubernetes/mock"
	mockcomponent "github.com/gardener/gardener/pkg/component/mock"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	. "github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
	shootpkg "github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
)

var _ = Describe("NodeExporter", func() {
	var (
		ctrl     *gomock.Controller
		botanist *Botanist
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		botanist = &Botanist{Operation: &operation.Operation{
			Config: &gardenletconfigv1alpha1.GardenletConfiguration{
				Monitoring: &gardenletconfigv1alpha1.MonitoringConfig{
					Shoot: &gardenletconfigv1alpha1.ShootMonitoringConfig{
						Enabled: ptr.To(true),
					},
				},
			},
		}}

		botanist.Shoot = &shootpkg.Shoot{}
		botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
			Spec: gardencorev1beta1.ShootSpec{
				Kubernetes: gardencorev1beta1.Kubernetes{
					Version: "1.31.1",
				},
			},
		})
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#DefaultNodeExporter", func() {
		var kubernetesClient *kubernetesmock.MockInterface

		BeforeEach(func() {
			kubernetesClient = kubernetesmock.NewMockInterface(ctrl)

			botanist.SeedClientSet = kubernetesClient
		})

		It("should successfully create a nodeExporter interface", func() {
			kubernetesClient.EXPECT().Client()

			nodeExporter, err := botanist.DefaultNodeExporter()
			Expect(nodeExporter).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("#ReconcileNodeExporter", func() {
		var (
			nodeExporter *mockcomponent.MockDeployWaiter

			ctx     = context.TODO()
			fakeErr = errors.New("fake err")
		)

		BeforeEach(func() {
			nodeExporter = mockcomponent.NewMockDeployWaiter(ctrl)

			botanist.Shoot.Components = &shootpkg.Components{
				SystemComponents: &shootpkg.SystemComponents{
					NodeExporter: nodeExporter,
				},
			}
		})

		Context("Shoot monitoring enabled", func() {
			It("should fail when the deploy function fails", func() {
				nodeExporter.EXPECT().Deploy(ctx).Return(fakeErr)

				Expect(botanist.ReconcileNodeExporter(ctx)).To(MatchError(fakeErr))
			})

			It("should successfully deploy", func() {
				nodeExporter.EXPECT().Deploy(ctx)

				Expect(botanist.ReconcileNodeExporter(ctx)).To(Succeed())
			})
		})

		Context("Shoot monitoring disabled", func() {
			BeforeEach(func() {
				botanist.Operation.Config.Monitoring.Shoot.Enabled = ptr.To(false)
			})

			It("should fail when the destroy function fails", func() {
				nodeExporter.EXPECT().Destroy(ctx).Return(fakeErr)

				Expect(botanist.ReconcileNodeExporter(ctx)).To(MatchError(fakeErr))
			})

			It("should successfully destroy", func() {
				nodeExporter.EXPECT().Destroy(ctx)

				Expect(botanist.ReconcileNodeExporter(ctx)).To(Succeed())
			})
		})

		Context("Shoot purpose is testing", func() {
			BeforeEach(func() {
				botanist.Shoot.Purpose = gardencorev1beta1.ShootPurposeTesting
			})

			It("should fail when the destroy function fails", func() {
				nodeExporter.EXPECT().Destroy(ctx).Return(fakeErr)

				Expect(botanist.ReconcileNodeExporter(ctx)).To(MatchError(fakeErr))
			})

			It("should successfully destroy", func() {
				nodeExporter.EXPECT().Destroy(ctx)

				Expect(botanist.ReconcileNodeExporter(ctx)).To(Succeed())
			})
		})
	})
})
