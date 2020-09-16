// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package kubernetes_test

import (
	. "github.com/gardener/gardener/pkg/client/kubernetes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("chart manifest options", func() {
	var (
		dopts *DeleteManifestOptions
	)

	BeforeEach(func() {
		dopts = &DeleteManifestOptions{}
	})

	Context("TolerateErrorFunc", func() {
		It("sets DeleteOptions", func() {
			var tTrue TolerateErrorFunc = func(_ error) bool { return true }
			tTrue.MutateDeleteManifestOptions(dopts)

			Expect(dopts.TolerateErrorFuncs).To(HaveLen(1))
			Expect(dopts.TolerateErrorFuncs[0](nil)).To(BeTrue())
		})
	})
})
