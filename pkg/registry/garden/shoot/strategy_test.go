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
	"context"
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

		Expect(result).To(HaveLen(5))
		Expect(result.Has(garden.ShootSeedNameDeprecated)).To(BeTrue())
		Expect(result.Get(garden.ShootSeedNameDeprecated)).To(Equal("foo"))
		Expect(result.Has(garden.ShootSeedName)).To(BeTrue())
		Expect(result.Get(garden.ShootSeedName)).To(Equal("foo"))
		Expect(result.Has(garden.ShootCloudProfileName)).To(BeTrue())
		Expect(result.Get(garden.ShootCloudProfileName)).To(Equal("baz"))
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
		Expect(fs.Get(garden.ShootSeedNameDeprecated)).To(Equal("foo"))
		Expect(fs.Get(garden.ShootSeedName)).To(Equal("foo"))
		Expect(err).NotTo(HaveOccurred())
	})
})

var _ = Describe("TriggerFunc", func() {
	It("should return correct matching values", func() {
		expected := []storage.MatchValue{
			{IndexName: garden.ShootSeedNameDeprecated, Value: "foo"},
			{IndexName: garden.ShootSeedName, Value: "foo"},
			{IndexName: garden.ShootCloudProfileName, Value: "baz"},
		}

		mv := strategy.TriggerFunc(newShoot("foo"))

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
		Expect(result.IndexFields).To(ConsistOf(garden.ShootSeedNameDeprecated, garden.ShootSeedName, garden.ShootCloudProfileName))
	})
})

var _ = Describe("Strategy", func() {

	Context("PrepareForUpdate", func() {
		Context("invalid GCP network CIRDs", func() {
			It("should remove more than one GCP networks", func() {
				shoot := newShoot("foo")

				shoot.Spec.Cloud.GCP = &garden.GCPCloud{
					Networks: garden.GCPNetworks{
						Workers: []string{"1.1.1.1/32", "1.1.1.2/32"},
					},
				}
				oldShoot := newShoot("foo")
				oldShoot.Spec.Cloud.GCP = shoot.Spec.Cloud.GCP.DeepCopy()

				strategy.Strategy.PrepareForUpdate(context.TODO(), shoot, oldShoot)

				Expect(shoot.Spec.Cloud.GCP.Networks.Workers).To(ConsistOf("1.1.1.1/32"))
				Expect(oldShoot.Spec.Cloud.GCP.Networks.Workers).To(ConsistOf("1.1.1.1/32"))
			})
		})
		Context("invalid Openstack network CIRDs", func() {
			It("should remove more than one OpenStack networks", func() {
				shoot := newShoot("foo")

				shoot.Spec.Cloud.OpenStack = &garden.OpenStackCloud{
					Networks: garden.OpenStackNetworks{
						Workers: []string{"1.1.1.1/32", "1.1.1.2/32"},
					},
				}
				oldShoot := newShoot("foo")
				oldShoot.Spec.Cloud.OpenStack = shoot.Spec.Cloud.OpenStack.DeepCopy()

				strategy.Strategy.PrepareForUpdate(context.TODO(), shoot, oldShoot)

				Expect(shoot.Spec.Cloud.OpenStack.Networks.Workers).To(ConsistOf("1.1.1.1/32"))
				Expect(oldShoot.Spec.Cloud.OpenStack.Networks.Workers).To(ConsistOf("1.1.1.1/32"))
			})
		})
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
			CloudProfileName: "baz",
			SeedName:         &seedName,
		},
	}
}
