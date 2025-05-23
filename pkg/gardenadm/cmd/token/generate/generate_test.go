// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package generate_test

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gbytes"
	"github.com/spf13/cobra"

	"github.com/gardener/gardener/pkg/gardenadm/cmd"
	. "github.com/gardener/gardener/pkg/gardenadm/cmd/token/generate"
	"github.com/gardener/gardener/pkg/utils/kubernetes/bootstraptoken"
	clitest "github.com/gardener/gardener/pkg/utils/test/cli"
)

var _ = Describe("Generate", func() {
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
		It("should compute a random bootstrap token and print it", func() {
			Expect(command.RunE(command, nil)).To(Succeed())

			Eventually(func() string {
				return strings.TrimSpace(string(stdOut.Contents()))
			}).Should(MatchRegexp(bootstraptoken.ValidBootstrapTokenRegex.String()))
		})
	})
})
