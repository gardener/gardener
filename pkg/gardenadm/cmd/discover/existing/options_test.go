// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package existing_test

import (
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/gardener/gardener/pkg/gardenadm/cmd/discover/existing"
	"github.com/gardener/gardener/pkg/gardenadm/cmd/discover/internal/shared"
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
			options.Name = "test-shoot"
			options.Namespace = "garden-test"
			options.ConfigDir = "some-path-to-config-dir"

			Expect(options.Validate()).To(Succeed())
		})

		It("should fail when --name is not set", func() {
			options.Kubeconfig = "some-path-to-kubeconfig"
			options.Namespace = "garden-test"
			options.ConfigDir = "some-path-to-config-dir"

			Expect(options.Validate()).To(MatchError(ContainSubstring("must provide --name")))
		})

		It("should fail when --namespace is not set", func() {
			options.Kubeconfig = "some-path-to-kubeconfig"
			options.Name = "test-shoot"
			options.ConfigDir = "some-path-to-config-dir"

			Expect(options.Validate()).To(MatchError(ContainSubstring("must provide --namespace")))
		})

		It("should fail when kubeconfig is not set", func() {
			options.Name = "test-shoot"
			options.Namespace = "garden-test"
			options.ConfigDir = "some-path-to-config-dir"

			Expect(options.Validate()).To(MatchError(ContainSubstring("must provide a path to a garden cluster kubeconfig")))
		})

		It("should fail when --config-dir is not set", func() {
			options.Kubeconfig = "some-path-to-kubeconfig"
			options.Name = "test-shoot"
			options.Namespace = "garden-test"

			Expect(options.Validate()).To(MatchError(ContainSubstring("must provide a path to a config directory")))
		})
	})

	Describe("#Complete", func() {
		It("should return nil", func() {
			Expect(options.Complete()).To(Succeed())
		})
	})
})
