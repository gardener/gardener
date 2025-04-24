// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package create

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/spf13/pflag"

	"github.com/gardener/gardener/pkg/gardenadm/cmd"
	tokenutils "github.com/gardener/gardener/pkg/gardenadm/cmd/token/utils"
	"github.com/gardener/gardener/pkg/utils"
)

const charSet = "0123456789abcdefghijklmnopqrstuvwxyz"

var validBootstrapToken = regexp.MustCompile(`^[a-z0-9]{6}.[a-z0-9]{16}$`)

// Options contains options for this command.
type Options struct {
	*cmd.Options
	// CreateOptions are the options for creating a bootstrap token.
	CreateOptions *tokenutils.Options
	// Token is the token to create in its full form (<token-id>.<token-secret>).
	Token string
}

// ParseArgs parses the arguments to the options.
func (o *Options) ParseArgs(args []string) error {
	if len(args) > 0 {
		o.Token = strings.TrimSpace(args[0])
	}

	if o.Token == "" {
		tokenID, err := utils.GenerateRandomStringFromCharset(6, charSet)
		if err != nil {
			return fmt.Errorf("failed computing random token ID: %w", err)
		}

		tokenSecret, err := utils.GenerateRandomStringFromCharset(16, charSet)
		if err != nil {
			return fmt.Errorf("failed computing random token secret: %w", err)
		}

		o.Token = tokenID + "." + tokenSecret
	}

	return o.CreateOptions.ParseArgs(args)
}

// Validate validates the options.
func (o *Options) Validate() error {
	if o.Token == "" {
		return fmt.Errorf("must provide a token to create")
	}

	if !validBootstrapToken.Match([]byte(o.Token)) {
		return fmt.Errorf("provided token %q does not match the expected format %q", o.Token, validBootstrapToken.String())
	}

	return o.CreateOptions.Validate()
}

// Complete completes the options.
func (o *Options) Complete() error {
	return o.CreateOptions.Complete()
}

func (o *Options) addFlags(fs *pflag.FlagSet) {
	o.CreateOptions.AddFlags(fs)
}
