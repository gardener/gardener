// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package botanist_test

import (
	"context"
	"fmt"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/pointer"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	kubernetesmock "github.com/gardener/gardener/pkg/client/kubernetes/mock"
	mocknodeexporter "github.com/gardener/gardener/pkg/component/nodeexporter/mock"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	"github.com/gardener/gardener/pkg/operation"
	. "github.com/gardener/gardener/pkg/operation/botanist"
	shootpkg "github.com/gardener/gardener/pkg/operation/shoot"
	"github.com/gardener/gardener/pkg/utils/imagevector"
)

var _ = Describe("NodeExporter", func() {
	var (
		ctrl     *gomock.Controller
		botanist *Botanist
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		botanist = &Botanist{Operation: &operation.Operation{
			Config: &config.GardenletConfiguration{
				Monitoring: &config.MonitoringConfig{
					Shoot: &config.ShootMonitoringConfig{
						Enabled: pointer.Bool(true),
					},
				},
			},
		}}

		botanist.Shoot = &shootpkg.Shoot{}
		botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
			Spec: gardencorev1beta1.ShootSpec{
				Kubernetes: gardencorev1beta1.Kubernetes{
					Version: "1.22.1",
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
			botanist.ImageVector = imagevector.ImageVector{{Name: "node-exporter"}}

			nodeExporter, err := botanist.DefaultNodeExporter()
			Expect(nodeExporter).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return an error because the image cannot be found", func() {
			botanist.ImageVector = imagevector.ImageVector{}

			nodeExporter, err := botanist.DefaultNodeExporter()
			Expect(nodeExporter).To(BeNil())
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("#ReconcileNodeExporter", func() {
		var (
			nodeExporter *mocknodeexporter.MockInterface

			ctx     = context.TODO()
			fakeErr = fmt.Errorf("fake err")
		)

		BeforeEach(func() {
			nodeExporter = mocknodeexporter.NewMockInterface(ctrl)

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
				botanist.Operation.Config.Monitoring.Shoot.Enabled = pointer.Bool(false)
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
