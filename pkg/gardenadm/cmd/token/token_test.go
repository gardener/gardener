// SPDX-FileCopyrightText: Copyright Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package token_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"

	"github.com/gardener/gardener/pkg/gardenadm/cmd"
	. "github.com/gardener/gardener/pkg/gardenadm/cmd/token"
	clitest "github.com/gardener/gardener/pkg/utils/test/cli"
)

var _ = Describe("Token", func() {
	var (
		globalOpts *cmd.Options
		command    *cobra.Command
	)

	BeforeEach(func() {
		globalOpts = &cmd.Options{}
		globalOpts.IOStreams, _, _, _ = clitest.NewTestIOStreams()
		command = NewCommand(globalOpts)
	})

	Describe("#RunE", func() {
		It("should not have a Run function", func() {
			Expect(command.RunE).To(BeNil())
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
