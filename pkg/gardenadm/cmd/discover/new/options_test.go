// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package new_test

import (
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/gardener/gardener/pkg/gardenadm/cmd/discover/internal/shared"
	. "github.com/gardener/gardener/pkg/gardenadm/cmd/discover/new"
)

var _ = Describe("Options", func() {
	var options *Options

	BeforeEach(func() {
		options = &Options{CommonOptions: &shared.CommonOptions{}}
	})

	Describe("#ParseArgs", func() {
		It("should set the kubeconfig from KUBECONFIG env var", func() {
			Expect(os.Setenv("KUBECONFIG", "kubeconfig")).To(Succeed())
			DeferCleanup(func() { Expect(os.Unsetenv("KUBECONFIG")).To(Succeed()) })

			Expect(options.ParseArgs(nil)).To(Succeed())
			Expect(options.Kubeconfig).To(Equal("kubeconfig"))
		})
	})

	Describe("#Validate", func() {
		It("should pass for valid options", func() {
			options.Kubeconfig = "some-path-to-kubeconfig"
			options.Manifest = "some-path-to-shoot-manifest"
			options.ConfigDir = "some-path-to-config-dir"

			Expect(options.Validate()).To(Succeed())
		})

		It("should fail when --manifest is not set", func() {
			options.Kubeconfig = "some-path-to-kubeconfig"
			options.ConfigDir = "some-path-to-config-dir"

			Expect(options.Validate()).To(MatchError(ContainSubstring("must provide --manifest")))
		})

		It("should fail when kubeconfig is not set", func() {
			options.Manifest = "some-path-to-shoot-manifest"
			options.ConfigDir = "some-path-to-config-dir"

			Expect(options.Validate()).To(MatchError(ContainSubstring("must provide a path to a garden cluster kubeconfig")))
		})

		It("should default the config dir from the manifest path during validation", func() {
			options.Kubeconfig = "some-path-to-kubeconfig"
			options.Manifest = "foo/bar/baz.yaml"

			Expect(options.Validate()).To(Succeed())
			Expect(options.ConfigDir).To(Equal("foo/bar"))
		})
	})

	Describe("#Complete", func() {
		It("should not change the config dir", func() {
			options.ConfigDir = "baz"
			Expect(options.Complete()).To(Succeed())
			Expect(options.ConfigDir).To(Equal("baz"))
		})
	})
})
