// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package token_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericiooptions"

	. "github.com/gardener/gardener/pkg/gardenadm/cmd/token"
)

var _ = Describe("Token", func() {
	var (
		ioStreams genericiooptions.IOStreams
		cmd       *cobra.Command
	)

	BeforeEach(func() {
		ioStreams, _, _, _ = genericiooptions.NewTestIOStreams()
		cmd = NewCommand(ioStreams)
	})

	Describe("#RunE", func() {
		It("should not have a Run function", func() {
			Expect(cmd.RunE).To(BeNil())
		})
	})
})
