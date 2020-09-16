// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package kubernetes_test

import (
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("service", func() {
	Describe("#DNSNamesForService", func() {
		It("should return all expected DNS names for the given service name and namespace", func() {
			Expect(kutil.DNSNamesForService("test", "default")).To(Equal([]string{
				"test",
				"test.default",
				"test.default.svc",
				"test.default.svc.cluster.local",
			}))
		})
	})
})
