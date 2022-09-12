// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package shoot

import (
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/features"
	gardenletfeatures "github.com/gardener/gardener/pkg/gardenlet/features"
	"github.com/gardener/gardener/pkg/operation/botanist/component/etcd"
	"github.com/gardener/gardener/pkg/operation/shoot"
	"github.com/gardener/gardener/pkg/utils/test"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("shoot control reconcile", func() {
	Describe("#getEtcdDeployTimeout", func() {
		var (
			s              *shoot.Shoot
			defaultTimeout time.Duration
		)

		BeforeEach(func() {
			s = &shoot.Shoot{}
			s.SetInfo(&gardencorev1beta1.Shoot{})
			defaultTimeout = 30 * time.Second
		})

		Context("deploy timeout for etcd in non-ha shoot", func() {
			It("HAControlPlanes feature is not enabled", func() {
				test.WithFeatureGate(gardenletfeatures.FeatureGate, features.HAControlPlanes, false)
				Expect(getEtcdDeployTimeout(s, defaultTimeout)).To(Equal(defaultTimeout))
			})

			It("HAControlPlanes feature is enabled but shoot is not marked to have HA control plane", func() {
				test.WithFeatureGate(gardenletfeatures.FeatureGate, features.HAControlPlanes, true)
				Expect(getEtcdDeployTimeout(s, defaultTimeout)).To(Equal(defaultTimeout))
			})

			It("HAControlPlanes feature is enabled, shoot spec has empty ControlPlane", func() {
				test.WithFeatureGate(gardenletfeatures.FeatureGate, features.HAControlPlanes, true)
				s.GetInfo().Spec.ControlPlane = &gardencorev1beta1.ControlPlane{}
				Expect(getEtcdDeployTimeout(s, defaultTimeout)).To(Equal(defaultTimeout))
			})

			It("HAControlPlanes feature is enabled and s is marked as multi-zonal", func() {
				test.WithFeatureGate(gardenletfeatures.FeatureGate, features.HAControlPlanes, true)
				s.GetInfo().Spec.ControlPlane = &gardencorev1beta1.ControlPlane{
					HighAvailability: &gardencorev1beta1.HighAvailability{FailureTolerance: gardencorev1beta1.FailureTolerance{Type: gardencorev1beta1.FailureToleranceTypeNode}},
				}
				Expect(getEtcdDeployTimeout(s, defaultTimeout)).To(Equal(etcd.DefaultTimeout))
			})
		})
	})

})
