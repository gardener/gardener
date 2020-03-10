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

	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/apis/core/validation"
	shootregistry "github.com/gardener/gardener/pkg/registry/core/shoot"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
)

var _ = Describe("Strategy", func() {
	Context("PrepareForUpdate", func() {
		Context("pod pids limit", func() {
			It("should enforce the minimum limit value", func() {
				var tooSmallValue int64 = 10

				shoot := newShoot("foo")
				shoot.Spec.Kubernetes.Kubelet = &core.KubeletConfig{
					PodPIDsLimit: &tooSmallValue,
				}
				shoot.Spec.Provider.Workers = []core.Worker{
					{
						Kubernetes: &core.WorkerKubernetes{
							Kubelet: &core.KubeletConfig{
								PodPIDsLimit: &tooSmallValue,
							},
						},
					},
				}

				oldShoot := newShoot("foo")
				oldShoot.Spec.Kubernetes.Kubelet = shoot.Spec.Kubernetes.Kubelet.DeepCopy()
				oldShoot.Spec.Provider.Workers = []core.Worker{*shoot.Spec.Provider.Workers[0].DeepCopy()}

				shootregistry.Strategy.PrepareForUpdate(context.TODO(), shoot, oldShoot)

				Expect(*shoot.Spec.Kubernetes.Kubelet.PodPIDsLimit).To(Equal(validation.PodPIDsLimitMinimum))
				Expect(*oldShoot.Spec.Kubernetes.Kubelet.PodPIDsLimit).To(Equal(validation.PodPIDsLimitMinimum))
				Expect(*shoot.Spec.Provider.Workers[0].Kubernetes.Kubelet.PodPIDsLimit).To(Equal(validation.PodPIDsLimitMinimum))
				Expect(*oldShoot.Spec.Provider.Workers[0].Kubernetes.Kubelet.PodPIDsLimit).To(Equal(validation.PodPIDsLimitMinimum))
			})

			It("should not touch values that are above the minimum", func() {
				var (
					tooSmallValue   int64 = 10
					highEnoughValue int64 = 105
				)

				shoot := newShoot("foo")
				shoot.Spec.Kubernetes.Kubelet = &core.KubeletConfig{
					PodPIDsLimit: &tooSmallValue,
				}
				shoot.Spec.Provider.Workers = []core.Worker{
					{
						Kubernetes: &core.WorkerKubernetes{
							Kubelet: &core.KubeletConfig{
								PodPIDsLimit: &highEnoughValue,
							},
						},
					},
				}

				oldShoot := newShoot("foo")
				oldShoot.Spec.Kubernetes.Kubelet = shoot.Spec.Kubernetes.Kubelet.DeepCopy()
				oldShoot.Spec.Provider.Workers = []core.Worker{*shoot.Spec.Provider.Workers[0].DeepCopy()}

				shootregistry.Strategy.PrepareForUpdate(context.TODO(), shoot, oldShoot)

				Expect(*shoot.Spec.Kubernetes.Kubelet.PodPIDsLimit).To(Equal(validation.PodPIDsLimitMinimum))
				Expect(*oldShoot.Spec.Kubernetes.Kubelet.PodPIDsLimit).To(Equal(validation.PodPIDsLimitMinimum))
				Expect(*shoot.Spec.Provider.Workers[0].Kubernetes.Kubelet.PodPIDsLimit).To(Equal(highEnoughValue))
				Expect(*oldShoot.Spec.Provider.Workers[0].Kubernetes.Kubelet.PodPIDsLimit).To(Equal(highEnoughValue))
			})
		})
	})
})

var _ = Describe("ToSelectableFields", func() {
	It("should return correct fields", func() {
		result := shootregistry.ToSelectableFields(newShoot("foo"))

		Expect(result).To(HaveLen(4))
		Expect(result.Has(core.ShootSeedName)).To(BeTrue())
		Expect(result.Get(core.ShootSeedName)).To(Equal("foo"))
		Expect(result.Has(core.ShootCloudProfileName)).To(BeTrue())
		Expect(result.Get(core.ShootCloudProfileName)).To(Equal("baz"))
	})
})

var _ = Describe("GetAttrs", func() {
	It("should return error when object is not Shoot", func() {
		_, _, err := shootregistry.GetAttrs(&core.Seed{})
		Expect(err).To(HaveOccurred())
	})

	It("should return correct result", func() {
		ls, fs, err := shootregistry.GetAttrs(newShoot("foo"))

		Expect(ls).To(HaveLen(1))
		Expect(ls.Get("foo")).To(Equal("bar"))
		Expect(fs.Get(core.ShootSeedName)).To(Equal("foo"))
		Expect(err).NotTo(HaveOccurred())
	})
})

var _ = Describe("SeedNameTriggerFunc", func() {
	It("should return spec.seedName", func() {
		actual := shootregistry.SeedNameTriggerFunc(newShoot("foo"))
		Expect(actual).To(Equal("foo"))
	})
})

var _ = Describe("MatchShoot", func() {
	It("should return correct predicate", func() {
		ls, _ := labels.Parse("app=test")
		fs := fields.OneTermEqualSelector(core.ShootSeedName, "foo")

		result := shootregistry.MatchShoot(ls, fs)

		Expect(result.Label).To(Equal(ls))
		Expect(result.Field).To(Equal(fs))
		Expect(result.IndexFields).To(ConsistOf(core.ShootSeedName))
	})
})

func newShoot(seedName string) *core.Shoot {
	return &core.Shoot{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "test-namespace",
			Labels:    map[string]string{"foo": "bar"},
		},
		Spec: core.ShootSpec{
			CloudProfileName: "baz",
			SeedName:         &seedName,
		},
	}
}
