// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package bootstrap_test

import (
	"bytes"
	"io"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericiooptions"

	"github.com/gardener/gardener/pkg/gardenadm/cmd"
	. "github.com/gardener/gardener/pkg/gardenadm/cmd/bootstrap"
)

var _ = Describe("Bootstrap", func() {
	var (
		globalOpts *cmd.Options
		out        *bytes.Buffer
		command    *cobra.Command
	)

	BeforeEach(func() {
		globalOpts = &cmd.Options{}
		globalOpts.IOStreams, _, out, _ = genericiooptions.NewTestIOStreams()
		command = NewCommand(globalOpts)
	})

	Describe("#RunE", func() {
		It("should return the expected output", func() {
			Expect(command.Flags().Set("kubeconfig", "some-path-to-kubeconfig")).To(Succeed())

			Expect(command.RunE(command, nil)).To(Succeed())

			output, err := io.ReadAll(out)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(output)).To(Equal("not implemented as well\n"))
		})
	})
})
