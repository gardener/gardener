// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package list_test

import (
	"bytes"
	"io"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericiooptions"

	. "github.com/gardener/gardener/pkg/gardenadm/cmd/token/list"
)

var _ = Describe("List", func() {
	var (
		ioStreams genericiooptions.IOStreams
		out       *bytes.Buffer
		cmd       *cobra.Command
	)

	BeforeEach(func() {
		ioStreams, _, out, _ = genericiooptions.NewTestIOStreams()
		cmd = NewCommand(ioStreams)
	})

	Describe("#RunE", func() {
		It("should return the expected output", func() {
			Expect(cmd.RunE(cmd, nil)).To(Succeed())

			output, err := io.ReadAll(out)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(output)).To(Equal("not implemented\n"))
		})
	})
})
