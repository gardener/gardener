// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

// package tm contains the generators for provider specific configuration

package generator

import (
	"io/ioutil"
	"os"
	"path"

	"github.com/pkg/errors"
	"sigs.k8s.io/yaml"
)

// MarshalAndWriteConfig marshals the provided config and write is as a file to the provided path
func MarshalAndWriteConfig(filepath string, config interface{}) error {
	raw, err := yaml.Marshal(config)
	if err != nil {
		return errors.Wrap(err, "unable to parse config")
	}

	if err := os.MkdirAll(path.Dir(filepath), os.ModePerm); err != nil {
		return errors.Wrapf(err, "unable to create path %s", path.Dir(filepath))
	}
	if err := ioutil.WriteFile(filepath, raw, os.ModePerm); err != nil {
		return errors.Wrapf(err, "unable to write config to %s", filepath)
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
