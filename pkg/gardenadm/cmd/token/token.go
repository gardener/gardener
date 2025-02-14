// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package token

import (
	"github.com/spf13/cobra"

	"github.com/gardener/gardener/pkg/gardenadm/cmd"
	"github.com/gardener/gardener/pkg/gardenadm/cmd/token/create"
	"github.com/gardener/gardener/pkg/gardenadm/cmd/token/delete"
	"github.com/gardener/gardener/pkg/gardenadm/cmd/token/generate"
	"github.com/gardener/gardener/pkg/gardenadm/cmd/token/list"
)

// NewCommand creates a new cobra.Command.
func NewCommand(globalOpts *cmd.Options) *cobra.Command {
	opts := &Options{Options: globalOpts}

	cmd := &cobra.Command{
		Use:   "token",
		Short: "Manage bootstrap and discovery tokens for gardenadm join",
		Long:  "Manage bootstrap and discovery tokens for gardenadm join",
	}

	opts.addFlags(cmd.Flags())

	cmd.AddCommand(list.NewCommand(globalOpts))
	cmd.AddCommand(generate.NewCommand(globalOpts))
	cmd.AddCommand(create.NewCommand(globalOpts))
	cmd.AddCommand(delete.NewCommand(globalOpts))

	return cmd
}
