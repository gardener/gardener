// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package utils_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"

	. "github.com/gardener/gardener/pkg/resourcemanager/controller/health/utils"
)

var _ = Describe("Ignored", func() {
	Describe("#IsIgnored", func() {
		var obj *corev1.Secret

		BeforeEach(func() {
			obj = &corev1.Secret{}
		})

		It("should return false because annotation does not exist", func() {
			Expect(IsIgnored(obj)).To(BeFalse())
		})

		It("should return false because annotation value is not truthy", func() {
			obj.Annotations = map[string]string{"resources.gardener.cloud/ignore": "foo"}
			Expect(IsIgnored(obj)).To(BeFalse())
		})

		It("should return true because annotation value is  truthy", func() {
			obj.Annotations = map[string]string{"resources.gardener.cloud/ignore": "true"}
			Expect(IsIgnored(obj)).To(BeTrue())
		})
	})
})
