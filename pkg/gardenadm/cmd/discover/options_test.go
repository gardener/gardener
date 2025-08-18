// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package discover_test

import (
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/gardener/gardener/pkg/gardenadm/cmd/discover"
)

var _ = Describe("Options", func() {
	var (
		options *Options
	)

	BeforeEach(func() {
		options = &Options{}
	})

	Describe("#ParseArgs", func() {
		It("should set the kubeconfig", func() {
			Expect(os.Setenv("KUBECONFIG", "kubeconfig")).To(Succeed())

			Expect(options.ParseArgs(nil)).To(Succeed())

			Expect(options.Kubeconfig).To(Equal("kubeconfig"))
		})

		It("should set the shoot manifest and the config dir", func() {
			Expect(options.ParseArgs([]string{"foo/bar/baz.yaml"})).To(Succeed())

			Expect(options.ShootManifest).To(Equal("foo/bar/baz.yaml"))
			Expect(options.ConfigDir).To(Equal("foo/bar"))
		})

		It("should not default the config dir when explicitly specified", func() {
			options.ConfigDir = "baz"

			Expect(options.ParseArgs([]string{"foo/bar/baz.yaml"})).To(Succeed())

			Expect(options.ShootManifest).To(Equal("foo/bar/baz.yaml"))
			Expect(options.ConfigDir).To(Equal("baz"))
		})
	})

	Describe("#Validate", func() {
		It("should pass for valid options", func() {
			options.Kubeconfig = "some-path-to-kubeconfig"
			options.ShootManifest = "some-path-to-shoot-manifest"
			options.ConfigDir = "some-path-to-config-dir"

			Expect(options.Validate()).To(Succeed())
		})

		It("should fail because kubeconfig path is not set", func() {
			options.ShootManifest = "some-path-to-shoot-manifest"

			Expect(options.Validate()).To(MatchError(ContainSubstring("must provide a path to a garden cluster kubeconfig")))
		})

		It("should fail because shoot manifest path is not set", func() {
			options.Kubeconfig = "some-path-to-kubeconfig"

			Expect(options.Validate()).To(MatchError(ContainSubstring("must provide a path to the shoot manifest file")))
		})

		It("should fail because config dir path is not set", func() {
			options.ShootManifest = "some-path-to-shoot-manifest"
			options.Kubeconfig = "some-path-to-kubeconfig"

			Expect(options.Validate()).To(MatchError(ContainSubstring("must provide a path to a config directory")))
		})
	})

	Describe("#Complete", func() {
		It("should return nil", func() {
			Expect(options.Complete()).To(Succeed())
		})
	})
})
