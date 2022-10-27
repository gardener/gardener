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

package botanist

import (
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/operation"
	operationshoot "github.com/gardener/gardener/pkg/operation/shoot"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
)

var _ = Describe("botanist", func() {

	Describe("#GetFailureToleranceType", func() {
		const shootName = "test-shoot"

		var (
			b     *Botanist
			shoot *gardencorev1beta1.Shoot
			o     *operation.Operation
		)

		BeforeEach(func() {
			shoot = &gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name: shootName,
				},
			}
			o = &operation.Operation{
				Shoot: &operationshoot.Shoot{},
			}
			o.Shoot.SetInfo(shoot)
			b = &Botanist{
				Operation: o,
			}
		})

		It("HAControlPlanes gardenlet feature is not enabled", func() {
			Expect(b.GetFailureToleranceType()).To(BeNil())
		})

		It("HAControlPlanes gardenlet feature is enabled and Shoot alpha HA annotation is set", func() {
			shoot.Annotations = map[string]string{
				v1beta1constants.ShootAlphaControlPlaneHighAvailability: v1beta1constants.ShootAlphaControlPlaneHighAvailabilityMultiZone,
			}
			Expect(b.GetFailureToleranceType()).To(PointTo(Equal(gardencorev1beta1.FailureToleranceTypeZone)))
		})

		It("HAControlPlanes gardenlet feature is enabled and Shoot Spec ControlPlane.HighAvailability is set", func() {
			shoot.Spec.ControlPlane = &gardencorev1beta1.ControlPlane{
				HighAvailability: &gardencorev1beta1.HighAvailability{FailureTolerance: gardencorev1beta1.FailureTolerance{Type: gardencorev1beta1.FailureToleranceTypeNode}},
			}
			Expect(b.GetFailureToleranceType()).To(PointTo(Equal(gardencorev1beta1.FailureToleranceTypeNode)))
		})
	})
})
