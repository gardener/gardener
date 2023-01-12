// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	kubernetesmock "github.com/gardener/gardener/pkg/client/kubernetes/mock"
	"github.com/gardener/gardener/pkg/features"
	gardenletfeatures "github.com/gardener/gardener/pkg/gardenlet/features"
	"github.com/gardener/gardener/pkg/operation"
	. "github.com/gardener/gardener/pkg/operation/botanist"
	mockkubernetesdashboard "github.com/gardener/gardener/pkg/operation/botanist/component/kubernetesdashboard/mock"
	"github.com/gardener/gardener/pkg/operation/garden"
	shootpkg "github.com/gardener/gardener/pkg/operation/shoot"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	"github.com/gardener/gardener/pkg/utils/test"
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
					Version: "1.22.1",
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
			defer test.WithFeatureGate(gardenletfeatures.FeatureGate, features.APIServerSNI, true)()

			botanist.ImageVector = imagevector.ImageVector{
				{Name: "kubernetes-dashboard"},
				{Name: "kubernetes-dashboard-metrics-scraper"},
			}

			kubernetesDashboard, err := botanist.DefaultKubernetesDashboard()
			Expect(kubernetesDashboard).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return an error because the kubernetes-dashboard image cannot be found", func() {
			botanist.ImageVector = imagevector.ImageVector{
				{Name: "kubernetes-dashboard-metrics-scraper"},
			}

			kubernetesDashboard, err := botanist.DefaultKubernetesDashboard()
			Expect(kubernetesDashboard).To(BeNil())
			Expect(err).To(HaveOccurred())
		})

		It("should return an error because the kubernetes-dashboard-metrics-scraper image cannot be found", func() {
			botanist.ImageVector = imagevector.ImageVector{
				{Name: "kubernetes-dashboard"},
			}

			kubernetesDashboard, err := botanist.DefaultKubernetesDashboard()
			Expect(kubernetesDashboard).To(BeNil())
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("#DeployKubernetesDashboard", func() {
		var (
			kubernetesdashboard *mockkubernetesdashboard.MockInterface

			ctx     = context.TODO()
			fakeErr = fmt.Errorf("fake err")
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
