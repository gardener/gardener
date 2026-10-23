// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package seed_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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
			newSeed.Status = core.SeedStatus{KubernetesVersion: new("1.2.3")}
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
		})

		Context("internal domains", func() {
			var domainA, domainB, domainC core.SeedDNSProviderConfig

			BeforeEach(func() {
				domainA = core.SeedDNSProviderConfig{Type: "provider-a", Domain: "internal.a.example.com", CredentialsRef: corev1.ObjectReference{Name: "secret-a"}}
				domainB = core.SeedDNSProviderConfig{Type: "provider-b", Domain: "internal.b.example.com", CredentialsRef: corev1.ObjectReference{Name: "secret-b"}}
				domainC = core.SeedDNSProviderConfig{Type: "provider-c", Domain: "internal.c.example.com", CredentialsRef: corev1.ObjectReference{Name: "secret-c"}}

				oldSeed.Spec.DNS = core.SeedDNS{Internal: &domainA, InternalDomains: []core.SeedDNSProviderConfig{domainA, domainB}}
				newSeed.Spec.DNS = *oldSeed.Spec.DNS.DeepCopy()
			})

			It("should replace the first entry of the internal domains if only the internal domain changed", func() {
				newSeed.Spec.DNS.Internal = domainC.DeepCopy()

				strategy.PrepareForUpdate(ctx, newSeed, oldSeed)

				Expect(newSeed.Spec.DNS.Internal).To(PointTo(Equal(domainC)))
				Expect(newSeed.Spec.DNS.InternalDomains).To(Equal([]core.SeedDNSProviderConfig{domainC, domainB}))
			})

			It("should set the internal domain to the first entry of the internal domains if only the internal domains changed", func() {
				newSeed.Spec.DNS.InternalDomains = []core.SeedDNSProviderConfig{domainC, domainA, domainB}

				strategy.PrepareForUpdate(ctx, newSeed, oldSeed)

				Expect(newSeed.Spec.DNS.Internal).To(PointTo(Equal(domainC)))
				Expect(newSeed.Spec.DNS.InternalDomains).To(Equal([]core.SeedDNSProviderConfig{domainC, domainA, domainB}))
			})

			It("should keep both fields if both changed to different values", func() {
				newSeed.Spec.DNS.Internal = domainC.DeepCopy()
				newSeed.Spec.DNS.InternalDomains = []core.SeedDNSProviderConfig{domainB, domainA}

				strategy.PrepareForUpdate(ctx, newSeed, oldSeed)

				Expect(newSeed.Spec.DNS.Internal).To(PointTo(Equal(domainC)))
				Expect(newSeed.Spec.DNS.InternalDomains).To(Equal([]core.SeedDNSProviderConfig{domainB, domainA}))
			})

			It("should keep both fields if both changed to the same value", func() {
				newSeed.Spec.DNS.Internal = domainC.DeepCopy()
				newSeed.Spec.DNS.InternalDomains = []core.SeedDNSProviderConfig{domainC, domainA}

				strategy.PrepareForUpdate(ctx, newSeed, oldSeed)

				Expect(newSeed.Spec.DNS.Internal).To(PointTo(Equal(domainC)))
				Expect(newSeed.Spec.DNS.InternalDomains).To(Equal([]core.SeedDNSProviderConfig{domainC, domainA}))
			})

			It("should keep both fields if the stored fields already differ", func() {
				oldSeed.Spec.DNS.Internal = domainC.DeepCopy()

				strategy.PrepareForUpdate(ctx, newSeed, oldSeed)

				Expect(newSeed.Spec.DNS.Internal).To(PointTo(Equal(domainA)))
				Expect(newSeed.Spec.DNS.InternalDomains).To(Equal([]core.SeedDNSProviderConfig{domainA, domainB}))
			})

			It("should not share data between the internal domain and the internal domains", func() {
				newSeed.Spec.DNS.Internal = domainC.DeepCopy()

				strategy.PrepareForUpdate(ctx, newSeed, oldSeed)
				newSeed.Spec.DNS.InternalDomains[0].Domain = "other.example.com"

				Expect(newSeed.Spec.DNS.Internal.Domain).To(Equal("internal.c.example.com"))
			})
		})
	})

	Describe("#Canonicalize", func() {
		var seed *core.Seed

		BeforeEach(func() {
			seed = &core.Seed{}
		})

		It("should add the labels for the seed provider and region", func() {
			seed.Spec = core.SeedSpec{Provider: core.SeedProvider{Type: "provider-type", Region: "provider-region"}}
			strategy.Canonicalize(seed)

			Expect(seed.Labels).To(And(
				HaveKeyWithValue("seed.gardener.cloud/provider", "provider-type"),
				HaveKeyWithValue("seed.gardener.cloud/region", "provider-region"),
			))
		})
	})
})
