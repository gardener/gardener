// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"fmt"
	"time"

	"github.com/spf13/pflag"

	"github.com/gardener/gardener/pkg/gardenadm/cmd"
)

// Options contains options for this command.
type Options struct {
	*cmd.Options
	// Description is the description for the bootstrap token.
	Description string
	// Validity duration of the bootstrap token.
	Validity time.Duration
	// PrintJoinCommand specifies whether to print the full `gardenadm join` command.
	PrintJoinCommand bool
	// WorkerPoolName is the name of the worker pool to use for the join command. If not provided, it is defaulted to
	// 'worker'.
	WorkerPoolName string
}

// ParseArgs parses the arguments to the options.
func (o *Options) ParseArgs(_ []string) error { return nil }

// Validate validates the options.
func (o *Options) Validate() error {
	if minValidity := 10 * time.Minute; o.Validity < minValidity {
		return fmt.Errorf("minimum validity duration is %s", minValidity)
	}
	if maxValidity := 24 * time.Hour; o.Validity > maxValidity {
		return fmt.Errorf("maximum validity duration is %s", maxValidity)
	}

	if o.PrintJoinCommand && len(o.WorkerPoolName) == 0 {
		return fmt.Errorf("must specify a worker pool name when using --print-join-command")
	}

	return nil
}

// Complete completes the options.
func (o *Options) Complete() error { return nil }

// AddFlags adds the flags to the command line flag set.
func (o *Options) AddFlags(fs *pflag.FlagSet) {
	fs.StringVarP(&o.Description, "description", "d", "Used for joining nodes via `gardenadm join`", "Description for the bootstrap token")
	fs.DurationVarP(&o.Validity, "validity", "v", time.Hour, "Validity duration of the bootstrap token")
	fs.BoolVarP(&o.PrintJoinCommand, "print-join-command", "j", false, "Instead of only printing the token, print the full machine-readable `gardenadm join` command that can be copied and ran on a machine that should join the cluster")
	fs.StringVarP(&o.WorkerPoolName, "worker-pool-name", "w", "worker", "Name of the worker pool to use for the join command. If not provided, it is defaulted to 'worker'.")
}
