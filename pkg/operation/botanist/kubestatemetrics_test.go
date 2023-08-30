// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	kubernetesmock "github.com/gardener/gardener/pkg/client/kubernetes/mock"
	mockkubestatemetrics "github.com/gardener/gardener/pkg/component/kubestatemetrics/mock"
	"github.com/gardener/gardener/pkg/operation"
	. "github.com/gardener/gardener/pkg/operation/botanist"
	shootpkg "github.com/gardener/gardener/pkg/operation/shoot"
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
			kubeStateMetrics *mockkubestatemetrics.MockInterface

			ctx     = context.TODO()
			fakeErr = fmt.Errorf("fake err")
		)

		BeforeEach(func() {
			kubeStateMetrics = mockkubestatemetrics.NewMockInterface(ctrl)

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
