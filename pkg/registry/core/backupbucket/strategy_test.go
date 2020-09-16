// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package backupbucket_test

import (
	"github.com/gardener/gardener/pkg/apis/core"
	backupbucketregistry "github.com/gardener/gardener/pkg/registry/core/backupbucket"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
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
