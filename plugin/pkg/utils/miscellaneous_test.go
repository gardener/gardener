// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package utils_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/admission"
	kubeinformers "k8s.io/client-go/informers"
	kubecorev1listers "k8s.io/client-go/listers/core/v1"

	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	gardencorev1beta1listers "github.com/gardener/gardener/pkg/client/core/listers/core/v1beta1"
	"github.com/gardener/gardener/pkg/features"
	"github.com/gardener/gardener/pkg/utils/test"
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
				SeedName: new("seed1"),
			},
		}

		shoot2 = gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "shoot2",
				Namespace: "garden-pr1",
			},
			Spec: gardencorev1beta1.ShootSpec{
				SeedName: new("seed1"),
			},
			Status: gardencorev1beta1.ShootStatus{
				SeedName: new("seed2"),
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

	Describe("#ValidateInternalDomainChangeForSeed", func() {
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
				DNS: core.SeedDNS{
					Internal: &core.SeedDNSProviderConfig{
						Domain: "foo.internal",
					},
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

		It("should do nothing if internal domain is unchanged", func() {
			Expect(ValidateInternalDomainChangeForSeed(oldSeedSpec, newSeedSpec, seedName, shootLister, kind)).To(Succeed())
		})

		It("should do nothing if old or new internal domain is nil", func() {
			oldSeedSpec.DNS.Internal = nil
			Expect(ValidateInternalDomainChangeForSeed(oldSeedSpec, newSeedSpec, seedName, shootLister, kind)).To(Succeed())

			oldSeedSpec.DNS.Internal = &core.SeedDNSProviderConfig{Domain: "foo.internal"}
			newSeedSpec.DNS.Internal = nil
			Expect(ValidateInternalDomainChangeForSeed(oldSeedSpec, newSeedSpec, seedName, shootLister, kind)).To(Succeed())
		})

		It("should do nothing if internal domain changed but no shoots exist", func() {
			newSeedSpec.DNS.Internal.Domain = "bar.internal"
			Expect(ValidateInternalDomainChangeForSeed(oldSeedSpec, newSeedSpec, seedName, shootLister, kind)).To(Succeed())
		})

		It("should return error if internal domain changed and shoots exist for this seed", func() {
			newSeedSpec.DNS.Internal.Domain = "bar.internal"
			Expect(coreInformerFactory.Core().V1beta1().Shoots().Informer().GetStore().Add(shoot)).To(Succeed())
			err := ValidateInternalDomainChangeForSeed(oldSeedSpec, newSeedSpec, seedName, shootLister, kind)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("cannot change internal domain"))
		})

		It("should do nothing if internal domain changed but shoots exist for other seeds", func() {
			newSeedSpec.DNS.Internal.Domain = "bar.internal"
			otherSeed := "other-seed"
			shoot.Spec.SeedName = &otherSeed
			shoot.Status.SeedName = &otherSeed
			Expect(coreInformerFactory.Core().V1beta1().Shoots().Informer().GetStore().Add(shoot)).To(Succeed())
			Expect(ValidateInternalDomainChangeForSeed(oldSeedSpec, newSeedSpec, seedName, shootLister, kind)).To(Succeed())
		})

		Context("with the MutableShootDomains feature gate", func() {
			BeforeEach(func() {
				DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.MutableShootDomains, true))

				oldSeedSpec.DNS.InternalDomains = []core.SeedDNSProviderConfig{{Domain: "foo.internal"}, {Domain: "former.internal"}}
				newSeedSpec = oldSeedSpec.DeepCopy()
				shoot.Status.AdvertisedAddresses = []gardencorev1beta1.ShootAdvertisedAddress{
					{Name: v1beta1constants.AdvertisedAddressExternal, URL: "https://api.shoot.project.example.com"},
					{Name: v1beta1constants.AdvertisedAddressInternal, URL: "https://api.shoot.project.internal.foo.internal"},
				}
				Expect(coreInformerFactory.Core().V1beta1().Shoots().Informer().GetStore().Add(shoot)).To(Succeed())
			})

			It("should accept a new internal domain while the former one is kept", func() {
				newSeedSpec.DNS.Internal.Domain = "bar.internal"
				newSeedSpec.DNS.InternalDomains = []core.SeedDNSProviderConfig{{Domain: "bar.internal"}, {Domain: "foo.internal"}, {Domain: "former.internal"}}

				Expect(ValidateInternalDomainChangeForSeed(oldSeedSpec, newSeedSpec, seedName, shootLister, kind)).To(Succeed())
			})

			It("should accept the removal of an internal domain that no shoot uses", func() {
				newSeedSpec.DNS.InternalDomains = []core.SeedDNSProviderConfig{{Domain: "foo.internal"}}

				Expect(ValidateInternalDomainChangeForSeed(oldSeedSpec, newSeedSpec, seedName, shootLister, kind)).To(Succeed())
			})

			It("should refuse the removal of an internal domain that a shoot uses", func() {
				newSeedSpec.DNS.Internal.Domain = "bar.internal"
				newSeedSpec.DNS.InternalDomains = []core.SeedDNSProviderConfig{{Domain: "bar.internal"}, {Domain: "former.internal"}}

				err := ValidateInternalDomainChangeForSeed(oldSeedSpec, newSeedSpec, seedName, shootLister, kind)
				Expect(err).To(MatchError(ContainSubstring("cannot remove internal domains [foo.internal]")))
			})

			It("should refuse the removal of an internal domain that a shoot uses as prior internal domain", func() {
				shoot.Status.AdvertisedAddresses = []gardencorev1beta1.ShootAdvertisedAddress{
					{Name: v1beta1constants.AdvertisedAddressInternal, URL: "https://api.shoot.project.internal.foo.internal"},
					{Name: v1beta1constants.AdvertisedAddressPriorInternal, URL: "https://api.shoot.project.internal.former.internal"},
				}
				Expect(coreInformerFactory.Core().V1beta1().Shoots().Informer().GetStore().Update(shoot)).To(Succeed())
				newSeedSpec.DNS.InternalDomains = []core.SeedDNSProviderConfig{{Domain: "foo.internal"}}

				err := ValidateInternalDomainChangeForSeed(oldSeedSpec, newSeedSpec, seedName, shootLister, kind)
				Expect(err).To(MatchError(ContainSubstring("cannot remove internal domains [former.internal]")))
			})

			It("should refuse the removal of all internal domains if a shoot uses one of them", func() {
				newSeedSpec.DNS.Internal = nil
				newSeedSpec.DNS.InternalDomains = nil

				err := ValidateInternalDomainChangeForSeed(oldSeedSpec, newSeedSpec, seedName, shootLister, kind)
				Expect(err).To(MatchError(ContainSubstring("cannot remove internal domains [foo.internal]")))
			})

			It("should consider the internal domain if the internal domains are not set", func() {
				oldSeedSpec.DNS.InternalDomains = nil
				newSeedSpec.DNS.Internal.Domain = "bar.internal"
				newSeedSpec.DNS.InternalDomains = nil

				err := ValidateInternalDomainChangeForSeed(oldSeedSpec, newSeedSpec, seedName, shootLister, kind)
				Expect(err).To(MatchError(ContainSubstring("cannot remove internal domains [foo.internal]")))
			})

			It("should ignore the external address of a shoot", func() {
				shoot.Status.AdvertisedAddresses = []gardencorev1beta1.ShootAdvertisedAddress{
					{Name: v1beta1constants.AdvertisedAddressExternal, URL: "https://api.shoot.project.former.internal"},
				}
				Expect(coreInformerFactory.Core().V1beta1().Shoots().Informer().GetStore().Update(shoot)).To(Succeed())
				newSeedSpec.DNS.InternalDomains = []core.SeedDNSProviderConfig{{Domain: "foo.internal"}}

				Expect(ValidateInternalDomainChangeForSeed(oldSeedSpec, newSeedSpec, seedName, shootLister, kind)).To(Succeed())
			})

			It("should refuse to replace and remove the first internal domain in one update while shoots are on the seed", func() {
				shoot.Status.AdvertisedAddresses = nil
				Expect(coreInformerFactory.Core().V1beta1().Shoots().Informer().GetStore().Update(shoot)).To(Succeed())
				newSeedSpec.DNS.Internal.Domain = "former.internal"
				newSeedSpec.DNS.InternalDomains = []core.SeedDNSProviderConfig{{Domain: "former.internal"}}

				err := ValidateInternalDomainChangeForSeed(oldSeedSpec, newSeedSpec, seedName, shootLister, kind)
				Expect(err).To(MatchError(ContainSubstring(`cannot remove the first internal domain "foo.internal"`)))
			})

			It("should accept the removal of a former first internal domain that no shoot uses", func() {
				oldSeedSpec.DNS.Internal.Domain = "former.internal"
				oldSeedSpec.DNS.InternalDomains = []core.SeedDNSProviderConfig{{Domain: "former.internal"}, {Domain: "foo.internal"}}
				shoot.Status.AdvertisedAddresses = []gardencorev1beta1.ShootAdvertisedAddress{
					{Name: v1beta1constants.AdvertisedAddressInternal, URL: "https://api.shoot.project.internal.former.internal"},
				}
				Expect(coreInformerFactory.Core().V1beta1().Shoots().Informer().GetStore().Update(shoot)).To(Succeed())
				newSeedSpec.DNS.Internal.Domain = "former.internal"
				newSeedSpec.DNS.InternalDomains = []core.SeedDNSProviderConfig{{Domain: "former.internal"}}

				Expect(ValidateInternalDomainChangeForSeed(oldSeedSpec, newSeedSpec, seedName, shootLister, kind)).To(Succeed())
			})

			It("should accept the removal of an internal domain that only shoots of other seeds use", func() {
				shoot.Spec.SeedName = new("other-seed")
				shoot.Status.SeedName = new("other-seed")
				Expect(coreInformerFactory.Core().V1beta1().Shoots().Informer().GetStore().Update(shoot)).To(Succeed())
				newSeedSpec.DNS.Internal.Domain = "former.internal"
				newSeedSpec.DNS.InternalDomains = []core.SeedDNSProviderConfig{{Domain: "former.internal"}}

				Expect(ValidateInternalDomainChangeForSeed(oldSeedSpec, newSeedSpec, seedName, shootLister, kind)).To(Succeed())
			})
		})
	})

	Describe("#ValidateInternalDomainDisableForSeed", func() {
		var (
			seedName = "foo"
			kind     = "Seed"

			coreInformerFactory gardencoreinformers.SharedInformerFactory
			shootLister         gardencorev1beta1listers.ShootLister

			oldSeedSpec, newSeedSpec *core.SeedSpec
			shoot                    *gardencorev1beta1.Shoot
		)

		BeforeEach(func() {
			DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.MutableShootDomains, true))

			coreInformerFactory = gardencoreinformers.NewSharedInformerFactory(nil, 0)
			shootLister = coreInformerFactory.Core().V1beta1().Shoots().Lister()

			oldSeedSpec = &core.SeedSpec{DNS: core.SeedDNS{InternalDomainEnabled: new(true)}}
			newSeedSpec = &core.SeedSpec{DNS: core.SeedDNS{InternalDomainEnabled: new(false)}}

			shoot = &gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{Namespace: "garden-project", Name: "shoot"},
				Spec:       gardencorev1beta1.ShootSpec{SeedName: &seedName},
			}
			Expect(coreInformerFactory.Core().V1beta1().Shoots().Informer().GetStore().Add(shoot)).To(Succeed())
		})

		It("should refuse the disable if a shoot on the seed has no external domain", func() {
			err := ValidateInternalDomainDisableForSeed(oldSeedSpec, newSeedSpec, seedName, shootLister, kind)
			Expect(err).To(MatchError(ContainSubstring(`cannot disable the internal domain of Seed "foo" as the shoots [garden-project/shoot] have no managed external domain`)))
		})

		It("should refuse the disable if a shoot on the seed has an unmanaged external domain", func() {
			shoot.Spec.DNS = &gardencorev1beta1.DNS{Domain: new("shoot.example.com"), Providers: []gardencorev1beta1.DNSProvider{{Type: new("unmanaged")}}}

			Expect(ValidateInternalDomainDisableForSeed(oldSeedSpec, newSeedSpec, seedName, shootLister, kind)).To(MatchError(ContainSubstring("cannot disable the internal domain")))
		})

		It("should refuse the disable if the old seed spec does not set the field", func() {
			oldSeedSpec.DNS.InternalDomainEnabled = nil

			Expect(ValidateInternalDomainDisableForSeed(oldSeedSpec, newSeedSpec, seedName, shootLister, kind)).To(MatchError(ContainSubstring("cannot disable the internal domain")))
		})

		It("should accept the disable if each shoot on the seed has a managed external domain", func() {
			shoot.Spec.DNS = &gardencorev1beta1.DNS{Domain: new("shoot.example.com"), Providers: []gardencorev1beta1.DNSProvider{{Type: new("some-provider"), Primary: new(true)}}}

			Expect(ValidateInternalDomainDisableForSeed(oldSeedSpec, newSeedSpec, seedName, shootLister, kind)).To(Succeed())
		})

		It("should accept the disable if the shoot without external domain is on another seed", func() {
			shoot.Spec.SeedName = new("other-seed")

			Expect(ValidateInternalDomainDisableForSeed(oldSeedSpec, newSeedSpec, seedName, shootLister, kind)).To(Succeed())
		})

		It("should accept an update if the internal domain stays disabled", func() {
			oldSeedSpec.DNS.InternalDomainEnabled = new(false)

			Expect(ValidateInternalDomainDisableForSeed(oldSeedSpec, newSeedSpec, seedName, shootLister, kind)).To(Succeed())
		})

		It("should accept the disable if the feature gate is off", func() {
			DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.MutableShootDomains, false))

			Expect(ValidateInternalDomainDisableForSeed(oldSeedSpec, newSeedSpec, seedName, shootLister, kind)).To(Succeed())
		})
	})

	Describe("#ValidateDefaultDomainsChangeForSeed", func() {
		var (
			seedName = "foo"
			kind     = "foo"

			coreInformerFactory gardencoreinformers.SharedInformerFactory
			kubeInformerFactory kubeinformers.SharedInformerFactory
			shootLister         gardencorev1beta1listers.ShootLister
			secretLister        kubecorev1listers.SecretLister

			oldSeedSpec, newSeedSpec *core.SeedSpec
			shoot                    *gardencorev1beta1.Shoot
		)

		BeforeEach(func() {
			coreInformerFactory = gardencoreinformers.NewSharedInformerFactory(nil, 0)
			shootLister = coreInformerFactory.Core().V1beta1().Shoots().Lister()

			kubeInformerFactory = kubeinformers.NewSharedInformerFactory(nil, 0)
			secretLister = kubeInformerFactory.Core().V1().Secrets().Lister()

			oldSeedSpec = &core.SeedSpec{
				DNS: core.SeedDNS{
					Defaults: []core.SeedDNSProviderConfig{
						{Domain: "example.com"},
						{Domain: "test.org"},
					},
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

		It("should do nothing if default domains are unchanged", func() {
			Expect(ValidateDefaultDomainsChangeForSeed(oldSeedSpec, newSeedSpec, seedName, shootLister, secretLister, kind)).To(Succeed())
		})

		It("should do nothing if domains are reordered but same domains exist", func() {
			newSeedSpec.DNS.Defaults = []core.SeedDNSProviderConfig{
				{Domain: "test.org"},
				{Domain: "example.com"},
			}
			Expect(ValidateDefaultDomainsChangeForSeed(oldSeedSpec, newSeedSpec, seedName, shootLister, secretLister, kind)).To(Succeed())
		})

		It("should do nothing if default domains are empty in both specs", func() {
			oldSeedSpec.DNS.Defaults = nil
			newSeedSpec.DNS.Defaults = nil
			Expect(ValidateDefaultDomainsChangeForSeed(oldSeedSpec, newSeedSpec, seedName, shootLister, secretLister, kind)).To(Succeed())
		})

		It("should do nothing if default domains changed but no shoots exist", func() {
			newSeedSpec.DNS.Defaults = []core.SeedDNSProviderConfig{
				{Domain: "new-domain.com"},
			}
			Expect(ValidateDefaultDomainsChangeForSeed(oldSeedSpec, newSeedSpec, seedName, shootLister, secretLister, kind)).To(Succeed())
		})

		It("should do nothing if default domains added (even with shoots on the seed)", func() {
			newSeedSpec.DNS.Defaults = []core.SeedDNSProviderConfig{
				{Domain: "example.com"},
				{Domain: "test.org"},
				{Domain: "new-domain.com"},
			}
			Expect(coreInformerFactory.Core().V1beta1().Shoots().Informer().GetStore().Add(shoot)).To(Succeed())
			Expect(ValidateDefaultDomainsChangeForSeed(oldSeedSpec, newSeedSpec, seedName, shootLister, secretLister, kind)).To(Succeed())
		})

		It("should do nothing if default domains removed but no shoots are using them", func() {
			newSeedSpec.DNS.Defaults = []core.SeedDNSProviderConfig{
				{Domain: "example.com"},
			}
			shoot.Spec.DNS = &gardencorev1beta1.DNS{Domain: new("my-shoot.my-project.other-domain.com")}
			Expect(coreInformerFactory.Core().V1beta1().Shoots().Informer().GetStore().Add(shoot)).To(Succeed())
			Expect(ValidateDefaultDomainsChangeForSeed(oldSeedSpec, newSeedSpec, seedName, shootLister, secretLister, kind)).To(Succeed())
		})

		It("should return error if default domains removed and shoots are using them", func() {
			newSeedSpec.DNS.Defaults = []core.SeedDNSProviderConfig{
				{Domain: "test.org"},
			}
			shoot.Spec.DNS = &gardencorev1beta1.DNS{Domain: new("my-shoot.my-project.example.com")}
			Expect(coreInformerFactory.Core().V1beta1().Shoots().Informer().GetStore().Add(shoot)).To(Succeed())
			err := ValidateDefaultDomainsChangeForSeed(oldSeedSpec, newSeedSpec, seedName, shootLister, secretLister, kind)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(`cannot remove default domains [example.com] from foo "foo" as they are still being used by shoots`))
		})

		It("should return error if multiple default domains removed and shoots are using them", func() {
			newSeedSpec.DNS.Defaults = []core.SeedDNSProviderConfig{}
			shoot.Spec.DNS = &gardencorev1beta1.DNS{Domain: new("my-shoot.my-project.test.org")}
			Expect(coreInformerFactory.Core().V1beta1().Shoots().Informer().GetStore().Add(shoot)).To(Succeed())
			err := ValidateDefaultDomainsChangeForSeed(oldSeedSpec, newSeedSpec, seedName, shootLister, secretLister, kind)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("cannot remove default domains"))
			Expect(err.Error()).To(ContainSubstring("test.org"))
		})

		It("should do nothing if default domains changed but shoots exist for other seeds", func() {
			newSeedSpec.DNS.Defaults = []core.SeedDNSProviderConfig{}
			otherSeed := "other-seed"
			shoot.Spec.SeedName = &otherSeed
			shoot.Status.SeedName = &otherSeed
			shoot.Spec.DNS = &gardencorev1beta1.DNS{Domain: new("my-shoot.my-project.example.com")}
			Expect(coreInformerFactory.Core().V1beta1().Shoots().Informer().GetStore().Add(shoot)).To(Succeed())
			Expect(ValidateDefaultDomainsChangeForSeed(oldSeedSpec, newSeedSpec, seedName, shootLister, secretLister, kind)).To(Succeed())
		})

		Context("when transitioning from global defaults to explicit defaults", func() {
			var globalDefaultSecret *corev1.Secret

			BeforeEach(func() {
				oldSeedSpec.DNS.Defaults = nil

				globalDefaultSecret = &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "default-domain-secret",
						Namespace: v1beta1constants.GardenNamespace,
						Labels: map[string]string{
							v1beta1constants.GardenRole: v1beta1constants.GardenRoleDefaultDomain,
						},
						Annotations: map[string]string{
							"dns.gardener.cloud/domain":   "global-default.com",
							"dns.gardener.cloud/provider": "aws-route53",
						},
					},
				}
			})

			It("should succeed when explicit defaults cover all used global defaults", func() {
				newSeedSpec.DNS.Defaults = []core.SeedDNSProviderConfig{
					{Domain: "global-default.com"},
					{Domain: "additional.com"},
				}

				shoot.Spec.DNS = &gardencorev1beta1.DNS{Domain: new("my-shoot.my-project.global-default.com")}
				Expect(coreInformerFactory.Core().V1beta1().Shoots().Informer().GetStore().Add(shoot)).To(Succeed())
				Expect(kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(globalDefaultSecret)).To(Succeed())

				Expect(ValidateDefaultDomainsChangeForSeed(oldSeedSpec, newSeedSpec, seedName, shootLister, secretLister, kind)).To(Succeed())
			})

			It("should fail when explicit defaults do not cover used global defaults", func() {
				newSeedSpec.DNS.Defaults = []core.SeedDNSProviderConfig{
					{Domain: "different.com"},
				}

				shoot.Spec.DNS = &gardencorev1beta1.DNS{Domain: new("my-shoot.my-project.global-default.com")}
				Expect(coreInformerFactory.Core().V1beta1().Shoots().Informer().GetStore().Add(shoot)).To(Succeed())
				Expect(kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(globalDefaultSecret)).To(Succeed())

				err := ValidateDefaultDomainsChangeForSeed(oldSeedSpec, newSeedSpec, seedName, shootLister, secretLister, kind)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("cannot configure explicit default domains"))
				Expect(err.Error()).To(ContainSubstring("global-default.com"))
			})

			It("should succeed when no shoots use global defaults", func() {
				newSeedSpec.DNS.Defaults = []core.SeedDNSProviderConfig{
					{Domain: "different.com"},
				}

				shoot.Spec.DNS = &gardencorev1beta1.DNS{Domain: new("my-shoot.my-project.other-domain.com")}
				Expect(coreInformerFactory.Core().V1beta1().Shoots().Informer().GetStore().Add(shoot)).To(Succeed())
				Expect(kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(globalDefaultSecret)).To(Succeed())

				Expect(ValidateDefaultDomainsChangeForSeed(oldSeedSpec, newSeedSpec, seedName, shootLister, secretLister, kind)).To(Succeed())
			})
		})
	})
})
