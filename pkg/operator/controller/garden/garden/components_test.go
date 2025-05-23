// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package garden_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	. "github.com/gardener/gardener/pkg/operator/controller/garden/garden"
)

var _ = Describe("Components", func() {
	Describe("GetAPIServerSNIDomains", func() {
		It("should return the correct SNI domains", func() {
			domains := []string{"foo.bar", "bar.foo.bar", "foo.foo.bar", "foo.bar.foo.bar", "api.bar"}
			sni := operatorv1alpha1.SNI{
				DomainPatterns: []string{"api.bar", "*.foo.bar"},
			}

			Expect(GetAPIServerSNIDomains(domains, sni)).To(Equal([]string{"api.bar", "bar.foo.bar", "foo.foo.bar"}))
		})
	})
})
