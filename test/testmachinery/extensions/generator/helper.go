// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

// package tm contains the generators for provider specific configuration

package generator

import (
	"errors"
	"fmt"
	"os"
	"path"

	"sigs.k8s.io/yaml"
)

// MarshalAndWriteConfig marshals the provided config and write is as a file to the provided path
func MarshalAndWriteConfig(filepath string, config any) error {
	raw, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("unable to parse config: %w", err)
	}

	if err := os.MkdirAll(path.Dir(filepath), 0750); err != nil {
		return fmt.Errorf("unable to create path %s: %w", path.Dir(filepath), err)
	}
	if err := os.WriteFile(filepath, raw, 0600); err != nil {
		return fmt.Errorf("unable to write config to %s: %w", filepath, err)
	}

	return nil
}

// ValidateString validates if a string is defined
func ValidateString(s *string) error {
	if s == nil || *s == "" {
		return errors.New("empty string")
	}
	return nil
}
