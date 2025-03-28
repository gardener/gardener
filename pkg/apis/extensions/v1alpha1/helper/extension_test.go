// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	. "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1/helper"
)

var _ = Describe("Extension", func() {
	Describe("#GetExtensionClassOrDefault", func() {
		It("should return the given extension class", func() {
			Expect(GetExtensionClassOrDefault(ptr.To[extensionsv1alpha1.ExtensionClass]("garden"))).To(BeEquivalentTo("garden"))
		})

		It("should return the given default class", func() {
			Expect(GetExtensionClassOrDefault(nil)).To(BeEquivalentTo("shoot"))
		})
	})
})
