// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	. "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1/helper"
)

var _ = Describe("helper", func() {
	Describe("#HasContainerdConfiguration", func() {
		It("should return false when CRI config is nil", func() {
			Expect(HasContainerdConfiguration(nil)).To(BeFalse())
		})

		It("should return false when containerd is not configured", func() {
			Expect(HasContainerdConfiguration(&extensionsv1alpha1.CRIConfig{
				Name: "cri-o",
			})).To(BeFalse())
		})

		It("should return true when containerd is configured", func() {
			Expect(HasContainerdConfiguration(&extensionsv1alpha1.CRIConfig{
				Name:       "containerd",
				Containerd: &extensionsv1alpha1.ContainerdConfig{},
			})).To(BeTrue())
		})
	})

	Describe("#FilePathsFrom", func() {
		It("should return the expected list", func() {
			file1 := extensionsv1alpha1.File{Path: "foo"}
			file2 := extensionsv1alpha1.File{Path: "bar"}

			Expect(FilePathsFrom([]extensionsv1alpha1.File{file1, file2})).To(ConsistOf("foo", "bar"))
		})
	})
})
