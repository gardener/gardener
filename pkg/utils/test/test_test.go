// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package test_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/gardener/gardener/pkg/utils/test"
)

func TestUtil(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Test Utils Suite")
}

var _ = Describe("Test Utils", func() {
	Describe("#WithVar", func() {
		It("should set and revert the value", func() {
			v := "foo"
			cleanup := WithVar(&v, "bar")
			Expect(v).To(Equal("bar"))
			cleanup()
			Expect(v).To(Equal("foo"))
		})
	})
})
