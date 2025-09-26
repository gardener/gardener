// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package seed

import (
	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = ginkgo.Describe("PrometheusRules", func() {
	ginkgo.Describe("#CentralPrometheusRules", func() {
		ginkgo.It("should return the expected objects", func() {
			Expect(CentralPrometheusRules()).To(HaveExactElements())
		})
	})
})
