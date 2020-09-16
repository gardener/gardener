// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
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
