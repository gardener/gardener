// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"

	. "github.com/gardener/gardener/pkg/gardenadm/botanist"
)

var _ = Describe("Machines", func() {
	Describe("#PreferredAddress", func() {
		var addresses []corev1.NodeAddress

		BeforeEach(func() {
			addresses = []corev1.NodeAddress{}
		})

		It("should return error if no addresses are present", func() {
			Expect(PreferredAddress(addresses)).Error().To(MatchError(ContainSubstring("no addresses found")))
		})

		It("should return the only address present", func() {
			addresses = []corev1.NodeAddress{{Type: corev1.NodeExternalIP, Address: "1.2.3.4"}}
			Expect(PreferredAddress(addresses)).To(Equal("1.2.3.4"))
		})

		It("should return the address with the highest preference", func() {
			addresses = []corev1.NodeAddress{
				{Type: corev1.NodeExternalIP, Address: "1.2.3.4"},
				{Type: corev1.NodeHostName, Address: "host.local"},
				{Type: corev1.NodeInternalIP, Address: "10.0.0.2"},
			}
			Expect(PreferredAddress(addresses)).To(Equal("10.0.0.2"))
		})

		It("should prefer InternalDNS over ExternalIP", func() {
			addresses = []corev1.NodeAddress{
				{Type: corev1.NodeExternalIP, Address: "1.2.3.4"},
				{Type: corev1.NodeInternalDNS, Address: "internal.dns"},
			}
			Expect(PreferredAddress(addresses)).To(Equal("internal.dns"))
		})

		It("should return unknown type if only unknown is present", func() {
			addresses = []corev1.NodeAddress{{Type: "UnknownType", Address: "unknown.addr"}}
			Expect(PreferredAddress(addresses)).To(Equal("unknown.addr"))
		})

		It("should prefer known type over unknown type", func() {
			addresses = []corev1.NodeAddress{
				{Type: "UnknownType", Address: "unknown.addr"},
				{Type: corev1.NodeExternalDNS, Address: "external.dns"},
			}
			Expect(PreferredAddress(addresses)).To(Equal("external.dns"))
		})
	})
})
