// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package kubernetes_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

var _ = Describe("service", func() {
	Describe("#DNSNamesForService", func() {
		It("should return all expected DNS names for the given service name and namespace", func() {
			Expect(kubernetesutils.DNSNamesForService("test", "default")).To(Equal([]string{
				"test",
				"test.default",
				"test.default.svc",
				"test.default.svc.cluster.local",
			}))
		})
	})

	Describe("#FQDNForService", func() {
		It("should return the expected fully qualified DNS name for the given service name and namespace", func() {
			Expect(kubernetesutils.FQDNForService("test", "default")).To(Equal("test.default.svc.cluster.local"))
		})
	})
})
