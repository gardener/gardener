// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package create

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/pflag"

	"github.com/gardener/gardener/pkg/gardenadm/cmd"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/kubernetes/bootstraptoken"
)

// Options contains options for this command.
type Options struct {
	*cmd.Options
	// Token contains the token ID and secret.
	Token Token
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

// Token contains the token ID and secret.
type Token struct {
	// ID is the token id.
	ID string
	// Secret is the token secret.
	Secret string
	// Combined is the token in the form '<id>.<secret>'.
	Combined string
}

// ParseArgs parses the arguments to the options.
func (o *Options) ParseArgs(args []string) error {
	if len(args) > 0 {
		o.Token.Combined = strings.TrimSpace(args[0])
	}

	if o.Token.Combined == "" {
		tokenID, err := utils.GenerateRandomStringFromCharset(6, bootstraptoken.CharSet)
		if err != nil {
			return fmt.Errorf("failed computing random token ID: %w", err)
		}

		tokenSecret, err := utils.GenerateRandomStringFromCharset(16, bootstraptoken.CharSet)
		if err != nil {
			return fmt.Errorf("failed computing random token secret: %w", err)
		}

		o.Token.Combined = fmt.Sprintf("%s.%s", tokenID, tokenSecret)
	}

	return nil
}

// Validate validates the options.
func (o *Options) Validate() error {
	if len(o.Token.Combined) == 0 {
		return fmt.Errorf("must provide a token to create")
	}

	if !bootstraptoken.ValidBootstrapTokenRegex.MatchString(o.Token.Combined) {
		return fmt.Errorf("provided token %q does not match the expected format %q", o.Token.Combined, bootstraptoken.ValidBootstrapTokenRegex.String())
	}

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
func (o *Options) Complete() error {
	split := strings.Split(o.Token.Combined, ".")
	if len(split) != 2 {
		return fmt.Errorf("token must be of the form %q, but got %q", bootstraptoken.ValidBootstrapTokenRegex, o.Token.Combined)
	}
	o.Token.ID, o.Token.Secret = split[0], split[1]

	return nil
}

func (o *Options) addFlags(fs *pflag.FlagSet) {
	fs.StringVarP(&o.Description, "description", "d", "Used for joining nodes via `gardenadm join`", "Description for the bootstrap token")
	fs.DurationVarP(&o.Validity, "validity", "", time.Hour, "Validity duration of the bootstrap token. Minimum is 10m, maximum is 24h.")
	fs.BoolVarP(&o.PrintJoinCommand, "print-join-command", "j", false, "Instead of only printing the token, print the full machine-readable `gardenadm join` command that can be copied and ran on a machine that should join the cluster")
	fs.StringVarP(&o.WorkerPoolName, "worker-pool-name", "w", "worker", "Name of the worker pool to use for the join command.")
}
