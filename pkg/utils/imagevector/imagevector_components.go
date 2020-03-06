// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package imagevector

import (
	"io"
	"os"

	"gopkg.in/yaml.v2"
)

const (
	// ComponentOverrideEnv is the name of the environment variable for image vector overrides of components deployed
	// by Gardener.
	ComponentOverrideEnv = "IMAGEVECTOR_OVERWRITE_COMPONENTS"
)

// ReadComponentOverwrite reads an ComponentImageVector from the given io.Reader.
func ReadComponentOverwrite(r io.Reader) (ComponentImageVectors, error) {
	data := struct {
		Components []ComponentImageVector `json:"components" yaml:"components"`
	}{}

	if err := yaml.NewDecoder(r).Decode(&data); err != nil {
		return nil, err
	}

	out := make(ComponentImageVectors, len(data.Components))
	for _, component := range data.Components {
		out[component.Name] = component.ImageVectorOverwrite
	}

	return out, nil
}

// ReadComponentOverwriteFile reads an ComponentImageVector from the file with the given name.
func ReadComponentOverwriteFile(name string) (ComponentImageVectors, error) {
	file, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return ReadComponentOverwrite(file)
}
