// SPDX-FileCopyrightText: Copyright Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package botanist_test

import (
	"context"

	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	. "github.com/gardener/gardener/pkg/gardenadm/botanist"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	botanistpkg "github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
)

var _ = Describe("Machines", func() {
	Describe("#ZoneForMachine", func() {
		var (
			ctx          context.Context
			b            *GardenadmBotanist
			machine      *machinev1alpha1.Machine
			machineClass *machinev1alpha1.MachineClass
		)

		BeforeEach(func() {
			ctx = context.Background()

			machineClass = &machinev1alpha1.MachineClass{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-class",
					Namespace: "test-ns",
				},
			}
			machine = &machinev1alpha1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-machine",
					Namespace: "test-ns",
				},
				Spec: machinev1alpha1.MachineSpec{
					Class: machinev1alpha1.ClassSpec{
						Name: "my-class",
					},
				},
			}
		})

		JustBeforeEach(func() {
			seedClient := fakeclient.NewClientBuilder().
				WithScheme(kubernetes.SeedScheme).
				WithObjects(machineClass).
				Build()
			clientSet := fakekubernetes.NewClientSetBuilder().
				WithClient(seedClient).
				Build()
			b = &GardenadmBotanist{
				Botanist: &botanistpkg.Botanist{
					Operation: &operation.Operation{
						SeedClientSet: clientSet,
					},
				},
			}
		})

		It("should return an error if the MachineClass does not exist", func() {
			machine.Spec.Class.Name = "non-existent-class"
			_, err := b.ZoneForMachine(ctx, machine)
			Expect(err).To(MatchError(ContainSubstring("failed getting machine class")))
		})

		It("should return an empty string if the MachineClass has no NodeTemplate", func() {
			zone, err := b.ZoneForMachine(ctx, machine)
			Expect(err).NotTo(HaveOccurred())
			Expect(zone).To(BeEmpty())
		})

		When("the MachineClass has a NodeTemplate", func() {
			BeforeEach(func() {
				machineClass.NodeTemplate = &machinev1alpha1.NodeTemplate{}
			})

			It("should return an empty string if no zone is set", func() {
				zone, err := b.ZoneForMachine(ctx, machine)
				Expect(err).NotTo(HaveOccurred())
				Expect(zone).To(BeEmpty())
			})

			When("with a zone", func() {
				BeforeEach(func() {
					machineClass.NodeTemplate.Zone = "eu-west-1a"
				})

				It("should return the zone", func() {
					zone, err := b.ZoneForMachine(ctx, machine)
					Expect(err).NotTo(HaveOccurred())
					Expect(zone).To(Equal("eu-west-1a"))
				})
			})
		})
	})

	Describe("#PreferredNodeAddress", func() {
		var addresses []corev1.NodeAddress

		BeforeEach(func() {
			addresses = []corev1.NodeAddress{}
		})

		It("should return error if no addresses are present", func() {
			Expect(PreferredNodeAddress(addresses)).Error().To(MatchError(ContainSubstring("no addresses found")))
		})

		It("should return the only address present", func() {
			addresses = []corev1.NodeAddress{{Type: corev1.NodeExternalIP, Address: "1.2.3.4"}}
			Expect(PreferredNodeAddress(addresses)).To(Equal("1.2.3.4"))
		})

		It("should return the address with the highest preference", func() {
			addresses = []corev1.NodeAddress{
				{Type: corev1.NodeExternalIP, Address: "1.2.3.4"},
				{Type: corev1.NodeHostName, Address: "host.local"},
				{Type: corev1.NodeInternalIP, Address: "10.0.0.2"},
			}
			Expect(PreferredNodeAddress(addresses)).To(Equal("10.0.0.2"))
		})

		It("should prefer InternalDNS over ExternalIP", func() {
			addresses = []corev1.NodeAddress{
				{Type: corev1.NodeExternalIP, Address: "1.2.3.4"},
				{Type: corev1.NodeInternalDNS, Address: "internal.dns"},
			}
			Expect(PreferredNodeAddress(addresses)).To(Equal("internal.dns"))
		})

		It("should return unknown type if only unknown is present", func() {
			addresses = []corev1.NodeAddress{{Type: "UnknownType", Address: "unknown.addr"}}
			Expect(PreferredNodeAddress(addresses)).To(Equal("unknown.addr"))
		})

		It("should prefer known type over unknown type", func() {
			addresses = []corev1.NodeAddress{
				{Type: "UnknownType", Address: "unknown.addr"},
				{Type: corev1.NodeExternalDNS, Address: "external.dns"},
			}
			Expect(PreferredNodeAddress(addresses)).To(Equal("external.dns"))
		})
	})
})
