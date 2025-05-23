// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package backupbucket_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/gardener/gardener/pkg/apis/core"
	backupbucketregistry "github.com/gardener/gardener/pkg/apiserver/registry/core/backupbucket"
)

var _ = Describe("ToSelectableFields", func() {
	It("should return correct fields", func() {
		result := backupbucketregistry.ToSelectableFields(newBackupBucket("foo"))

		Expect(result).To(HaveLen(3))
		Expect(result.Has(core.BackupBucketSeedName)).To(BeTrue())
		Expect(result.Get(core.BackupBucketSeedName)).To(Equal("foo"))
	})
})

var _ = Describe("GetAttrs", func() {
	It("should return error when object is not BackupBucket", func() {
		_, _, err := backupbucketregistry.GetAttrs(&core.Seed{})
		Expect(err).To(HaveOccurred())
	})

	It("should return correct result", func() {
		ls, fs, err := backupbucketregistry.GetAttrs(newBackupBucket("foo"))

		Expect(ls).To(HaveLen(1))
		Expect(ls.Get("foo")).To(Equal("bar"))
		Expect(fs.Get(core.BackupBucketSeedName)).To(Equal("foo"))
		Expect(err).NotTo(HaveOccurred())
	})
})

var _ = Describe("SeedNameTriggerFunc", func() {
	It("should return spec.seedName", func() {
		actual := backupbucketregistry.SeedNameTriggerFunc(newBackupBucket("foo"))
		Expect(actual).To(Equal("foo"))
	})
})

var _ = Describe("MatchBackupBucket", func() {
	It("should return correct predicate", func() {
		ls, _ := labels.Parse("app=test")
		fs := fields.OneTermEqualSelector(core.BackupBucketSeedName, "foo")

		result := backupbucketregistry.MatchBackupBucket(ls, fs)

		Expect(result.Label).To(Equal(ls))
		Expect(result.Field).To(Equal(fs))
		Expect(result.IndexFields).To(ConsistOf(core.BackupBucketSeedName))
	})
})

var _ = Describe("Strategy", func() {
	var (
		ctx      = context.TODO()
		strategy = backupbucketregistry.Strategy
	)

	Describe("#PrepareForUpdate", func() {
		var oldBucket, newBucket *core.BackupBucket

		BeforeEach(func() {
			oldBucket = &core.BackupBucket{}
			newBucket = &core.BackupBucket{}
		})

		Describe("#generationIncrement", func() {
			It("should bump generation when spec.secretRef is synced with spec.credentialsRef", func() {
				newBucket.Spec = core.BackupBucketSpec{
					SecretRef: corev1.SecretReference{
						Namespace: "namespace",
						Name:      "name",
					},
				}
				oldBucket = newBucket.DeepCopy()

				strategy.PrepareForUpdate(ctx, newBucket, oldBucket)
				Expect(newBucket.Generation).To(Equal(oldBucket.Generation + 1))
			})

			It("should not bump generation when spec.secretRef is already synced with spec.credentialsRef", func() {
				oldBucket.Spec = core.BackupBucketSpec{
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
				newBucket.Spec = core.BackupBucketSpec{
					SecretRef: corev1.SecretReference{
						Namespace: "namespace",
						Name:      "name",
					},
				}

				strategy.PrepareForUpdate(ctx, newBucket, oldBucket)
				Expect(newBucket.Generation).To(Equal(oldBucket.Generation))
			})

			It("should bump generation when spec.credentialsRef is synced with spec.secretRef", func() {
				newBucket.Spec = core.BackupBucketSpec{
					CredentialsRef: &corev1.ObjectReference{
						APIVersion: "v1",
						Kind:       "Secret",
						Namespace:  "namespace",
						Name:       "name",
					},
				}
				oldBucket = newBucket.DeepCopy()

				strategy.PrepareForUpdate(ctx, newBucket, oldBucket)
				Expect(newBucket.Generation).To(Equal(oldBucket.Generation + 1))
			})

			It("should not bump generation when spec.credentialsRef is already synced with spec.secretRef", func() {
				oldBucket.Spec = core.BackupBucketSpec{
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
				newBucket.Spec = core.BackupBucketSpec{
					CredentialsRef: &corev1.ObjectReference{
						APIVersion: "v1",
						Kind:       "Secret",
						Namespace:  "namespace",
						Name:       "name",
					},
				}

				strategy.PrepareForUpdate(ctx, newBucket, oldBucket)
				Expect(newBucket.Generation).To(Equal(oldBucket.Generation))
			})
		})

		Describe("#syncBackupSecretRefAndCredentialsRef", func() {
			It("should sync secretRef with credentialsRef", func() {
				newBucket.Spec = core.BackupBucketSpec{
					SecretRef: corev1.SecretReference{
						Namespace: "namespace",
						Name:      "name",
					},
				}

				Expect(newBucket.Spec.CredentialsRef).To(BeNil())

				strategy.PrepareForUpdate(ctx, newBucket, oldBucket)

				Expect(newBucket.Spec.CredentialsRef).ToNot(BeNil())
				Expect(newBucket.Spec.CredentialsRef.APIVersion).To(Equal("v1"))
				Expect(newBucket.Spec.CredentialsRef.Kind).To(Equal("Secret"))
				Expect(newBucket.Spec.CredentialsRef.Namespace).To(Equal("namespace"))
				Expect(newBucket.Spec.CredentialsRef.Name).To(Equal("name"))
			})

			It("should sync backup.credentialsRef referring secret with backup.secretRef", func() {
				newBucket.Spec = core.BackupBucketSpec{
					CredentialsRef: &corev1.ObjectReference{
						APIVersion: "v1",
						Kind:       "Secret",
						Namespace:  "namespace",
						Name:       "name",
					},
				}

				Expect(newBucket.Spec.SecretRef.Namespace).To(BeEmpty())
				Expect(newBucket.Spec.SecretRef.Name).To(BeEmpty())

				strategy.PrepareForUpdate(ctx, newBucket, oldBucket)

				Expect(newBucket.Spec.SecretRef.Namespace).To(Equal("namespace"))
				Expect(newBucket.Spec.SecretRef.Name).To(Equal("name"))
			})

			It("should not sync backup.credentialsRef referring workloadidentity with backup.secretRef", func() {
				newBucket.Spec = core.BackupBucketSpec{
					CredentialsRef: &corev1.ObjectReference{
						APIVersion: "security.gardener.cloud/v1alpha1",
						Kind:       "WorkloadIdentity",
						Namespace:  "namespace",
						Name:       "name",
					},
				}

				Expect(newBucket.Spec.SecretRef.Namespace).To(BeEmpty())
				Expect(newBucket.Spec.SecretRef.Name).To(BeEmpty())

				strategy.PrepareForUpdate(ctx, newBucket, oldBucket)

				Expect(newBucket.Spec.SecretRef.Namespace).To(BeEmpty())
				Expect(newBucket.Spec.SecretRef.Name).To(BeEmpty())
			})

			It("should not sync empty backup.credentialsRef with backup.secretRef", func() {
				newBucket.Spec = core.BackupBucketSpec{
					CredentialsRef: nil,
					SecretRef:      corev1.SecretReference{},
				}

				strategy.PrepareForUpdate(ctx, newBucket, oldBucket)

				Expect(newBucket.Spec.SecretRef.Namespace).To(BeEmpty())
				Expect(newBucket.Spec.SecretRef.Name).To(BeEmpty())
				Expect(newBucket.Spec.CredentialsRef).To(BeNil())
			})

			It("should not sync backup.credentialsRef with backup.secretRef when they refer different resources", func() {
				newBucket.Spec = core.BackupBucketSpec{
					CredentialsRef: &corev1.ObjectReference{
						APIVersion: "security.gardener.cloud/v1alpha1",
						Kind:       "WorkloadIdentity",
						Namespace:  "namespace1",
						Name:       "name1",
					},
					SecretRef: corev1.SecretReference{
						Namespace: "namespace2",
						Name:      "name2",
					},
				}

				strategy.PrepareForUpdate(ctx, newBucket, oldBucket)

				Expect(newBucket.Spec.SecretRef.Namespace).To(Equal("namespace2"))
				Expect(newBucket.Spec.SecretRef.Name).To(Equal("name2"))

				Expect(newBucket.Spec.CredentialsRef).ToNot(BeNil())
				Expect(newBucket.Spec.CredentialsRef.APIVersion).To(Equal("security.gardener.cloud/v1alpha1"))
				Expect(newBucket.Spec.CredentialsRef.Kind).To(Equal("WorkloadIdentity"))
				Expect(newBucket.Spec.CredentialsRef.Namespace).To(Equal("namespace1"))
				Expect(newBucket.Spec.CredentialsRef.Name).To(Equal("name1"))
			})
		})
	})
})

func newBackupBucket(seedName string) *core.BackupBucket {
	return &core.BackupBucket{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "test-namespace",
			Labels:    map[string]string{"foo": "bar"},
		},
		Spec: core.BackupBucketSpec{
			SeedName: &seedName,
		},
	}
}
