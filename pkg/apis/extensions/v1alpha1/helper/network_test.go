// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper_test

import (
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	. "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1/helper"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("helper", func() {
	Describe("#DeterminePrimaryIPFamily", func() {
		It("should return IPv4 for empty ipFamilies", func() {
			Expect(DeterminePrimaryIPFamily(nil)).To(Equal(extensionsv1alpha1.IPFamilyIPv4))
			Expect(DeterminePrimaryIPFamily([]extensionsv1alpha1.IPFamily{})).To(Equal(extensionsv1alpha1.IPFamilyIPv4))
		})

		It("should return IPv4 if it's the first entry", func() {
			Expect(DeterminePrimaryIPFamily([]extensionsv1alpha1.IPFamily{extensionsv1alpha1.IPFamilyIPv4})).To(Equal(extensionsv1alpha1.IPFamilyIPv4))
			Expect(DeterminePrimaryIPFamily([]extensionsv1alpha1.IPFamily{extensionsv1alpha1.IPFamilyIPv4, extensionsv1alpha1.IPFamilyIPv6})).To(Equal(extensionsv1alpha1.IPFamilyIPv4))
		})

		It("should return IPv6 if it's the first entry", func() {
			Expect(DeterminePrimaryIPFamily([]extensionsv1alpha1.IPFamily{extensionsv1alpha1.IPFamilyIPv6})).To(Equal(extensionsv1alpha1.IPFamilyIPv6))
			Expect(DeterminePrimaryIPFamily([]extensionsv1alpha1.IPFamily{extensionsv1alpha1.IPFamilyIPv6, extensionsv1alpha1.IPFamilyIPv4})).To(Equal(extensionsv1alpha1.IPFamilyIPv6))
		})
	})
})
