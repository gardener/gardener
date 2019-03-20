// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package shoot_test

import (
	"testing"

	"github.com/gardener/gardener/pkg/apis/garden"
	strategy "github.com/gardener/gardener/pkg/registry/garden/shoot"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apiserver/pkg/storage"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestHealth(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Shoot Suite")
}

var _ = Describe("ToSelectableFields", func() {
	It("should return correct fields", func() {
		result := strategy.ToSelectableFields(newShoot("foo"))

		Expect(result).To(HaveLen(3))
		Expect(result.Has(garden.ShootSeedName)).To(BeTrue())
		Expect(result.Get(garden.ShootSeedName)).To(Equal("foo"))
	})
})

var _ = Describe("GetAttrs", func() {
	It("should return error when object is not Shoot", func() {
		_, _, err := strategy.GetAttrs(&garden.Seed{})
		Expect(err).To(HaveOccurred())
	})

	It("should return correct result", func() {
		ls, fs, err := strategy.GetAttrs(newShoot("foo"))

		Expect(ls).To(HaveLen(1))
		Expect(ls.Get("foo")).To(Equal("bar"))
		Expect(fs.Get(garden.ShootSeedName)).To(Equal("foo"))
		Expect(err).NotTo(HaveOccurred())
	})
})

var _ = Describe("SeedTriggerFunc", func() {
	It("should return correct matching values", func() {
		expected := []storage.MatchValue{{IndexName: garden.ShootSeedName, Value: "foo"}}

		mv := strategy.SeedTriggerFunc(newShoot("foo"))

		Expect(mv).To(Equal(expected))

	})
})

var _ = Describe("MatchShoot", func() {
	It("should return correct predicate", func() {
		ls, _ := labels.Parse("app=test")
		fs := fields.OneTermEqualSelector(garden.ShootSeedName, "foo")

		result := strategy.MatchShoot(ls, fs)

		Expect(result.Label).To(Equal(ls))
		Expect(result.Field).To(Equal(fs))
		Expect(result.IndexFields).To(ConsistOf(garden.ShootSeedName))

	})
})

func newShoot(seedName string) *garden.Shoot {
	return &garden.Shoot{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "test-namespace",
			Labels:    map[string]string{"foo": "bar"},
		},
		Spec: garden.ShootSpec{
			Cloud: garden.Cloud{
				Seed: &seedName,
			},
		},
	}
}
