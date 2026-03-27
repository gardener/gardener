// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardener_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/pkg/utils/gardener"
)

var _ = Describe("ControllerInstallation", func() {
	Describe("#ManagedResourceNameForControllerInstallation", func() {
		It("should return the ControllerInstallation name for seed ControllerInstallations", func() {
			controllerInstallation := &gardencorev1beta1.ControllerInstallation{
				ObjectMeta: metav1.ObjectMeta{Name: "foo-abc12"},
				Spec: gardencorev1beta1.ControllerInstallationSpec{
					RegistrationRef: corev1.ObjectReference{Name: "foo"},
					SeedRef:         &corev1.ObjectReference{Name: "seed1"},
				},
			}
			Expect(ManagedResourceNameForControllerInstallation(controllerInstallation)).To(Equal("foo-abc12"))
		})

		It("should return the RegistrationRef name for self-hosted shoot ControllerInstallations", func() {
			controllerInstallation := &gardencorev1beta1.ControllerInstallation{
				ObjectMeta: metav1.ObjectMeta{Name: "foo-abc12"},
				Spec: gardencorev1beta1.ControllerInstallationSpec{
					RegistrationRef: corev1.ObjectReference{Name: "foo"},
					ShootRef:        &corev1.ObjectReference{Name: "shoot1", Namespace: "garden"},
				},
			}
			Expect(ManagedResourceNameForControllerInstallation(controllerInstallation)).To(Equal("foo"))
		})
	})

	Describe("#NamespaceNameForControllerInstallation", func() {
		It("should return the correct namespace name for a seed ControllerInstallation", func() {
			controllerInstallation := &gardencorev1beta1.ControllerInstallation{
				ObjectMeta: metav1.ObjectMeta{Name: "foo"},
			}
			Expect(NamespaceNameForControllerInstallation(controllerInstallation)).To(Equal("extension-foo"))
		})

		It("should return the correct namespace name for a self-hosted shoot ControllerInstallation", func() {
			controllerInstallation := &gardencorev1beta1.ControllerInstallation{
				ObjectMeta: metav1.ObjectMeta{Name: "foo-abc12"},
				Spec: gardencorev1beta1.ControllerInstallationSpec{
					RegistrationRef: corev1.ObjectReference{Name: "foo"},
					ShootRef:        &corev1.ObjectReference{Name: "shoot1", Namespace: "garden"},
				},
			}
			Expect(NamespaceNameForControllerInstallation(controllerInstallation)).To(Equal("extension-foo"))
		})
	})
})
