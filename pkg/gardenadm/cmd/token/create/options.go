// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package create

import (
	"fmt"
	"strings"

	"github.com/spf13/pflag"

	"github.com/gardener/gardener/pkg/gardenadm/cmd"
	tokenutils "github.com/gardener/gardener/pkg/gardenadm/cmd/token/utils"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/kubernetes/bootstraptoken"
)

// Options contains options for this command.
type Options struct {
	*cmd.Options
	// CreateOptions are the options for creating a bootstrap token.
	CreateOptions *tokenutils.Options

	// Token contains the token ID and secret.
	Token Token
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

	return o.CreateOptions.ParseArgs(args)
}

// Validate validates the options.
func (o *Options) Validate() error {
	if len(o.Token.Combined) == 0 {
		return fmt.Errorf("must provide a token to create")
	}

	if !bootstraptoken.ValidBootstrapTokenRegex.MatchString(o.Token.Combined) {
		return fmt.Errorf("provided token %q does not match the expected format %q", o.Token.Combined, bootstraptoken.ValidBootstrapTokenRegex.String())
	}

	return o.CreateOptions.Validate()
}

// Complete completes the options.
func (o *Options) Complete() error {
	split := strings.Split(o.Token.Combined, ".")
	if len(split) != 2 {
		return fmt.Errorf("token must be of the form %q, but got %q", bootstraptoken.ValidBootstrapTokenRegex, o.Token.Combined)
	}
	o.Token.ID, o.Token.Secret = split[0], split[1]

	return o.CreateOptions.Complete()
}

func (o *Options) addFlags(fs *pflag.FlagSet) {
	o.CreateOptions.AddFlags(fs)
}
