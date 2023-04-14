// Copyright 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package utils_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/utils/pointer"

	"github.com/gardener/gardener/pkg/apis/core"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/internalversion"
	gardencorelisters "github.com/gardener/gardener/pkg/client/core/listers/core/internalversion"
	. "github.com/gardener/gardener/plugin/pkg/utils"
)

var _ = Describe("Miscellaneous", func() {
	var (
		shoot1 = core.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "shoot1",
				Namespace: "garden-pr1",
			},
			Spec: core.ShootSpec{
				SeedName: pointer.String("seed1"),
			},
		}

		shoot2 = core.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "shoot2",
				Namespace: "garden-pr1",
			},
			Spec: core.ShootSpec{
				SeedName: pointer.String("seed1"),
			},
			Status: core.ShootStatus{
				SeedName: pointer.String("seed2"),
			},
		}

		shoot3 = core.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "shoot3",
				Namespace: "garden-pr1",
			},
			Spec: core.ShootSpec{
				SeedName: nil,
			},
		}
	)

	shoots := []*core.Shoot{
		&shoot1,
		&shoot2,
		&shoot3,
	}
	now := metav1.Now()

	DescribeTable("#SkipVerification",
		func(operation admission.Operation, metadata metav1.ObjectMeta, expected bool) {
			Expect(SkipVerification(operation, metadata)).To(Equal(expected))
		},
		Entry("operation create with nil metadata", admission.Create, nil, false),
		Entry("operation connect with nil metadata", admission.Connect, nil, false),
		Entry("operation delete with nil metadata", admission.Delete, nil, false),
		Entry("operation create and object with deletion timestamp", admission.Create, metav1.ObjectMeta{DeletionTimestamp: &now}, false),
		Entry("operation update and object with deletion timestamp", admission.Update, metav1.ObjectMeta{DeletionTimestamp: &now}, true),
		Entry("operation update and object without deletion timestamp", admission.Update, metav1.ObjectMeta{Name: "obj1"}, false),
	)

	DescribeTable("#IsSeedUsedByShoot",
		func(seedName string, expected bool) {
			Expect(IsSeedUsedByShoot(seedName, shoots)).To(Equal(expected))
		},
		Entry("is used by shoot", "seed1", true),
		Entry("is used by shoot in migration", "seed2", true),
		Entry("is unused", "seed3", false),
	)

	Describe("#NewAttributesWithName", func() {
		It("should return admission.Attributes with the given name", func() {
			name := "name"
			attrs := admission.NewAttributesRecord(&shoot1, nil, core.Kind("Shoot").WithVersion("version"), shoot1.Namespace, "", core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

			newAttrs := NewAttributesWithName(attrs, name)

			Expect(newAttrs.GetName()).To(Equal(name))
		})
	})

	Describe("#ValidateZoneRemovalFromSeeds", func() {
		var (
			seedName = "foo"
			kind     = "foo"

			coreInformerFactory gardencoreinformers.SharedInformerFactory
			shootLister         gardencorelisters.ShootLister

			oldSeedSpec, newSeedSpec *core.SeedSpec
			shoot                    *core.Shoot
		)

		BeforeEach(func() {
			coreInformerFactory = gardencoreinformers.NewSharedInformerFactory(nil, 0)
			shootLister = coreInformerFactory.Core().InternalVersion().Shoots().Lister()

			oldSeedSpec = &core.SeedSpec{
				Provider: core.SeedProvider{
					Zones: []string{"1", "2"},
				},
			}
			newSeedSpec = oldSeedSpec.DeepCopy()

			shoot = &core.Shoot{
				Spec: core.ShootSpec{
					SeedName: &seedName,
				},
				Status: core.ShootStatus{
					SeedName: &seedName,
				},
			}
		})

		It("should do nothing because a new zone was added", func() {
			newSeedSpec.Provider.Zones = append(newSeedSpec.Provider.Zones, "3")

			Expect(ValidateZoneRemovalFromSeeds(oldSeedSpec, newSeedSpec, seedName, shootLister, kind)).To(Succeed())
		})

		It("should do nothing because no zone was removed and no shoots exist", func() {
			Expect(ValidateZoneRemovalFromSeeds(oldSeedSpec, newSeedSpec, seedName, shootLister, kind)).To(Succeed())
		})

		It("should do nothing because no zone was removed even though shoots exist", func() {
			Expect(coreInformerFactory.Core().InternalVersion().Shoots().Informer().GetStore().Add(shoot)).To(Succeed())

			Expect(ValidateZoneRemovalFromSeeds(oldSeedSpec, newSeedSpec, seedName, shootLister, kind)).To(Succeed())
		})

		It("should do nothing because zone was removed and no shoots exist", func() {
			newSeedSpec.Provider.Zones = []string{"2"}

			Expect(ValidateZoneRemovalFromSeeds(oldSeedSpec, newSeedSpec, seedName, shootLister, kind)).To(Succeed())
		})

		It("should return an error because zone was removed even though shoots exist", func() {
			newSeedSpec.Provider.Zones = []string{"2"}
			Expect(coreInformerFactory.Core().InternalVersion().Shoots().Informer().GetStore().Add(shoot)).To(Succeed())

			Expect(ValidateZoneRemovalFromSeeds(oldSeedSpec, newSeedSpec, seedName, shootLister, kind)).To(MatchError(ContainSubstring("cannot remove zones")))
		})
	})
})
