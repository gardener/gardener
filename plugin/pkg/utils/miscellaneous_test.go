// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package utils_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	gardencorev1beta1listers "github.com/gardener/gardener/pkg/client/core/listers/core/v1beta1"
	. "github.com/gardener/gardener/plugin/pkg/utils"
)

var _ = Describe("Miscellaneous", func() {
	var (
		now metav1.Time

		coreInformerFactory gardencoreinformers.SharedInformerFactory
		shootLister         gardencorev1beta1listers.ShootLister

		shoots []*gardencorev1beta1.Shoot
		shoot1 = &gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "shoot1",
				Namespace: "garden-pr1",
			},
			Spec: gardencorev1beta1.ShootSpec{
				SeedName: ptr.To("seed1"),
			},
		}

		shoot2 = &gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "shoot2",
				Namespace: "garden-pr1",
			},
			Spec: gardencorev1beta1.ShootSpec{
				SeedName: ptr.To("seed1"),
			},
			Status: gardencorev1beta1.ShootStatus{
				SeedName: ptr.To("seed2"),
			},
		}

		shoot3 = &gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "shoot3",
				Namespace: "garden-pr1",
			},
			Spec: gardencorev1beta1.ShootSpec{
				SeedName: nil,
			},
		}
	)

	BeforeEach(func() {
		now = metav1.Now()

		coreInformerFactory = gardencoreinformers.NewSharedInformerFactory(nil, 0)
		shootLister = coreInformerFactory.Core().V1beta1().Shoots().Lister()

		shoots = []*gardencorev1beta1.Shoot{
			shoot1,
			shoot2,
			shoot3,
		}
	})

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

	Describe("#ListShootsUsingSeed", func() {
		BeforeEach(func() {
			for _, shoot := range shoots {
				Expect(coreInformerFactory.Core().V1beta1().Shoots().Informer().GetStore().Add(shoot)).To(Succeed())
			}
		})

		It("should consider spec.seedName", func() {
			Expect(ListShootsUsingSeed("seed1", shootLister)).To(ConsistOf(
				PointTo(HaveField("ObjectMeta.Name", "shoot1")),
				PointTo(HaveField("ObjectMeta.Name", "shoot2")),
			))
		})

		It("should consider status.seedName", func() {
			Expect(ListShootsUsingSeed("seed2", shootLister)).To(ConsistOf(
				PointTo(HaveField("ObjectMeta.Name", "shoot2")),
			))
		})

		It("should return empty list if there is no referencing shoot", func() {
			Expect(ListShootsUsingSeed("non-existing", shootLister)).To(BeEmpty())
		})
	})

	DescribeTable("#IsSeedUsedByAnyShoot",
		func(seedName string, expected bool) {
			for _, shoot := range shoots {
				Expect(coreInformerFactory.Core().V1beta1().Shoots().Informer().GetStore().Add(shoot)).To(Succeed())
			}

			Expect(IsSeedUsedByAnyShoot(seedName, shootLister)).To(Equal(expected))
		},
		Entry("is used by shoot", "seed1", true),
		Entry("is used by shoot in migration", "seed2", true),
		Entry("is unused", "seed3", false),
	)

	Describe("#IsSeedUsedByShoot", func() {
		It("should consider spec.seedName", func() {
			Expect(IsSeedUsedByShoot("seed1", shoot1)).To(BeTrue())
			Expect(IsSeedUsedByShoot("seed1", shoot2)).To(BeTrue())
		})

		It("should consider status.seedName", func() {
			Expect(IsSeedUsedByShoot("seed2", shoot1)).To(BeFalse())
			Expect(IsSeedUsedByShoot("seed2", shoot2)).To(BeTrue())
		})

		It("should handle unscheduled shoots", func() {
			Expect(IsSeedUsedByShoot("foo", shoot3)).To(BeFalse())
		})
	})

	Describe("#NewAttributesWithName", func() {
		It("should return admission.Attributes with the given name", func() {
			name := "name"
			attrs := admission.NewAttributesRecord(shoot1, nil, core.Kind("Shoot").WithVersion("version"), shoot1.Namespace, "", core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

			newAttrs := NewAttributesWithName(attrs, name)

			Expect(newAttrs.GetName()).To(Equal(name))
		})
	})

	Describe("#ValidateZoneRemovalFromSeeds", func() {
		var (
			seedName = "foo"
			kind     = "foo"

			oldSeedSpec, newSeedSpec *core.SeedSpec
			shoot                    *gardencorev1beta1.Shoot
		)

		BeforeEach(func() {
			oldSeedSpec = &core.SeedSpec{
				Provider: core.SeedProvider{
					Zones: []string{"1", "2"},
				},
			}
			newSeedSpec = oldSeedSpec.DeepCopy()

			shoot = &gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					SeedName: &seedName,
				},
				Status: gardencorev1beta1.ShootStatus{
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
			Expect(coreInformerFactory.Core().V1beta1().Shoots().Informer().GetStore().Add(shoot)).To(Succeed())

			Expect(ValidateZoneRemovalFromSeeds(oldSeedSpec, newSeedSpec, seedName, shootLister, kind)).To(Succeed())
		})

		It("should do nothing because zone was removed and no shoots exist", func() {
			newSeedSpec.Provider.Zones = []string{"2"}

			Expect(ValidateZoneRemovalFromSeeds(oldSeedSpec, newSeedSpec, seedName, shootLister, kind)).To(Succeed())
		})

		It("should return an error because zone was removed even though shoots exist", func() {
			newSeedSpec.Provider.Zones = []string{"2"}
			Expect(coreInformerFactory.Core().V1beta1().Shoots().Informer().GetStore().Add(shoot)).To(Succeed())

			Expect(ValidateZoneRemovalFromSeeds(oldSeedSpec, newSeedSpec, seedName, shootLister, kind)).To(MatchError(ContainSubstring("cannot remove zones")))
		})
	})

	Describe("#ValidateSeedNetworksUpdateWithShoots", func() {
		var (
			seedName = "foo"
			kind     = "foo"

			oldSeedSpec, newSeedSpec *core.SeedSpec
			shoot                    *gardencorev1beta1.Shoot
		)

		BeforeEach(func() {
			oldSeedSpec = &core.SeedSpec{
				Networks: core.SeedNetworks{
					Nodes:    ptr.To("10.0.0.0/16"),
					Pods:     "10.1.0.0/16",
					Services: "10.2.0.0/16",
					VPN:      ptr.To("10.3.0.0/24"),
				},
			}
			newSeedSpec = oldSeedSpec.DeepCopy()

			shoot = &gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "garden-foo",
					Name:      "bar",
				},
				Spec: gardencorev1beta1.ShootSpec{
					SeedName: &seedName,
					Networking: &gardencorev1beta1.Networking{
						Nodes:    ptr.To("10.4.0.0/16"),
						Pods:     ptr.To("10.5.0.0/16"),
						Services: ptr.To("10.6.0.0/16"),
					},
				},
				Status: gardencorev1beta1.ShootStatus{
					SeedName: &seedName,
				},
			}

			Expect(coreInformerFactory.Core().V1beta1().Shoots().Informer().GetStore().Add(shoot)).To(Succeed())
		})

		It("should do nothing because networks were not changed", func() {
			Expect(ValidateSeedNetworksUpdateWithShoots(oldSeedSpec, newSeedSpec, seedName, shootLister, kind)).To(Succeed())
		})

		It("should allow VPN network update if there are no overlapping shoots", func() {
			newSeedSpec.Networks.VPN = ptr.To("100.0.0.0/24")

			Expect(ValidateSeedNetworksUpdateWithShoots(oldSeedSpec, newSeedSpec, seedName, shootLister, kind)).To(Succeed())
		})

		It("should forbid VPN network update if there are overlapping shoots", func() {
			newSeedSpec.Networks.VPN = ptr.To("10.4.0.0/24")

			Expect(ValidateSeedNetworksUpdateWithShoots(oldSeedSpec, newSeedSpec, seedName, shootLister, kind)).To(MatchError(And(ContainSubstring("overlap with Shoot"), ContainSubstring("garden-foo/bar"))))
		})
	})
})
