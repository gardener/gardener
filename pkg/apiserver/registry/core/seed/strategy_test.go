// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package seed_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
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

			Context("#SyncDNSProviderCredentials", func() {
				It("should bump generation when seed.spec.dns.provider.secretRef is synced with seed.spec.dns.provider.credentialsRef", func() {
					newSeed.Spec.DNS.Provider = &core.SeedDNSProvider{
						SecretRef: corev1.SecretReference{
							Namespace: "namespace",
							Name:      "name",
						},
					}
					oldSeed = newSeed.DeepCopy()

					strategy.PrepareForUpdate(ctx, newSeed, oldSeed)
					Expect(newSeed.Generation).To(Equal(oldSeed.Generation + 1))
				})

				It("should not bump generation when seed.spec.dns.provider.secretRef is already synced with seed.spec.dns.provider.credentialsRef", func() {
					oldSeed.Spec.DNS.Provider = &core.SeedDNSProvider{
						SecretRef: corev1.SecretReference{
							Namespace: "namespace",
							Name:      "name",
						},
						CredentialsRef: &corev1.ObjectReference{
							APIVersion: "v1",
							Kind:       "Secret",
							Namespace:  "namespace",
							Name:       "name",
						},
					}
					newSeed.Spec.DNS.Provider = &core.SeedDNSProvider{
						SecretRef: corev1.SecretReference{
							Namespace: "namespace",
							Name:      "name",
						},
					}

					strategy.PrepareForUpdate(ctx, newSeed, oldSeed)
					Expect(newSeed.Generation).To(Equal(oldSeed.Generation))
				})

				It("should bump generation when seed.spec.dns.provider.credentialsRef is synced with seed.spec.dns.provider.secretRef", func() {
					newSeed.Spec.DNS.Provider = &core.SeedDNSProvider{
						CredentialsRef: &corev1.ObjectReference{
							APIVersion: "v1",
							Kind:       "Secret",
							Namespace:  "namespace",
							Name:       "name",
						},
					}
					oldSeed = newSeed.DeepCopy()

					strategy.PrepareForUpdate(ctx, newSeed, oldSeed)
					Expect(newSeed.Generation).To(Equal(oldSeed.Generation + 1))
				})

				It("should not bump generation when seed.spec.dns.provider.credentialsRef is already synced with seed.spec.dns.provider.secretRef", func() {
					oldSeed.Spec.DNS.Provider = &core.SeedDNSProvider{
						SecretRef: corev1.SecretReference{
							Namespace: "namespace",
							Name:      "name",
						},
						CredentialsRef: &corev1.ObjectReference{
							APIVersion: "v1",
							Kind:       "Secret",
							Namespace:  "namespace",
							Name:       "name",
						},
					}
					newSeed.Spec.DNS.Provider = &core.SeedDNSProvider{
						CredentialsRef: &corev1.ObjectReference{
							APIVersion: "v1",
							Kind:       "Secret",
							Namespace:  "namespace",
							Name:       "name",
						},
					}

					strategy.PrepareForUpdate(ctx, newSeed, oldSeed)
					Expect(newSeed.Generation).To(Equal(oldSeed.Generation))
				})
			})
		})
	})

	Describe("#SyncDNSProviderCredentials", func() {
		var seed *core.Seed

		BeforeEach(func() {
			seed = &core.Seed{}
		})

		It("should sync dns.provider.secretRef with dns.provider.credentialsRef", func() {
			seed.Spec.DNS.Provider = &core.SeedDNSProvider{
				SecretRef: corev1.SecretReference{
					Namespace: "namespace",
					Name:      "name",
				},
			}

			Expect(seed.Spec.DNS.Provider.CredentialsRef).To(BeNil())

			SyncDNSProviderCredentials(seed)

			Expect(seed.Spec.DNS.Provider.CredentialsRef).ToNot(BeNil())
			Expect(seed.Spec.DNS.Provider.CredentialsRef.APIVersion).To(Equal("v1"))
			Expect(seed.Spec.DNS.Provider.CredentialsRef.Kind).To(Equal("Secret"))
			Expect(seed.Spec.DNS.Provider.CredentialsRef.Namespace).To(Equal("namespace"))
			Expect(seed.Spec.DNS.Provider.CredentialsRef.Name).To(Equal("name"))
		})

		It("should sync dns.provider.credentialsRef referring secret with dns.provider.secretRef", func() {
			seed.Spec.DNS.Provider = &core.SeedDNSProvider{
				CredentialsRef: &corev1.ObjectReference{
					APIVersion: "v1",
					Kind:       "Secret",
					Namespace:  "namespace",
					Name:       "name",
				},
			}

			Expect(seed.Spec.DNS.Provider.SecretRef.Namespace).To(BeEmpty())
			Expect(seed.Spec.DNS.Provider.SecretRef.Name).To(BeEmpty())

			SyncDNSProviderCredentials(seed)

			Expect(seed.Spec.DNS.Provider.SecretRef.Namespace).To(Equal("namespace"))
			Expect(seed.Spec.DNS.Provider.SecretRef.Name).To(Equal("name"))
		})

		It("should not sync dns.provider.credentialsRef referring workloadidentity with dns.provider.secretRef", func() {
			seed.Spec.DNS.Provider = &core.SeedDNSProvider{
				CredentialsRef: &corev1.ObjectReference{
					APIVersion: "security.gardener.cloud/v1alpha1",
					Kind:       "WorkloadIdentity",
					Namespace:  "namespace",
					Name:       "name",
				},
			}

			Expect(seed.Spec.DNS.Provider.SecretRef.Namespace).To(BeEmpty())
			Expect(seed.Spec.DNS.Provider.SecretRef.Name).To(BeEmpty())

			SyncDNSProviderCredentials(seed)

			Expect(seed.Spec.DNS.Provider.SecretRef.Namespace).To(BeEmpty())
			Expect(seed.Spec.DNS.Provider.SecretRef.Name).To(BeEmpty())
		})

		It("should not sync empty dns.provider.credentialsRef with dns.provider.secretRef", func() {
			seed.Spec.DNS.Provider = &core.SeedDNSProvider{
				CredentialsRef: nil,
				SecretRef:      corev1.SecretReference{},
			}

			SyncDNSProviderCredentials(seed)

			Expect(seed.Spec.DNS.Provider.SecretRef.Namespace).To(BeEmpty())
			Expect(seed.Spec.DNS.Provider.SecretRef.Name).To(BeEmpty())
			Expect(seed.Spec.DNS.Provider.CredentialsRef).To(BeNil())
		})

		It("should not sync dns.provider.credentialsRef with dns.provider.secretRef when they refer different resources", func() {
			seed.Spec.DNS.Provider = &core.SeedDNSProvider{
				CredentialsRef: &corev1.ObjectReference{
					APIVersion: "security.gardener.cloud/v1alpha1",
					Kind:       "WorkloadIdentity",
					Namespace:  "namespace",
					Name:       "name",
				},
				SecretRef: corev1.SecretReference{
					Namespace: "namespace",
					Name:      "name",
				},
			}

			SyncDNSProviderCredentials(seed)

			Expect(seed.Spec.DNS.Provider.SecretRef.Namespace).To(Equal("namespace"))
			Expect(seed.Spec.DNS.Provider.SecretRef.Name).To(Equal("name"))

			Expect(seed.Spec.DNS.Provider.CredentialsRef).ToNot(BeNil())
			Expect(seed.Spec.DNS.Provider.CredentialsRef.APIVersion).To(Equal("security.gardener.cloud/v1alpha1"))
			Expect(seed.Spec.DNS.Provider.CredentialsRef.Kind).To(Equal("WorkloadIdentity"))
			Expect(seed.Spec.DNS.Provider.CredentialsRef.Namespace).To(Equal("namespace"))
			Expect(seed.Spec.DNS.Provider.CredentialsRef.Name).To(Equal("name"))
		})

		It("Should not sync anything when backup is nil", func() {
			seed.Spec.DNS.Provider = nil

			SyncDNSProviderCredentials(seed)

			Expect(seed.Spec.DNS.Provider).To(BeNil())
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
