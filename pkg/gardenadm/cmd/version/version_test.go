// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package version_test

import (
	"bytes"
	"io"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericiooptions"

	. "github.com/gardener/gardener/pkg/gardenadm/cmd/version"
)

var _ = Describe("Version", func() {
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
			cmd.Run(cmd, nil)

			output, err := io.ReadAll(out)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(output)).To(Equal("gardenadm version v0.0.0-master+$Format:%H$"))
		})
	})
})
