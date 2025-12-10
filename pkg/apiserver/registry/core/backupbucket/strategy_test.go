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
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/pkg/apis/core"
	backupbucketregistry "github.com/gardener/gardener/pkg/apiserver/registry/core/backupbucket"
)

var _ = Describe("ToSelectableFields", func() {
	It("should return correct fields", func() {
		result := backupbucketregistry.ToSelectableFields(newBackupBucket())

		Expect(result).To(HaveLen(5))
		Expect(result.Has(core.BackupBucketSeedName)).To(BeTrue())
		Expect(result.Get(core.BackupBucketSeedName)).To(Equal("foo"))
		Expect(result.Has(core.BackupBucketShootRefName)).To(BeTrue())
		Expect(result.Get(core.BackupBucketShootRefName)).To(Equal("shoot-name"))
		Expect(result.Has(core.BackupBucketShootRefNamespace)).To(BeTrue())
		Expect(result.Get(core.BackupBucketShootRefNamespace)).To(Equal("shoot-namespace"))
	})
})

var _ = Describe("GetAttrs", func() {
	It("should return error when object is not BackupBucket", func() {
		_, _, err := backupbucketregistry.GetAttrs(&core.Seed{})
		Expect(err).To(HaveOccurred())
	})

	It("should return correct result", func() {
		ls, fs, err := backupbucketregistry.GetAttrs(newBackupBucket())

		Expect(ls).To(HaveLen(1))
		Expect(ls.Get("foo")).To(Equal("bar"))
		Expect(fs.Get(core.BackupBucketSeedName)).To(Equal("foo"))
		Expect(fs.Get(core.BackupBucketShootRefName)).To(Equal("shoot-name"))
		Expect(fs.Get(core.BackupBucketShootRefNamespace)).To(Equal("shoot-namespace"))
		Expect(err).NotTo(HaveOccurred())
	})
})

var _ = Describe("SeedNameIndexFunc", func() {
	It("should return spec.seedName", func() {
		result, err := backupbucketregistry.SeedNameIndexFunc(newBackupBucket())
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(ConsistOf("foo"))
	})
})

var _ = Describe("ShootRefNameIndexFunc", func() {
	It("should return nothing because spec.shootRef is not set", func() {
		backupBucket := newBackupBucket()
		backupBucket.Spec.ShootRef = nil

		result, err := backupbucketregistry.ShootRefNameIndexFunc(backupBucket)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(ConsistOf(BeEmpty()))
	})

	It("should return spec.shootRef.name", func() {
		result, err := backupbucketregistry.ShootRefNameIndexFunc(newBackupBucket())
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(ConsistOf("shoot-name"))
	})
})

var _ = Describe("ShootRefNamespaceIndexFunc", func() {
	It("should return nothing because spec.shootRef is not set", func() {
		backupBucket := newBackupBucket()
		backupBucket.Spec.ShootRef = nil

		result, err := backupbucketregistry.ShootRefNamespaceIndexFunc(backupBucket)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(ConsistOf(BeEmpty()))
	})

	It("should return spec.shootRef.namespace", func() {
		result, err := backupbucketregistry.ShootRefNamespaceIndexFunc(newBackupBucket())
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(ConsistOf("shoot-namespace"))
	})
})

var _ = Describe("MatchBackupBucket", func() {
	It("should return correct predicate", func() {
		ls, _ := labels.Parse("app=test")
		fs := fields.OneTermEqualSelector(core.BackupBucketSeedName, "foo")

		result := backupbucketregistry.MatchBackupBucket(ls, fs)

		Expect(result.Label).To(Equal(ls))
		Expect(result.Field).To(Equal(fs))
		Expect(result.IndexFields).To(ConsistOf(core.BackupBucketSeedName, core.BackupBucketShootRefName, core.BackupBucketShootRefNamespace))
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
			It("should not bump generation if nothing changed", func() {
				strategy.PrepareForUpdate(ctx, newBucket, oldBucket)
				Expect(newBucket.Generation).To(Equal(oldBucket.Generation))
			})

			It("should increase generation when credentialsRef has changed", func() {
				newBucket.Spec.CredentialsRef = &corev1.ObjectReference{
					APIVersion: "v1",
					Kind:       "Secret",
					Namespace:  "namespace",
					Name:       "name",
				}

				strategy.PrepareForUpdate(ctx, newBucket, oldBucket)
				Expect(newBucket.Generation).To(Equal(oldBucket.Generation + 1))
			})

			It("should bump the generation if the deletionTimestamp was set", func() {
				now := metav1.Now()
				newBucket.DeletionTimestamp = &now

				strategy.PrepareForUpdate(ctx, newBucket, oldBucket)

				Expect(newBucket.Generation).To(Equal(oldBucket.Generation + 1))
			})

			It("should not bump the generation if the deletionTimestamp was already set", func() {
				now := metav1.Now()
				oldBucket.DeletionTimestamp = &now
				newBucket.DeletionTimestamp = &now

				strategy.PrepareForUpdate(ctx, newBucket, oldBucket)

				Expect(newBucket.Generation).To(Equal(oldBucket.Generation))
			})

			It("should bump the generation and remove the annotation if the operation annotation was set to reconcile", func() {
				metav1.SetMetaDataAnnotation(&newBucket.ObjectMeta, "gardener.cloud/operation", "reconcile")

				strategy.PrepareForUpdate(ctx, newBucket, oldBucket)

				Expect(newBucket.Generation).To(Equal(oldBucket.Generation + 1))
				Expect(newBucket.Annotations).NotTo(ContainElement("gardener.cloud/operation"))
			})

			It("should not bump the generation if the operation annotation change its value to other than reconcile", func() {
				metav1.SetMetaDataAnnotation(&oldBucket.ObjectMeta, "gardener.cloud/operation", "reconcile")
				metav1.SetMetaDataAnnotation(&newBucket.ObjectMeta, "gardener.cloud/operation", "other-operation")

				strategy.PrepareForUpdate(ctx, newBucket, oldBucket)

				Expect(newBucket.Generation).To(Equal(oldBucket.Generation))
			})

			It("should bump the generation if the operation annotation changed its value", func() {
				metav1.SetMetaDataAnnotation(&oldBucket.ObjectMeta, "gardener.cloud/operation", "other-operation")
				metav1.SetMetaDataAnnotation(&newBucket.ObjectMeta, "gardener.cloud/operation", "reconcile")

				strategy.PrepareForUpdate(ctx, newBucket, oldBucket)

				Expect(newBucket.Generation).To(Equal(oldBucket.Generation + 1))
			})

			It("should not bump the generation and remove the annotation if the operation annotation was not set to reconcile operation", func() {
				metav1.SetMetaDataAnnotation(&newBucket.ObjectMeta, "gardener.cloud/operation", "other-operation")

				strategy.PrepareForUpdate(ctx, newBucket, oldBucket)

				Expect(newBucket.Generation).To(Equal(oldBucket.Generation))
				Expect(newBucket.Annotations).To(HaveKeyWithValue("gardener.cloud/operation", "other-operation"))
			})
		})
	})
})

func newBackupBucket() *core.BackupBucket {
	return &core.BackupBucket{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "test-namespace",
			Labels:    map[string]string{"foo": "bar"},
		},
		Spec: core.BackupBucketSpec{
			SeedName: ptr.To("foo"),
			ShootRef: &corev1.ObjectReference{
				Name:      "shoot-name",
				Namespace: "shoot-namespace",
			},
		},
	}
}
