// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package seed_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/pkg/apis/core"
	. "github.com/gardener/gardener/pkg/apiserver/registry/core/seed"
)

var _ = Describe("Strategy", func() {
	var (
		ctx      = context.TODO()
		strategy = Strategy{}
	)

	Describe("#PrepareForUpdate", func() {
		var oldSeed, newSeed *core.Seed

		BeforeEach(func() {
			oldSeed = &core.Seed{}
			newSeed = &core.Seed{}
		})

		It("should preserve the status", func() {
			newSeed.Status = core.SeedStatus{KubernetesVersion: ptr.To("1.2.3")}
			strategy.PrepareForUpdate(ctx, newSeed, oldSeed)
			Expect(newSeed.Status).To(Equal(oldSeed.Status))
		})

		Context("generation increment", func() {
			It("should not bump the generation if nothing changed", func() {
				strategy.PrepareForUpdate(ctx, newSeed, oldSeed)

				Expect(newSeed.Generation).To(Equal(oldSeed.Generation))
			})

			It("should bump the generation if the spec changed", func() {
				newSeed.Spec.Provider.Type = "foo"

				strategy.PrepareForUpdate(ctx, newSeed, oldSeed)

				Expect(newSeed.Generation).To(Equal(oldSeed.Generation + 1))
			})

			It("should bump the generation if the deletionTimestamp was set", func() {
				now := metav1.Now()
				newSeed.DeletionTimestamp = &now

				strategy.PrepareForUpdate(ctx, newSeed, oldSeed)

				Expect(newSeed.Generation).To(Equal(oldSeed.Generation + 1))
			})

			It("should not bump the generation if the deletionTimestamp was already set", func() {
				now := metav1.Now()
				oldSeed.DeletionTimestamp = &now
				newSeed.DeletionTimestamp = &now

				strategy.PrepareForUpdate(ctx, newSeed, oldSeed)

				Expect(newSeed.Generation).To(Equal(oldSeed.Generation))
			})

			It("should bump the generation if the operation annotation was set to renew-garden-access-secrets", func() {
				metav1.SetMetaDataAnnotation(&newSeed.ObjectMeta, "gardener.cloud/operation", "renew-garden-access-secrets")

				strategy.PrepareForUpdate(ctx, newSeed, oldSeed)

				Expect(newSeed.Generation).To(Equal(oldSeed.Generation + 1))
			})

			It("should bump the generation if the operation annotation was set to renew-kubeconfig", func() {
				metav1.SetMetaDataAnnotation(&newSeed.ObjectMeta, "gardener.cloud/operation", "renew-kubeconfig")

				strategy.PrepareForUpdate(ctx, newSeed, oldSeed)

				Expect(newSeed.Generation).To(Equal(oldSeed.Generation + 1))
			})

			It("should bump the generation if the operation annotation was set to renew-workload-identity-tokens", func() {
				metav1.SetMetaDataAnnotation(&newSeed.ObjectMeta, "gardener.cloud/operation", "renew-workload-identity-tokens")

				strategy.PrepareForUpdate(ctx, newSeed, oldSeed)

				Expect(newSeed.Generation).To(Equal(oldSeed.Generation + 1))
			})

			It("should bump the generation and remove the annotation if the operation annotation was set to reconcile", func() {
				metav1.SetMetaDataAnnotation(&newSeed.ObjectMeta, "gardener.cloud/operation", "reconcile")

				strategy.PrepareForUpdate(ctx, newSeed, oldSeed)

				Expect(newSeed.Generation).To(Equal(oldSeed.Generation + 1))
				Expect(newSeed.Annotations).NotTo(ContainElement("gardener.cloud/operation"))
			})

			It("should not bump the generation if the operation annotation didn't change", func() {
				metav1.SetMetaDataAnnotation(&oldSeed.ObjectMeta, "gardener.cloud/operation", "renew-garden-access-secrets")
				metav1.SetMetaDataAnnotation(&newSeed.ObjectMeta, "gardener.cloud/operation", "renew-garden-access-secrets")

				strategy.PrepareForUpdate(ctx, newSeed, oldSeed)

				Expect(newSeed.Generation).To(Equal(oldSeed.Generation))
			})

			Context("access restrictions", func() {
				BeforeEach(func() {
					newSeed = &core.Seed{}
					oldSeed = newSeed.DeepCopy()
				})

				It("should remove the access restriction when the label is dropped", func() {
					oldSeed.Labels = map[string]string{"seed.gardener.cloud/eu-access": "true"}
					oldSeed.Spec.AccessRestrictions = []core.AccessRestriction{{Name: "eu-access-only"}}
					newSeed.Spec.AccessRestrictions = []core.AccessRestriction{{Name: "eu-access-only"}}

					strategy.PrepareForUpdate(ctx, newSeed, oldSeed)

					Expect(newSeed.Spec.AccessRestrictions).To(BeEmpty())
					Expect(newSeed.Labels).To(BeEmpty())
				})

				It("should remove the label when the access restriction is dropped", func() {
					oldSeed.Labels = map[string]string{"seed.gardener.cloud/eu-access": "true"}
					oldSeed.Spec.AccessRestrictions = []core.AccessRestriction{{Name: "eu-access-only"}}
					newSeed.Labels = map[string]string{"seed.gardener.cloud/eu-access": "true"}

					strategy.PrepareForUpdate(ctx, newSeed, oldSeed)

					Expect(newSeed.Spec.AccessRestrictions).To(BeEmpty())
					Expect(newSeed.Labels).To(BeEmpty())
				})
			})
		})
	})

	Describe("#Canonicalize", func() {
		It("should add the access restriction if the legacy label is present", func() {
			seed := &core.Seed{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"seed.gardener.cloud/eu-access": "true"},
				},
			}

			strategy.Canonicalize(seed)

			Expect(seed.Spec.AccessRestrictions).To(HaveExactElements(core.AccessRestriction{Name: "eu-access-only"}))
		})

		It("should add the label if the access restriction is configured", func() {
			seed := &core.Seed{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"seed.gardener.cloud/eu-access": "true"},
				},
				Spec: core.SeedSpec{
					AccessRestrictions: []core.AccessRestriction{{Name: "eu-access-only"}},
				},
			}

			strategy.Canonicalize(seed)

			Expect(seed.Labels).To(HaveKeyWithValue("seed.gardener.cloud/eu-access", "true"))
		})

		It("should not add the access restriction or label", func() {
			seed := &core.Seed{}

			strategy.Canonicalize(seed)

			Expect(seed.Spec.AccessRestrictions).To(BeEmpty())
			Expect(seed.Labels).To(BeEmpty())
		})
	})
})
