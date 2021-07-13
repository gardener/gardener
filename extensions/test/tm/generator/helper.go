// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
func MarshalAndWriteConfig(filepath string, config interface{}) error {
	raw, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("unable to parse config: %w", err)
	}

	if err := os.MkdirAll(path.Dir(filepath), os.ModePerm); err != nil {
		return fmt.Errorf("unable to create path %s: %w", path.Dir(filepath), err)
	}
	if err := os.WriteFile(filepath, raw, os.ModePerm); err != nil {
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
