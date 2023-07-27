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

	"github.com/Masterminds/semver"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	kubernetesmock "github.com/gardener/gardener/pkg/client/kubernetes/mock"
	mockvpa "github.com/gardener/gardener/pkg/component/vpa/mock"
	"github.com/gardener/gardener/pkg/operation"
	. "github.com/gardener/gardener/pkg/operation/botanist"
	seedpkg "github.com/gardener/gardener/pkg/operation/seed"
	shootpkg "github.com/gardener/gardener/pkg/operation/shoot"
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
			botanist.Seed.KubernetesVersion = semver.MustParse("v1.25.0")
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
			fakeErr = fmt.Errorf("fake err")
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
