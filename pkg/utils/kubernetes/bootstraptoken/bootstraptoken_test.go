// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package bootstraptoken_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/gardener/gardener/pkg/utils/kubernetes/bootstraptoken"
)

var _ = Describe("#TokenID", func() {
	const (
		namespace = "bar"
		name      = "baz"
	)

	It("should compute the expected id (w/o namespace", func() {
		Expect(TokenID(metav1.ObjectMeta{Name: name})).To(Equal("baa5a0"))
	})

	It("should compute the expected id (w/ namespace", func() {
		Expect(TokenID(metav1.ObjectMeta{Name: name, Namespace: namespace})).To(Equal("cc19de"))
	})
})
