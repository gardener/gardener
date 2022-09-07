// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/features"
	gardenletfeatures "github.com/gardener/gardener/pkg/gardenlet/features"
	"github.com/gardener/gardener/pkg/utils/test"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
)

var _ = Describe("#GetFailureToleranceType", func() {
	var shoot *gardencorev1beta1.Shoot

	BeforeEach(func() {
		shoot = &gardencorev1beta1.Shoot{}
	})

	It("gardenlet HAControlPlanes feature gate is not enabled", func() {
		Expect(GetFailureToleranceType(shoot)).To(BeNil())
	})

	It("gardenlet HAControlPlanes feature is enabled and alpha annotation is set", func() {
		test.WithFeatureGate(gardenletfeatures.FeatureGate, features.HAControlPlanes, true)
		shoot.Annotations = map[string]string{
			v1beta1constants.ShootAlphaControlPlaneHighAvailability: v1beta1constants.ShootAlphaControlPlaneHighAvailabilityMultiZone,
		}
		Expect(GetFailureToleranceType(shoot)).To(PointTo(Equal(gardencorev1beta1.FailureToleranceTypeZone)))
	})

	It("gardenlet HAControlPlanes feature is enabled and Shoot spec ControlPlane.HighAvailability is set", func() {
		test.WithFeatureGate(gardenletfeatures.FeatureGate, features.HAControlPlanes, true)
		shoot.Spec.ControlPlane = &gardencorev1beta1.ControlPlane{
			HighAvailability: &gardencorev1beta1.HighAvailability{FailureTolerance: gardencorev1beta1.FailureTolerance{FailureToleranceType: gardencorev1beta1.FailureToleranceTypeNode}},
		}
		Expect(GetFailureToleranceType(shoot)).To(PointTo(Equal(gardencorev1beta1.FailureToleranceTypeNode)))
	})
})
