// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package mutator_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	seedmanagementinformers "github.com/gardener/gardener/pkg/client/seedmanagement/informers/externalversions"
	. "github.com/gardener/gardener/plugin/pkg/seed/mutator"
)

var _ = Describe("mutator", func() {
	var (
		ctx = context.Background()

		coreInformerFactory           gardencoreinformers.SharedInformerFactory
		seedManagementInformerFactory seedmanagementinformers.SharedInformerFactory

		handler *MutateSeed
	)

	BeforeEach(func() {
		var err error
		handler, err = New()
		Expect(err).NotTo(HaveOccurred())

		coreInformerFactory = gardencoreinformers.NewSharedInformerFactory(nil, 0)
		seedManagementInformerFactory = seedmanagementinformers.NewSharedInformerFactory(nil, 0)
	})

	Describe("#Admit", func() {
		var (
			seed        *core.Seed
			shoot       *gardencorev1beta1.Shoot
			managedSeed *seedmanagementv1alpha1.ManagedSeed
		)

		BeforeEach(func() {
			seed = &core.Seed{ObjectMeta: metav1.ObjectMeta{Name: "the-seed"}}
			shoot = &gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{Name: "the-shoot", Namespace: "garden"},
				Spec:       gardencorev1beta1.ShootSpec{SeedName: ptr.To("parent-seed")},
			}
			managedSeed = &seedmanagementv1alpha1.ManagedSeed{
				ObjectMeta: metav1.ObjectMeta{Name: "the-seed", Namespace: "garden"},
				Spec:       seedmanagementv1alpha1.ManagedSeedSpec{Shoot: &seedmanagementv1alpha1.Shoot{Name: shoot.Name}},
			}

			handler.AssignReadyFunc(func() bool { return true })
			handler.SetCoreInformerFactory(coreInformerFactory)
			handler.SetSeedManagementInformerFactory(seedManagementInformerFactory)
		})

		Context("create", func() {
			It("should add the label for the seed name", func() {
				attrs := admission.NewAttributesRecord(seed, nil, core.Kind("Seed").WithVersion("version"), "", seed.Name, core.Resource("seeds").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)
				Expect(handler.Admit(ctx, attrs, nil)).To(Succeed())

				Expect(seed.Labels).To(HaveKeyWithValue("name.seed.gardener.cloud/the-seed", "true"))
			})

			It("should add the label for the parent seed name", func() {
				Expect(seedManagementInformerFactory.Seedmanagement().V1alpha1().ManagedSeeds().Informer().GetStore().Add(managedSeed)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Shoots().Informer().GetStore().Add(shoot)).To(Succeed())

				attrs := admission.NewAttributesRecord(seed, nil, core.Kind("Seed").WithVersion("version"), "", seed.Name, core.Resource("seeds").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)
				Expect(handler.Admit(ctx, attrs, nil)).To(Succeed())

				Expect(seed.Labels).To(And(
					HaveKeyWithValue("name.seed.gardener.cloud/the-seed", "true"),
					HaveKeyWithValue("name.seed.gardener.cloud/parent-seed", "true"),
				))
			})
		})

		Context("update", func() {
			It("should add the label for the seed name", func() {
				attrs := admission.NewAttributesRecord(seed, seed, core.Kind("Seed").WithVersion("version"), "", seed.Name, core.Resource("seeds").WithVersion("version"), "", admission.Update, &metav1.CreateOptions{}, false, nil)
				Expect(handler.Admit(ctx, attrs, nil)).To(Succeed())

				Expect(seed.Labels).To(HaveKeyWithValue("name.seed.gardener.cloud/the-seed", "true"))
			})

			It("should add the label for the parent seed name", func() {
				Expect(seedManagementInformerFactory.Seedmanagement().V1alpha1().ManagedSeeds().Informer().GetStore().Add(managedSeed)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Shoots().Informer().GetStore().Add(shoot)).To(Succeed())

				attrs := admission.NewAttributesRecord(seed, seed, core.Kind("Seed").WithVersion("version"), "", seed.Name, core.Resource("seeds").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
				Expect(handler.Admit(ctx, attrs, nil)).To(Succeed())

				Expect(seed.Labels).To(And(
					HaveKeyWithValue("name.seed.gardener.cloud/the-seed", "true"),
					HaveKeyWithValue("name.seed.gardener.cloud/parent-seed", "true"),
				))
			})

			It("should remove unneeded labels", func() {
				Expect(seedManagementInformerFactory.Seedmanagement().V1alpha1().ManagedSeeds().Informer().GetStore().Add(managedSeed)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Shoots().Informer().GetStore().Add(shoot)).To(Succeed())

				metav1.SetMetaDataLabel(&seed.ObjectMeta, "name.seed.gardener.cloud/foo", "true")

				attrs := admission.NewAttributesRecord(seed, seed, core.Kind("Seed").WithVersion("version"), "", seed.Name, core.Resource("seeds").WithVersion("version"), "", admission.Update, &metav1.CreateOptions{}, false, nil)
				Expect(handler.Admit(ctx, attrs, nil)).To(Succeed())

				Expect(seed.Labels).To(And(
					HaveKeyWithValue("name.seed.gardener.cloud/the-seed", "true"),
					HaveKeyWithValue("name.seed.gardener.cloud/parent-seed", "true"),
					Not(HaveKey("name.seed.gardener.cloud/foo")),
				))
			})
		})
	})

	Describe("#Register", func() {
		It("should register the plugin", func() {
			plugins := admission.NewPlugins()
			Register(plugins)

			Expect(plugins.Registered()).To(HaveExactElements("SeedMutator"))
		})
	})

	Describe("#New", func() {
		It("should handle only DELETE and Update operations", func() {
			Expect(handler.Handles(admission.Create)).To(BeTrue())
			Expect(handler.Handles(admission.Update)).To(BeTrue())
			Expect(handler.Handles(admission.Connect)).NotTo(BeTrue())
			Expect(handler.Handles(admission.Delete)).NotTo(BeTrue())
		})
	})

	Describe("#ValidateInitialization", func() {
		It("should return error if a lister is missing", func() {
			Expect(handler.ValidateInitialization()).To(MatchError(ContainSubstring("missing managed seed lister")))
		})

		It("should not return error if all listers are set", func() {
			handler.SetCoreInformerFactory(coreInformerFactory)
			handler.SetSeedManagementInformerFactory(seedManagementInformerFactory)

			Expect(handler.ValidateInitialization()).To(Succeed())
		})
	})
})
