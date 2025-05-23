// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/gardener/gardener/pkg/apis/core"
	. "github.com/gardener/gardener/pkg/apis/core/helper"
)

var _ = Describe("Helper", func() {
	DescribeTable("#GetSecretBindingTypes",
		func(secretBinding *core.SecretBinding, expected []string) {
			actual := GetSecretBindingTypes(secretBinding)
			Expect(actual).To(Equal(expected))
		},

		Entry("with nil provider type", &core.SecretBinding{Provider: nil}, []string{}),
		Entry("with single-value provider type", &core.SecretBinding{Provider: &core.SecretBindingProvider{Type: "foo"}}, []string{"foo"}),
		Entry("with multi-value provider type", &core.SecretBinding{Provider: &core.SecretBindingProvider{Type: "foo,bar,baz"}}, []string{"foo", "bar", "baz"}),
	)
})
