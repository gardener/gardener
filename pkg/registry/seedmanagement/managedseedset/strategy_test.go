// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package managedseedset_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/utils/pointer"

	"github.com/gardener/gardener/pkg/apis/core"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/apis/seedmanagement"
	. "github.com/gardener/gardener/pkg/registry/seedmanagement/managedseedset"
)

var _ = Describe("Strategy", func() {
	var (
		ctx      = context.TODO()
		strategy = Strategy{}
	)

	Describe("#PrepareForUpdate", func() {
		var oldManagedSeedSet, newManagedSeedSet *seedmanagement.ManagedSeedSet

		BeforeEach(func() {
			oldManagedSeedSet = &seedmanagement.ManagedSeedSet{}
			newManagedSeedSet = &seedmanagement.ManagedSeedSet{}
		})

		It("should increase the generation if the spec has changed", func() {
			newManagedSeedSet.Spec.Replicas = pointer.Int32(1)

			strategy.PrepareForUpdate(ctx, newManagedSeedSet, oldManagedSeedSet)
			Expect(newManagedSeedSet.Generation).To(Equal(oldManagedSeedSet.Generation + 1))
		})

		It("should increase the generation if the deletion timestamp is set", func() {
			deletionTimestamp := metav1.Now()
			newManagedSeedSet.DeletionTimestamp = &deletionTimestamp

			strategy.PrepareForUpdate(ctx, newManagedSeedSet, oldManagedSeedSet)
			Expect(newManagedSeedSet.Generation).To(Equal(oldManagedSeedSet.Generation + 1))
		})

		It("should increase the generation if the operation annotation with value reconcile was added", func() {
			newManagedSeedSet.Annotations = map[string]string{
				v1beta1constants.GardenerOperation: v1beta1constants.GardenerOperationReconcile,
			}

			strategy.PrepareForUpdate(ctx, newManagedSeedSet, oldManagedSeedSet)
			Expect(newManagedSeedSet.Generation).To(Equal(oldManagedSeedSet.Generation + 1))
			Expect(newManagedSeedSet.Annotations).To(BeEmpty())
		})

		It("should not increase the generation if neither the spec has changed nor the deletion timestamp is set", func() {
			strategy.PrepareForUpdate(ctx, newManagedSeedSet, oldManagedSeedSet)
			Expect(newManagedSeedSet.Generation).To(Equal(oldManagedSeedSet.Generation))
		})
	})
})

var _ = Describe("ToSelectableFields", func() {
	It("should return correct fields", func() {
		result := ToSelectableFields(newManagedSeedSet())

		Expect(result).To(HaveLen(2))
	})
})

var _ = Describe("GetAttrs", func() {
	It("should return error when object is not ManagedSeedSet", func() {
		_, _, err := GetAttrs(&core.Seed{})
		Expect(err).To(HaveOccurred())
	})

	It("should return correct result", func() {
		ls, _, err := GetAttrs(newManagedSeedSet())

		Expect(ls).To(HaveLen(1))
		Expect(ls.Get("foo")).To(Equal("bar"))
		Expect(err).NotTo(HaveOccurred())
	})
})

var _ = Describe("MatchManagedSeedSet", func() {
	It("should return correct predicate", func() {
		ls, _ := labels.Parse("app=test")

		result := MatchManagedSeedSet(ls, nil)

		Expect(result.Label).To(Equal(ls))
	})
})

func newManagedSeedSet() *seedmanagement.ManagedSeedSet {
	return &seedmanagement.ManagedSeedSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "test-namespace",
			Labels:    map[string]string{"foo": "bar"},
		},
		Spec: seedmanagement.ManagedSeedSetSpec{},
	}
}
