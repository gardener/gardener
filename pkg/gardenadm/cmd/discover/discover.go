// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package discover

import (
	"github.com/spf13/cobra"

	"github.com/gardener/gardener/pkg/gardenadm/cmd"
	"github.com/gardener/gardener/pkg/gardenadm/cmd/discover/existing"
	dnew "github.com/gardener/gardener/pkg/gardenadm/cmd/discover/new"
)

// NewCommand creates a new cobra.Command.
func NewCommand(globalOpts *cmd.Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "discover",
		Short: "Conveniently download Gardener configuration resources from an existing garden cluster",
		Long:  "Conveniently download Gardener configuration resources from an existing garden cluster (CloudProfile, ControllerRegistrations, ControllerDeployments, etc.)",
	}

	cmd.AddCommand(dnew.NewCommand(globalOpts))
	cmd.AddCommand(existing.NewCommand(globalOpts))

	return cmd
}
