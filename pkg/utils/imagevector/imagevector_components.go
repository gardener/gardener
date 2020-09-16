// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

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
