// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package backupentry_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/pkg/apis/core"
	backupentryregistry "github.com/gardener/gardener/pkg/apiserver/registry/core/backupentry"
)

var _ = Describe("PrepareForUpdate", func() {
	var (
		oldBackupEntry *core.BackupEntry
		backupEntry    *core.BackupEntry
	)

	It("should increase the generation if spec has changed", func() {
		oldBackupEntry = newBackupEntry("seed", "bucket")

		backupEntry = oldBackupEntry.DeepCopy()
		backupEntry.Spec.BucketName = "other-bucket"

		backupentryregistry.NewStrategy().PrepareForUpdate(context.TODO(), backupEntry, oldBackupEntry)

		Expect(backupEntry.Generation).To(Equal(oldBackupEntry.Generation + 1))
	})

	It("should increase the generation if the new backupEntry has deletionTimestamp", func() {
		oldBackupEntry = newBackupEntry("seed", "bucket")

		backupEntry = oldBackupEntry.DeepCopy()
		backupEntry.DeletionTimestamp = &metav1.Time{Time: time.Now()}

		backupentryregistry.NewStrategy().PrepareForUpdate(context.TODO(), backupEntry, oldBackupEntry)

		Expect(backupEntry.Generation).To(Equal(oldBackupEntry.Generation + 1))
	})

	It("should increase the generation if the new backupEntry has ForceDeletion annotation", func() {
		oldBackupEntry = newBackupEntry("seed", "bucket")

		backupEntry = oldBackupEntry.DeepCopy()
		metav1.SetMetaDataAnnotation(&backupEntry.ObjectMeta, "backupentry.core.gardener.cloud/force-deletion", "true")

		backupentryregistry.NewStrategy().PrepareForUpdate(context.TODO(), backupEntry, oldBackupEntry)

		Expect(backupEntry.Generation).To(Equal(oldBackupEntry.Generation + 1))
	})

	It("should not increase the generation if both the new and old backupEntry has ForceDeletion annotation", func() {
		oldBackupEntry = newBackupEntry("seed", "bucket")

		backupEntry = oldBackupEntry.DeepCopy()
		metav1.SetMetaDataAnnotation(&backupEntry.ObjectMeta, "backupentry.core.gardener.cloud/force-deletion", "true")
		metav1.SetMetaDataAnnotation(&oldBackupEntry.ObjectMeta, "backupentry.core.gardener.cloud/force-deletion", "true")

		backupentryregistry.NewStrategy().PrepareForUpdate(context.TODO(), backupEntry, oldBackupEntry)

		Expect(backupEntry.Generation).To(Equal(oldBackupEntry.Generation))
	})

	It("should increase the generation if the new backupEntry has Operation Annotation other than restore and remove the annotation", func() {
		oldBackupEntry = newBackupEntry("seed", "bucket")

		backupEntry = oldBackupEntry.DeepCopy()
		metav1.SetMetaDataAnnotation(&backupEntry.ObjectMeta, "gardener.cloud/operation", "reconcile")

		backupentryregistry.NewStrategy().PrepareForUpdate(context.TODO(), backupEntry, oldBackupEntry)

		Expect(backupEntry.Generation).To(Equal(oldBackupEntry.Generation + 1))
		Expect(backupEntry.Annotations).NotTo(HaveKey("gardener.cloud/operation"))
	})

	It("should increase the generation if the new backupEntry has Operation Annotation restore and not remove the annotation", func() {
		oldBackupEntry = newBackupEntry("seed", "bucket")

		backupEntry = oldBackupEntry.DeepCopy()
		metav1.SetMetaDataAnnotation(&backupEntry.ObjectMeta, "gardener.cloud/operation", "restore")

		backupentryregistry.NewStrategy().PrepareForUpdate(context.TODO(), backupEntry, oldBackupEntry)

		Expect(backupEntry.Generation).To(Equal(oldBackupEntry.Generation + 1))
		Expect(backupEntry.Annotations).To(HaveKey("gardener.cloud/operation"))
	})

	It("should not increase the generation if both the new and old backupEntry has Operation Annotation restore and not remove the annotation", func() {
		oldBackupEntry = newBackupEntry("seed", "bucket")

		backupEntry = oldBackupEntry.DeepCopy()
		metav1.SetMetaDataAnnotation(&backupEntry.ObjectMeta, "gardener.cloud/operation", "restore")
		metav1.SetMetaDataAnnotation(&oldBackupEntry.ObjectMeta, "gardener.cloud/operation", "restore")

		backupentryregistry.NewStrategy().PrepareForUpdate(context.TODO(), backupEntry, oldBackupEntry)

		Expect(backupEntry.Generation).To(Equal(oldBackupEntry.Generation))
		Expect(backupEntry.Annotations).To(HaveKey("gardener.cloud/operation"))
	})
})

var _ = Describe("#Canonicalize", func() {
	var backupEntry *core.BackupEntry

	BeforeEach(func() {
		backupEntry = &core.BackupEntry{}
	})

	Context("seed names", func() {
		It("should correctly add the seed labels", func() {
			metav1.SetMetaDataLabel(&backupEntry.ObjectMeta, "foo", "bar")
			metav1.SetMetaDataLabel(&backupEntry.ObjectMeta, "name.seed.gardener.cloud/foo", "true")
			backupEntry.Spec.SeedName = ptr.To("spec-seed")
			backupEntry.Status.SeedName = ptr.To("status-seed")

			backupentryregistry.NewStrategy().Canonicalize(backupEntry)

			Expect(backupEntry.Labels).To(Equal(map[string]string{
				"foo":                                  "bar",
				"name.seed.gardener.cloud/spec-seed":   "true",
				"name.seed.gardener.cloud/status-seed": "true",
			}))
		})
	})
})

var _ = Describe("ToSelectableFields", func() {
	It("should return correct fields", func() {
		result := backupentryregistry.ToSelectableFields(newBackupEntry("foo", "dash"))

		Expect(result).To(HaveLen(4))
		Expect(result.Has(core.BackupEntrySeedName)).To(BeTrue())
		Expect(result.Get(core.BackupEntrySeedName)).To(Equal("foo"))
		Expect(result.Has(core.BackupEntryBucketName)).To(BeTrue())
		Expect(result.Get(core.BackupEntryBucketName)).To(Equal("dash"))
	})
})

var _ = Describe("GetAttrs", func() {
	It("should return error when object is not BackupEntry", func() {
		_, _, err := backupentryregistry.GetAttrs(&core.Seed{})
		Expect(err).To(HaveOccurred())
	})

	It("should return correct result", func() {
		ls, fs, err := backupentryregistry.GetAttrs(newBackupEntry("foo", "dash"))

		Expect(ls).To(HaveLen(1))
		Expect(ls.Get("foo")).To(Equal("bar"))
		Expect(fs.Get(core.BackupEntrySeedName)).To(Equal("foo"))
		Expect(err).NotTo(HaveOccurred())
	})
})

var _ = Describe("SeedNameIndexFunc", func() {
	It("should return spec.seedName", func() {
		result, err := backupentryregistry.SeedNameIndexFunc(newBackupEntry("foo", "dash"))
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(ConsistOf("foo"))
	})
})

var _ = Describe("BucketNameIndexFunc", func() {
	It("should return spec.BucketName", func() {
		result, err := backupentryregistry.BucketNameIndexFunc(newBackupEntry("foo", "dash"))
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(ConsistOf("dash"))
	})
})

var _ = Describe("MatchBackupEntry", func() {
	It("should return correct predicate", func() {
		ls, _ := labels.Parse("app=test")
		fs := fields.OneTermEqualSelector(core.BackupEntrySeedName, "foo")

		result := backupentryregistry.MatchBackupEntry(ls, fs)

		Expect(result.Label).To(Equal(ls))
		Expect(result.Field).To(Equal(fs))
		Expect(result.IndexFields).To(ConsistOf(core.BackupEntrySeedName, core.BackupEntryBucketName))
	})
})

func newBackupEntry(seedName string, bucketName string) *core.BackupEntry {
	return &core.BackupEntry{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "test-namespace",
			Labels:    map[string]string{"foo": "bar"},
		},
		Spec: core.BackupEntrySpec{
			SeedName:   &seedName,
			BucketName: bucketName,
		},
	}
}
