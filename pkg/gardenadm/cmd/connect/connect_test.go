// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package connect_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"

	"github.com/gardener/gardener/pkg/gardenadm/cmd"
	. "github.com/gardener/gardener/pkg/gardenadm/cmd/connect"
	clitest "github.com/gardener/gardener/pkg/utils/test/cli"
)

var _ = Describe("Connect", func() {
	var (
		globalOpts *cmd.Options
		command    *cobra.Command
	)

	BeforeEach(func() {
		globalOpts = &cmd.Options{}
		globalOpts.IOStreams, _, _, _ = clitest.NewTestIOStreams()
		command = NewCommand(globalOpts)
	})

	Describe("#Args", func() {
		It("should accept no arguments", func() {
			Expect(command.Args(command, nil)).To(Succeed())
		})

		It("should accept a single positional argument", func() {
			Expect(command.Args(command, []string{"foo"})).To(Succeed())
		})

		It("should reject more than one positional argument", func() {
			Expect(command.Args(command, []string{"foo", "bar"})).To(HaveOccurred())
		})
	})
})
