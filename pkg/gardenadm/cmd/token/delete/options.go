// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package delete

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/spf13/pflag"
	bootstraptokenapi "k8s.io/cluster-bootstrap/token/api"

	"github.com/gardener/gardener/pkg/gardenadm/cmd"
	"github.com/gardener/gardener/pkg/utils/kubernetes/bootstraptoken"
)

var (
	validBootstrapTokenID     = regexp.MustCompile(`^[a-z0-9]{6}$`)
	validBootstrapTokenSecret = regexp.MustCompile(`^bootstrap-token-[a-z0-9]{6}$`)
)

// Options contains options for this command.
type Options struct {
	*cmd.Options
	// TokenValues is the name of the Secret holding the bootstrap token, or the full token of the form
	// "[a-z0-9]{6}.[a-z0-9]{16}", or the token ID of the form "[a-z0-9]{6}" to delete.
	TokenValues []string
	// TokenIDs are the actual token IDs parsed from the token values.
	TokenIDs []string
}

// ParseArgs parses the arguments to the options.
func (o *Options) ParseArgs(args []string) error {
	for _, arg := range args {
		o.TokenValues = append(o.TokenValues, strings.TrimSpace(arg))
	}

	return nil
}

// Validate validates the options.
func (o *Options) Validate() error {
	if len(o.TokenValues) == 0 {
		return fmt.Errorf("must provide at least one token value to delete")
	}

	for _, tokenValue := range o.TokenValues {
		if !bootstraptoken.ValidBootstrapTokenRegex.Match([]byte(tokenValue)) &&
			!validBootstrapTokenID.Match([]byte(tokenValue)) &&
			!validBootstrapTokenSecret.Match([]byte(tokenValue)) {
			return fmt.Errorf("invalid token value %q - accepted formats are %q or %q or %q", tokenValue, bootstraptoken.ValidBootstrapTokenRegex, validBootstrapTokenID, validBootstrapTokenSecret)
		}
	}

	return nil
}

// Complete completes the options.
func (o *Options) Complete() error {
	for _, tokenValue := range o.TokenValues {
		switch {
		case bootstraptoken.ValidBootstrapTokenRegex.Match([]byte(tokenValue)):
			o.TokenIDs = append(o.TokenIDs, strings.Split(tokenValue, ".")[0])
		case validBootstrapTokenID.Match([]byte(tokenValue)):
			o.TokenIDs = append(o.TokenIDs, tokenValue)
		case validBootstrapTokenSecret.Match([]byte(tokenValue)):
			o.TokenIDs = append(o.TokenIDs, strings.TrimPrefix(tokenValue, bootstraptokenapi.BootstrapTokenSecretPrefix))
		}
	}

	return nil
}

func (o *Options) addFlags(_ *pflag.FlagSet) {}
