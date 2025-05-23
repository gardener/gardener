// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"flag"
	"fmt"
	"os"

	"github.com/hashicorp/go-multierror"
	"github.com/spf13/viper"
	"k8s.io/apimachinery/pkg/util/sets"
)

// ParseConfigForFlags tries to read configuration from the specified config file
// and applies its values to the non specified flags.
func ParseConfigForFlags(configFilePath string, fs *flag.FlagSet) error {
	if configFilePath == "" {
		return nil
	}

	if _, err := os.Stat(configFilePath); err != nil {
		return err
	}

	viper.SetConfigFile(configFilePath)

	if err := viper.ReadInConfig(); err != nil {
		return err
	}

	return applyConfig(fs)
}

// applyConfig applies configuration values that are specified
// in the configuration file to the specific flags.
// Only flags are updated that are not defined by command line.
func applyConfig(fs *flag.FlagSet) error {
	var allErrs *multierror.Error
	definedFlags := sets.Set[string]{}

	// get all flags that are defined by command line
	fs.Visit(func(f *flag.Flag) {
		definedFlags.Insert(f.Name)
	})

	fs.VisitAll(func(f *flag.Flag) {
		if definedFlags.Has(f.Name) {
			return
		}

		if err := f.Value.Set(viper.GetString(f.Name)); err != nil {
			allErrs = multierror.Append(allErrs, fmt.Errorf("unable to set configuration for flag %s: %w", f.Name, err))
		}
	})

	return allErrs.ErrorOrNil()
}
