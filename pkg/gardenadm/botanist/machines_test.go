// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist_test

import (
	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/gardener/gardener/pkg/gardenadm/botanist"
)

var _ = Describe("Machines", func() {
	Describe("#PreferredAddressForMachine", func() {
		var machine *machinev1alpha1.Machine

		BeforeEach(func() {
			machine = &machinev1alpha1.Machine{
				ObjectMeta: metav1.ObjectMeta{Name: "test-machine"},
				Status:     machinev1alpha1.MachineStatus{},
			}
		})

		It("should return error if no addresses are present", func() {
			Expect(PreferredAddressForMachine(machine)).Error().To(MatchError(ContainSubstring("no addresses found")))
		})

		It("should return the only address present", func() {
			machine.Status.Addresses = []corev1.NodeAddress{{Type: corev1.NodeExternalIP, Address: "1.2.3.4"}}
			Expect(PreferredAddressForMachine(machine)).To(Equal("1.2.3.4"))
		})

		It("should return the address with the highest preference", func() {
			machine.Status.Addresses = []corev1.NodeAddress{
				{Type: corev1.NodeExternalIP, Address: "1.2.3.4"},
				{Type: corev1.NodeHostName, Address: "host.local"},
				{Type: corev1.NodeInternalIP, Address: "10.0.0.2"},
			}
			Expect(PreferredAddressForMachine(machine)).To(Equal("10.0.0.2"))
		})

		It("should prefer InternalDNS over ExternalIP", func() {
			machine.Status.Addresses = []corev1.NodeAddress{
				{Type: corev1.NodeExternalIP, Address: "1.2.3.4"},
				{Type: corev1.NodeInternalDNS, Address: "internal.dns"},
			}
			Expect(PreferredAddressForMachine(machine)).To(Equal("internal.dns"))
		})

		It("should return unknown type if only unknown is present", func() {
			machine.Status.Addresses = []corev1.NodeAddress{{Type: "UnknownType", Address: "unknown.addr"}}
			Expect(PreferredAddressForMachine(machine)).To(Equal("unknown.addr"))
		})

		It("should prefer known type over unknown type", func() {
			machine.Status.Addresses = []corev1.NodeAddress{
				{Type: "UnknownType", Address: "unknown.addr"},
				{Type: corev1.NodeExternalDNS, Address: "external.dns"},
			}
			Expect(PreferredAddressForMachine(machine)).To(Equal("external.dns"))
		})
	})
})
