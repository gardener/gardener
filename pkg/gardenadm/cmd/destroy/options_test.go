// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package destroy_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/gardener/gardener/pkg/gardenadm/cmd"
	. "github.com/gardener/gardener/pkg/gardenadm/cmd/destroy"
)

var _ = Describe("Options", func() {
	var options *Options

	BeforeEach(func() {
		options = &Options{
			Options: &cmd.Options{},
			ManifestOptions: cmd.ManifestOptions{
				ConfigDir: "some-path-to-config-dir",
			},
			Kubeconfig: "some-path-to-kubeconfig",
		}
	})

	Describe("#ParseArgs", func() {
		It("should return nil", func() {
			Expect(options.ParseArgs(nil)).To(Succeed())
		})

		It("should default the kubeconfig from the KUBECONFIG environment variable", func() {
			GinkgoT().Setenv("KUBECONFIG", "some-path-from-env")
			options.Kubeconfig = ""

			Expect(options.ParseArgs(nil)).To(Succeed())
			Expect(options.Kubeconfig).To(Equal("some-path-from-env"))
		})
	})

	Describe("#Validate", func() {
		It("should pass for valid options", func() {
			Expect(options.Validate()).To(Succeed())
		})

		It("should fail because kubeconfig path is not set", func() {
			options.Kubeconfig = ""

			Expect(options.Validate()).To(MatchError(ContainSubstring("must provide a path to a bootstrap cluster kubeconfig")))
		})

		It("should fail because config dir path is not set", func() {
			options.ConfigDir = ""

			Expect(options.Validate()).To(MatchError(ContainSubstring("must provide a path to a config directory")))
		})
	})

	Describe("#Complete", func() {
		It("should return nil", func() {
			Expect(options.Complete()).To(Succeed())
		})
	})
})
