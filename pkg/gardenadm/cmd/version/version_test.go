// SPDX-FileCopyrightText: Copyright Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package version_test

import (
	"regexp"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gbytes"
	"github.com/spf13/cobra"

	"github.com/gardener/gardener/pkg/gardenadm/cmd"
	. "github.com/gardener/gardener/pkg/gardenadm/cmd/version"
	clitest "github.com/gardener/gardener/pkg/utils/test/cli"
)

var _ = Describe("Version", func() {
	var (
		globalOpts *cmd.Options
		stdOut     *Buffer
		command    *cobra.Command
	)

	BeforeEach(func() {
		globalOpts = &cmd.Options{}
		globalOpts.IOStreams, _, stdOut, _ = clitest.NewTestIOStreams()
		command = NewCommand(globalOpts)
	})

	Describe("#RunE", func() {
		It("should return the expected output", func() {
			command.Run(command, nil)

			Eventually(stdOut).Should(Say(regexp.QuoteMeta("gardenadm version v0.0.0-master+$Format:%H$")))
		})
	})

	Describe("#Args", func() {
		It("should accept no arguments", func() {
			Expect(command.Args(command, nil)).To(Succeed())
		})

		It("should reject any positional arguments", func() {
			Expect(command.Args(command, []string{"foo"})).To(HaveOccurred())
		})
	})
})
