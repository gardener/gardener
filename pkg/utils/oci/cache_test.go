// SPDX-FileCopyrightText: Copyright Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package oci

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("cache", func() {
	It("should store and retrieve values", func() {
		key := "foo"
		data := []byte("bar")
		c := newCache()

		_, found := c.Get(key)
		Expect(found).To(BeFalse())

		c.Set(key, data)

		out, found := c.Get(key)
		Expect(found).To(BeTrue())
		Expect(out).To(Equal(data))
	})
})
