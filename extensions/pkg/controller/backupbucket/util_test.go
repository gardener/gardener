// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package backupbucket_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/gardener/gardener/extensions/pkg/controller/backupbucket"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

var _ = Describe("Util", func() {
	Describe("#GeneratedSecretObjectMeta", func() {
		var backupBucket *extensionsv1alpha1.BackupBucket

		BeforeEach(func() {
			backupBucket = &extensionsv1alpha1.BackupBucket{ObjectMeta: metav1.ObjectMeta{Name: "foo"}}
		})

		It("should return 'garden' namespace since annotation is not present", func() {
			Expect(GeneratedSecretObjectMeta(backupBucket)).To(Equal(metav1.ObjectMeta{
				Name:      "generated-bucket-foo",
				Namespace: "garden",
			}))
		})

		It("should return 'bar' namespace as specified in the annotation", func() {
			metav1.SetMetaDataAnnotation(&backupBucket.ObjectMeta, "backupbucket.extensions.gardener.cloud/generated-secret-namespace", "bar")
			Expect(GeneratedSecretObjectMeta(backupBucket)).To(Equal(metav1.ObjectMeta{
				Name:      "generated-bucket-foo",
				Namespace: "bar",
			}))
		})
	})
})
