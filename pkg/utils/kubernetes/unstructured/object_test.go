// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package unstructured_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/gardener/gardener/pkg/utils/kubernetes/unstructured"
)

var _ = Describe("Object", func() {
	Describe("#FilterMetadata", func() {
		It("should remove the fields", func() {
			content := map[string]any{
				"foo": "",
				"bar": "",
				"metadata": map[string]any{
					"foo": "",
					"bar": "",
				},
			}

			Expect(FilterMetadata(content, "foo", "bar")).To(Equal(map[string]any{
				"foo":      "",
				"bar":      "",
				"metadata": map[string]any{},
			}))
		})
	})
})
