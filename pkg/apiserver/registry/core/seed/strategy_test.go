// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
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

			It("should bump generation when backup.secretRef is synced with backup.credentialsRef", func() {
				newSeed.Spec.Backup = &core.Backup{
					SecretRef: corev1.SecretReference{
						Namespace: "namespace",
						Name:      "name",
					},
				}
				oldSeed = newSeed.DeepCopy()

				strategy.PrepareForUpdate(ctx, newSeed, oldSeed)
				Expect(newSeed.Generation).To(Equal(oldSeed.Generation + 1))
			})

			It("should not bump generation when backup.secretRef is already synced with backup.credentialsRef", func() {
				oldSeed.Spec.Backup = &core.Backup{
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
				newSeed.Spec.Backup = &core.Backup{
					SecretRef: corev1.SecretReference{
						Namespace: "namespace",
						Name:      "name",
					},
				}

				strategy.PrepareForUpdate(ctx, newSeed, oldSeed)
				Expect(newSeed.Generation).To(Equal(oldSeed.Generation))
			})

			It("should bump generation when backup.credentialsRef is synced with backup.secretRef", func() {
				newSeed.Spec.Backup = &core.Backup{
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

			It("should not bump generation when backup.credentialsRef is already synced with backup.secretRef", func() {
				oldSeed.Spec.Backup = &core.Backup{
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
				newSeed.Spec.Backup = &core.Backup{
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

		Describe("#syncBackupSecretRefAndCredentialsRef", func() {
			It("should sync backup.secretRef with backup.credentialsRef", func() {
				newSeed.Spec.Backup = &core.Backup{
					SecretRef: corev1.SecretReference{
						Namespace: "namespace",
						Name:      "name",
					},
				}

				Expect(newSeed.Spec.Backup.CredentialsRef).To(BeNil())

				strategy.PrepareForUpdate(ctx, newSeed, oldSeed)

				Expect(newSeed.Spec.Backup.CredentialsRef).ToNot(BeNil())
				Expect(newSeed.Spec.Backup.CredentialsRef.APIVersion).To(Equal("v1"))
				Expect(newSeed.Spec.Backup.CredentialsRef.Kind).To(Equal("Secret"))
				Expect(newSeed.Spec.Backup.CredentialsRef.Namespace).To(Equal("namespace"))
				Expect(newSeed.Spec.Backup.CredentialsRef.Name).To(Equal("name"))
			})

			It("should sync backup.credentialsRef referring secret with backup.secretRef", func() {
				newSeed.Spec.Backup = &core.Backup{
					CredentialsRef: &corev1.ObjectReference{
						APIVersion: "v1",
						Kind:       "Secret",
						Namespace:  "namespace",
						Name:       "name",
					},
				}

				Expect(newSeed.Spec.Backup.SecretRef.Namespace).To(BeEmpty())
				Expect(newSeed.Spec.Backup.SecretRef.Name).To(BeEmpty())

				strategy.PrepareForUpdate(ctx, newSeed, oldSeed)

				Expect(newSeed.Spec.Backup.SecretRef.Namespace).To(Equal("namespace"))
				Expect(newSeed.Spec.Backup.SecretRef.Name).To(Equal("name"))
			})

			It("should not sync backup.credentialsRef referring workloadidentity with backup.secretRef", func() {
				newSeed.Spec.Backup = &core.Backup{
					CredentialsRef: &corev1.ObjectReference{
						APIVersion: "security.gardener.cloud/v1alpha1",
						Kind:       "WorkloadIdentity",
						Namespace:  "namespace",
						Name:       "name",
					},
				}

				Expect(newSeed.Spec.Backup.SecretRef.Namespace).To(BeEmpty())
				Expect(newSeed.Spec.Backup.SecretRef.Name).To(BeEmpty())

				strategy.PrepareForUpdate(ctx, newSeed, oldSeed)

				Expect(newSeed.Spec.Backup.SecretRef.Namespace).To(BeEmpty())
				Expect(newSeed.Spec.Backup.SecretRef.Name).To(BeEmpty())
			})

			It("should not sync empty backup.credentialsRef with backup.secretRef", func() {
				newSeed.Spec.Backup = &core.Backup{
					CredentialsRef: nil,
					SecretRef:      corev1.SecretReference{},
				}

				strategy.PrepareForUpdate(ctx, newSeed, oldSeed)

				Expect(newSeed.Spec.Backup.SecretRef.Namespace).To(BeEmpty())
				Expect(newSeed.Spec.Backup.SecretRef.Name).To(BeEmpty())
				Expect(newSeed.Spec.Backup.CredentialsRef).To(BeNil())
			})

			It("should not sync backup.credentialsRef with backup.secretRef when they refer different resources", func() {
				newSeed.Spec.Backup = &core.Backup{
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

				strategy.PrepareForUpdate(ctx, newSeed, oldSeed)

				Expect(newSeed.Spec.Backup.SecretRef.Namespace).To(Equal("namespace"))
				Expect(newSeed.Spec.Backup.SecretRef.Name).To(Equal("name"))

				Expect(newSeed.Spec.Backup.CredentialsRef).ToNot(BeNil())
				Expect(newSeed.Spec.Backup.CredentialsRef.APIVersion).To(Equal("security.gardener.cloud/v1alpha1"))
				Expect(newSeed.Spec.Backup.CredentialsRef.Kind).To(Equal("WorkloadIdentity"))
				Expect(newSeed.Spec.Backup.CredentialsRef.Namespace).To(Equal("namespace"))
				Expect(newSeed.Spec.Backup.CredentialsRef.Name).To(Equal("name"))
			})

			It("Should not sync anything when backup is nil", func() {
				newSeed.Spec.Backup = nil

				strategy.PrepareForUpdate(ctx, newSeed, oldSeed)

				Expect(newSeed.Spec.Backup).To(BeNil())
			})
		})
	})
})
