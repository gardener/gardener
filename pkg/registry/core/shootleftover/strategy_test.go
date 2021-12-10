// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package shootleftover_test

import (
	"context"

	"github.com/gardener/gardener/pkg/apis/core"
	. "github.com/gardener/gardener/pkg/registry/core/shootleftover"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
)

var _ = Describe("Strategy", func() {
	var (
		ctx      = context.TODO()
		strategy = Strategy{}
	)

	Describe("#PrepareForUpdate", func() {
		var oldShootLeftover, newShootLeftover *core.ShootLeftover

		BeforeEach(func() {
			oldShootLeftover = &core.ShootLeftover{}
			newShootLeftover = &core.ShootLeftover{}
		})

		It("should increase the generation if the spec has changed", func() {
			newShootLeftover.Spec.SeedName = "foo"

			strategy.PrepareForUpdate(ctx, newShootLeftover, oldShootLeftover)
			Expect(newShootLeftover.Generation).To(Equal(oldShootLeftover.Generation + 1))
		})

		It("should increase the generation if the deletion timestamp is set", func() {
			deletionTimestamp := metav1.Now()
			newShootLeftover.DeletionTimestamp = &deletionTimestamp

			strategy.PrepareForUpdate(ctx, newShootLeftover, oldShootLeftover)
			Expect(newShootLeftover.Generation).To(Equal(oldShootLeftover.Generation + 1))
		})

		It("should not increase the generation if neither the spec has changed nor the deletion timestamp is set", func() {
			strategy.PrepareForUpdate(ctx, newShootLeftover, oldShootLeftover)
			Expect(newShootLeftover.Generation).To(Equal(oldShootLeftover.Generation))
		})
	})
})

var _ = Describe("ToSelectableFields", func() {
	It("should return correct fields", func() {
		result := ToSelectableFields(newShootLeftover("foo"))

		Expect(result).To(HaveLen(3))
		Expect(result.Has(core.ShootLeftoverSeedName)).To(BeTrue())
		Expect(result.Get(core.ShootLeftoverSeedName)).To(Equal("foo"))
	})
})

var _ = Describe("GetAttrs", func() {
	It("should return error when object is not ShootLeftover", func() {
		_, _, err := GetAttrs(&core.Seed{})
		Expect(err).To(HaveOccurred())
	})

	It("should return correct result", func() {
		ls, fs, err := GetAttrs(newShootLeftover("foo"))

		Expect(ls).To(HaveLen(1))
		Expect(ls.Get("foo")).To(Equal("bar"))
		Expect(fs.Get(core.ShootLeftoverSeedName)).To(Equal("foo"))
		Expect(err).NotTo(HaveOccurred())
	})
})

var _ = Describe("ShootNameTriggerFunc", func() {
	It("should return spec.seedName", func() {
		actual := SeedNameTriggerFunc(newShootLeftover("foo"))
		Expect(actual).To(Equal("foo"))
	})
})

var _ = Describe("MatchShootLeftover", func() {
	It("should return correct predicate", func() {
		ls, _ := labels.Parse("app=test")
		fs := fields.OneTermEqualSelector(core.ShootLeftoverSeedName, "foo")

		result := MatchShootLeftover(ls, fs)

		Expect(result.Label).To(Equal(ls))
		Expect(result.Field).To(Equal(fs))
		Expect(result.IndexFields).To(ConsistOf(core.ShootLeftoverSeedName))
	})
})

func newShootLeftover(seedName string) *core.ShootLeftover {
	return &core.ShootLeftover{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "test-namespace",
			Labels:    map[string]string{"foo": "bar"},
		},
		Spec: core.ShootLeftoverSpec{
			SeedName: seedName,
		},
	}
}
