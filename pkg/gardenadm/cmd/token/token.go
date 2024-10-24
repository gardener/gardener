// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package token

import (
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericiooptions"

	"github.com/gardener/gardener/pkg/gardenadm/cmd/token/create"
	"github.com/gardener/gardener/pkg/gardenadm/cmd/token/delete"
	"github.com/gardener/gardener/pkg/gardenadm/cmd/token/generate"
	"github.com/gardener/gardener/pkg/gardenadm/cmd/token/list"
)

// NewCommand creates a new cobra.Command.
func NewCommand(ioStreams genericiooptions.IOStreams) *cobra.Command {
	opts := &Options{}

	cmd := &cobra.Command{
		Use:   "token",
		Short: "Manage bootstrap and discovery tokens for gardenadm join",
		Long:  "Manage bootstrap and discovery tokens for gardenadm join",
	}

	opts.addFlags(cmd.Flags())

	cmd.AddCommand(list.NewCommand(ioStreams))
	cmd.AddCommand(generate.NewCommand(ioStreams))
	cmd.AddCommand(create.NewCommand(ioStreams))
	cmd.AddCommand(delete.NewCommand(ioStreams))

	return cmd
}
