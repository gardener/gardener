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

	"github.com/Masterminds/semver"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	kubernetesmock "github.com/gardener/gardener/pkg/client/kubernetes/mock"
	"github.com/gardener/gardener/pkg/operation"
	. "github.com/gardener/gardener/pkg/operation/botanist"
	mockvpa "github.com/gardener/gardener/pkg/operation/botanist/component/vpa/mock"
	seedpkg "github.com/gardener/gardener/pkg/operation/seed"
	shootpkg "github.com/gardener/gardener/pkg/operation/shoot"
	"github.com/gardener/gardener/pkg/utils/imagevector"
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
			botanist.Seed.SetInfo(&gardencorev1beta1.Seed{})
			botanist.Seed.KubernetesVersion = semver.MustParse("v1.25.0")
			botanist.Shoot = &shootpkg.Shoot{}
			botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{})
		})

		It("should successfully create a VPA component", func() {
			botanist.ImageVector = imagevector.ImageVector{
				{Name: "vpa-admission-controller"},
				{Name: "vpa-recommender"},
				{Name: "vpa-updater"},
			}

			vpa, err := botanist.DefaultVerticalPodAutoscaler()
			Expect(vpa).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return an error because the vpa-admission-controller image cannot be found", func() {
			botanist.ImageVector = imagevector.ImageVector{
				{Name: "vpa-recommender"},
				{Name: "vpa-updater"},
			}

			vpa, err := botanist.DefaultVerticalPodAutoscaler()
			Expect(vpa).To(BeNil())
			Expect(err).To(HaveOccurred())
		})

		It("should return an error because the vpa-recommender image cannot be found", func() {
			botanist.ImageVector = imagevector.ImageVector{
				{Name: "vpa-admission-controller"},
				{Name: "vpa-updater"},
			}

			vpa, err := botanist.DefaultVerticalPodAutoscaler()
			Expect(vpa).To(BeNil())
			Expect(err).To(HaveOccurred())
		})

		It("should return an error because the vpa-updater image cannot be found", func() {
			botanist.ImageVector = imagevector.ImageVector{
				{Name: "vpa-admission-controller"},
				{Name: "vpa-recommender"},
			}

			vpa, err := botanist.DefaultVerticalPodAutoscaler()
			Expect(vpa).To(BeNil())
			Expect(err).To(HaveOccurred())
		})

		DescribeTable("should correctly set topology-aware routing value",
			func(seed *gardencorev1beta1.Seed, shoot *gardencorev1beta1.Shoot, matcher gomegatypes.GomegaMatcher) {
				botanist.ImageVector = imagevector.ImageVector{
					{Name: "vpa-admission-controller"},
					{Name: "vpa-recommender"},
					{Name: "vpa-updater"},
				}

				botanist.Seed.SetInfo(seed)
				botanist.Shoot.SetInfo(shoot)

				vpa, err := botanist.DefaultVerticalPodAutoscaler()
				Expect(vpa).NotTo(BeNil())
				Expect(err).NotTo(HaveOccurred())
				values := vpa.GetValues()
				Expect(values.AdmissionController.TopologyAwareRoutingEnabled).To(matcher)
			},

			Entry("seed setting is nil, shoot control plane is not HA",
				&gardencorev1beta1.Seed{Spec: gardencorev1beta1.SeedSpec{Settings: nil}},
				&gardencorev1beta1.Shoot{Spec: gardencorev1beta1.ShootSpec{ControlPlane: &gardencorev1beta1.ControlPlane{HighAvailability: nil}}},
				BeFalse(),
			),
			Entry("seed setting is disabled, shoot control plane is not HA",
				&gardencorev1beta1.Seed{Spec: gardencorev1beta1.SeedSpec{Settings: &gardencorev1beta1.SeedSettings{TopologyAwareRouting: &gardencorev1beta1.SeedSettingTopologyAwareRouting{Enabled: false}}}},
				&gardencorev1beta1.Shoot{Spec: gardencorev1beta1.ShootSpec{ControlPlane: &gardencorev1beta1.ControlPlane{HighAvailability: nil}}},
				BeFalse(),
			),
			Entry("seed setting is enabled, shoot control plane is not HA",
				&gardencorev1beta1.Seed{Spec: gardencorev1beta1.SeedSpec{Settings: &gardencorev1beta1.SeedSettings{TopologyAwareRouting: &gardencorev1beta1.SeedSettingTopologyAwareRouting{Enabled: true}}}},
				&gardencorev1beta1.Shoot{Spec: gardencorev1beta1.ShootSpec{ControlPlane: &gardencorev1beta1.ControlPlane{HighAvailability: nil}}},
				BeFalse(),
			),
			Entry("seed setting is nil, shoot control plane is HA with failure tolerance type 'zone'",
				&gardencorev1beta1.Seed{Spec: gardencorev1beta1.SeedSpec{Settings: nil}},
				&gardencorev1beta1.Shoot{Spec: gardencorev1beta1.ShootSpec{ControlPlane: &gardencorev1beta1.ControlPlane{HighAvailability: &gardencorev1beta1.HighAvailability{FailureTolerance: gardencorev1beta1.FailureTolerance{Type: gardencorev1beta1.FailureToleranceTypeZone}}}}},
				BeFalse(),
			),
			Entry("seed setting is disabled, shoot control plane is HA with failure tolerance type 'zone'",
				&gardencorev1beta1.Seed{Spec: gardencorev1beta1.SeedSpec{Settings: &gardencorev1beta1.SeedSettings{TopologyAwareRouting: &gardencorev1beta1.SeedSettingTopologyAwareRouting{Enabled: false}}}},
				&gardencorev1beta1.Shoot{Spec: gardencorev1beta1.ShootSpec{ControlPlane: &gardencorev1beta1.ControlPlane{HighAvailability: &gardencorev1beta1.HighAvailability{FailureTolerance: gardencorev1beta1.FailureTolerance{Type: gardencorev1beta1.FailureToleranceTypeZone}}}}},
				BeFalse(),
			),
			Entry("seed setting is enabled, shoot control plane is HA with failure tolerance type 'zone'",
				&gardencorev1beta1.Seed{Spec: gardencorev1beta1.SeedSpec{Settings: &gardencorev1beta1.SeedSettings{TopologyAwareRouting: &gardencorev1beta1.SeedSettingTopologyAwareRouting{Enabled: true}}}},
				&gardencorev1beta1.Shoot{Spec: gardencorev1beta1.ShootSpec{ControlPlane: &gardencorev1beta1.ControlPlane{HighAvailability: &gardencorev1beta1.HighAvailability{FailureTolerance: gardencorev1beta1.FailureTolerance{Type: gardencorev1beta1.FailureToleranceTypeZone}}}}},
				BeTrue(),
			),
		)
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
