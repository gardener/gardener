// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package join_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gbytes"
	"github.com/spf13/cobra"
	logzap "sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/gardener/gardener/pkg/gardenadm/cmd"
	. "github.com/gardener/gardener/pkg/gardenadm/cmd/join"
	"github.com/gardener/gardener/pkg/logger"
	clitest "github.com/gardener/gardener/pkg/utils/test/cli"
)

var _ = Describe("Join", func() {
	var (
		globalOpts *cmd.Options
		stdErr     *Buffer
		command    *cobra.Command
	)

	BeforeEach(func() {
		globalOpts = &cmd.Options{}
		globalOpts.IOStreams, _, _, stdErr = clitest.NewTestIOStreams()
		globalOpts.Log = logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, logzap.WriteTo(stdErr))
		command = NewCommand(globalOpts)
	})

	Describe("#RunE", func() {
		It("should return the expected output", func() {
			Expect(command.RunE(command, nil)).To(Succeed())

			Eventually(stdErr).Should(Say("Not implemented either"))
		})
	})
})
