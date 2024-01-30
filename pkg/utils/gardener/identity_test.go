// Copyright 2024 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package gardener_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/pkg/utils/gardener"
)

var _ = Describe("Identity", func() {
	Describe("#MaintainSeedNameLabels", func() {
		It("should maintain the labels", func() {
			obj := &gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"seed.gardener.cloud/old-seed": "true"}},
				Spec:       gardencorev1beta1.ShootSpec{SeedName: pointer.String("spec-seed")},
				Status:     gardencorev1beta1.ShootStatus{SeedName: pointer.String("status-seed")},
			}

			MaintainSeedNameLabels(obj, obj.Spec.SeedName, obj.Status.SeedName)

			Expect(obj.Labels).To(And(
				HaveKeyWithValue("seed.gardener.cloud/spec-seed", "true"),
				HaveKeyWithValue("seed.gardener.cloud/status-seed", "true"),
			))
		})

		It("should maintain the labels when spec and status names are equal", func() {
			obj := &gardencorev1beta1.Shoot{
				Spec:   gardencorev1beta1.ShootSpec{SeedName: pointer.String("seed")},
				Status: gardencorev1beta1.ShootStatus{SeedName: pointer.String("seed")},
			}

			MaintainSeedNameLabels(obj, obj.Spec.SeedName, obj.Status.SeedName)

			Expect(obj.Labels).To(HaveKeyWithValue("seed.gardener.cloud/seed", "true"))
		})

		It("should maintain the labels when spec and status names are empty", func() {
			obj := &gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"foo": "bar", "seed.gardener.cloud/old-seed": "true"}},
			}

			MaintainSeedNameLabels(obj, obj.Spec.SeedName, obj.Status.SeedName)

			Expect(obj.Labels).To(Equal(map[string]string{"foo": "bar"}))
		})
	})
})
