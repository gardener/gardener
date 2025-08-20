// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardener_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/pkg/utils/gardener"
)

var _ = Describe("Identity", func() {
	Describe("#MaintainSeedNameLabels", func() {
		It("should maintain the labels", func() {
			obj := &gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"name.seed.gardener.cloud/old-seed": "true"}},
				Spec:       gardencorev1beta1.ShootSpec{SeedName: ptr.To("spec-seed")},
				Status:     gardencorev1beta1.ShootStatus{SeedName: ptr.To("status-seed")},
			}

			MaintainSeedNameLabels(obj, obj.Spec.SeedName, obj.Status.SeedName)

			Expect(obj.Labels).To(And(
				HaveKeyWithValue("name.seed.gardener.cloud/spec-seed", "true"),
				HaveKeyWithValue("name.seed.gardener.cloud/status-seed", "true"),
			))
		})

		It("should maintain the labels when spec and status names are equal", func() {
			obj := &gardencorev1beta1.Shoot{
				Spec:   gardencorev1beta1.ShootSpec{SeedName: ptr.To("seed")},
				Status: gardencorev1beta1.ShootStatus{SeedName: ptr.To("seed")},
			}

			MaintainSeedNameLabels(obj, obj.Spec.SeedName, obj.Status.SeedName)

			Expect(obj.Labels).To(HaveKeyWithValue("name.seed.gardener.cloud/seed", "true"))
		})

		It("should maintain the labels when spec and status names are empty", func() {
			obj := &gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"foo": "bar", "name.seed.gardener.cloud/old-seed": "true"}},
			}

			MaintainSeedNameLabels(obj, obj.Spec.SeedName, obj.Status.SeedName)

			Expect(obj.Labels).To(Equal(map[string]string{"foo": "bar"}))
		})
	})
})
