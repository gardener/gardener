// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package backupentry_test

import (
	"github.com/gardener/gardener/pkg/apis/core"
	backupentryregistry "github.com/gardener/gardener/pkg/registry/core/backupentry"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
)

var _ = Describe("ToSelectableFields", func() {
	It("should return correct fields", func() {
		result := backupentryregistry.ToSelectableFields(newBackupEntry("foo"))

		Expect(result).To(HaveLen(3))
		Expect(result.Has(core.BackupEntrySeedName)).To(BeTrue())
		Expect(result.Get(core.BackupEntrySeedName)).To(Equal("foo"))
	})
})

var _ = Describe("GetAttrs", func() {
	It("should return error when object is not BackupEntry", func() {
		_, _, err := backupentryregistry.GetAttrs(&core.Seed{})
		Expect(err).To(HaveOccurred())
	})

	It("should return correct result", func() {
		ls, fs, err := backupentryregistry.GetAttrs(newBackupEntry("foo"))

		Expect(ls).To(HaveLen(1))
		Expect(ls.Get("foo")).To(Equal("bar"))
		Expect(fs.Get(core.BackupEntrySeedName)).To(Equal("foo"))
		Expect(err).NotTo(HaveOccurred())
	})
})

var _ = Describe("SeedNameTriggerFunc", func() {
	It("should return spec.seedName", func() {
		actual := backupentryregistry.SeedNameTriggerFunc(newBackupEntry("foo"))
		Expect(actual).To(Equal("foo"))
	})
})

var _ = Describe("MatchBackupEntry", func() {
	It("should return correct predicate", func() {
		ls, _ := labels.Parse("app=test")
		fs := fields.OneTermEqualSelector(core.BackupEntrySeedName, "foo")

		result := backupentryregistry.MatchBackupEntry(ls, fs)

		Expect(result.Label).To(Equal(ls))
		Expect(result.Field).To(Equal(fs))
		Expect(result.IndexFields).To(ConsistOf(core.BackupEntrySeedName))
	})
})

func newBackupEntry(seedName string) *core.BackupEntry {
	return &core.BackupEntry{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "test-namespace",
			Labels:    map[string]string{"foo": "bar"},
		},
		Spec: core.BackupEntrySpec{
			SeedName: &seedName,
		},
	}
}
