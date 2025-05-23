// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package utils_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
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
		shoot1 = gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "shoot1",
				Namespace: "garden-pr1",
			},
			Spec: gardencorev1beta1.ShootSpec{
				SeedName: ptr.To("seed1"),
			},
		}

		shoot2 = gardencorev1beta1.Shoot{
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

		shoot3 = gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "shoot3",
				Namespace: "garden-pr1",
			},
			Spec: gardencorev1beta1.ShootSpec{
				SeedName: nil,
			},
		}
	)

	shoots := []*gardencorev1beta1.Shoot{
		&shoot1,
		&shoot2,
		&shoot3,
	}
	now := metav1.Now()

	coreShoot1 := core.Shoot{}
	err := gardencorev1beta1.Convert_v1beta1_Shoot_To_core_Shoot(&shoot1, &coreShoot1, nil)
	Expect(err).NotTo(HaveOccurred())
	coreShoot2 := core.Shoot{}
	err = gardencorev1beta1.Convert_v1beta1_Shoot_To_core_Shoot(&shoot2, &coreShoot2, nil)
	Expect(err).NotTo(HaveOccurred())
	coreShoot3 := core.Shoot{}
	err = gardencorev1beta1.Convert_v1beta1_Shoot_To_core_Shoot(&shoot3, &coreShoot3, nil)
	Expect(err).NotTo(HaveOccurred())

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
			shootLister         gardencorev1beta1listers.ShootLister

			oldSeedSpec, newSeedSpec *core.SeedSpec
			shoot                    *gardencorev1beta1.Shoot
		)

		BeforeEach(func() {
			coreInformerFactory = gardencoreinformers.NewSharedInformerFactory(nil, 0)
			shootLister = coreInformerFactory.Core().V1beta1().Shoots().Lister()

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
})
