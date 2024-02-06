// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package botanist

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	"k8s.io/utils/ptr"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	kubernetesmock "github.com/gardener/gardener/pkg/client/kubernetes/mock"
	mockblackboxexporter "github.com/gardener/gardener/pkg/component/blackboxexporter/mock"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	"github.com/gardener/gardener/pkg/operation"
	shootpkg "github.com/gardener/gardener/pkg/operation/shoot"
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
					Version: "1.26.1",
				},
				Purpose: &shootPurposeEvaluation,
			},
		})
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#DefaultBlackboxExporter", func() {
		var kubernetesClient *kubernetesmock.MockInterface

		BeforeEach(func() {
			kubernetesClient = kubernetesmock.NewMockInterface(ctrl)

			botanist.SeedClientSet = kubernetesClient
		})

		It("should successfully create a blackbox-exporter interface", func() {
			kubernetesClient.EXPECT().Client()

			blackboxExporter, err := botanist.DefaultBlackboxExporter()
			Expect(blackboxExporter).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("#ReconcileBlackboxExporter", func() {
		var (
			blackboxExporter *mockblackboxexporter.MockInterface

			ctx     = context.TODO()
			fakeErr = fmt.Errorf("fake err")
		)

		BeforeEach(func() {
			blackboxExporter = mockblackboxexporter.NewMockInterface(ctrl)

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

					Expect(botanist.ReconcileBlackboxExporter(ctx)).To(MatchError(fakeErr))
				})

				It("should successfully deploy", func() {
					blackboxExporter.EXPECT().Deploy(ctx)

					Expect(botanist.ReconcileBlackboxExporter(ctx)).To(Succeed())
				})
			})

			Context("shoot purpose is testing", func() {
				BeforeEach(func() {
					botanist.Shoot.Purpose = shootPurposeTesting
				})

				It("should fail when the destroy function fails", func() {
					blackboxExporter.EXPECT().Destroy(ctx).Return(fakeErr)

					Expect(botanist.ReconcileBlackboxExporter(ctx)).To(MatchError(fakeErr))
				})

				It("should successfully destroy", func() {
					blackboxExporter.EXPECT().Destroy(ctx)

					Expect(botanist.ReconcileBlackboxExporter(ctx)).To(Succeed())
				})
			})
		})

		Context("shoot monitoring is disabled in GardenletConfiguration", func() {
			BeforeEach(func() {
				botanist.Config = &config.GardenletConfiguration{
					Monitoring: &config.MonitoringConfig{
						Shoot: &config.ShootMonitoringConfig{
							Enabled: ptr.To(false),
						},
					},
				}
			})

			Context("shoot purpose is not testing", func() {
				It("should successfully destroy", func() {
					blackboxExporter.EXPECT().Destroy(ctx)

					Expect(botanist.ReconcileBlackboxExporter(ctx)).To(Succeed())
				})
			})

			Context("shoot purpose is testing", func() {
				BeforeEach(func() {
					botanist.Shoot.Purpose = shootPurposeTesting
				})

				It("should successfully destroy", func() {
					blackboxExporter.EXPECT().Destroy(ctx)

					Expect(botanist.ReconcileBlackboxExporter(ctx)).To(Succeed())
				})
			})
		})
	})
})
