// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package imagevector

import (
	"os"

	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/yaml"
)

const (
	// ComponentOverrideEnv is the name of the environment variable for image vector overrides of components deployed
	// by Gardener.
	ComponentOverrideEnv = "IMAGEVECTOR_OVERWRITE_COMPONENTS"
)

// ReadComponentOverwrite reads an ComponentImageVector from the given io.Reader.
func ReadComponentOverwrite(buf []byte) (ComponentImageVectors, error) {
	data := struct {
		Components []ComponentImageVector `json:"components" yaml:"components"`
	}{}

	if err := yaml.Unmarshal(buf, &data); err != nil {
		return nil, err
	}

	componentImageVectors := make(ComponentImageVectors, len(data.Components))
	for _, component := range data.Components {
		componentImageVectors[component.Name] = component.ImageVectorOverwrite
	}

	if errs := ValidateComponentImageVectors(componentImageVectors, field.NewPath("components")); len(errs) > 0 {
		return nil, errs.ToAggregate()
	}

	return componentImageVectors, nil
}

// ReadComponentOverwriteFile reads an ComponentImageVector from the file with the given name.
func ReadComponentOverwriteFile(name string) (ComponentImageVectors, error) {
	buf, err := os.ReadFile(name) // #nosec: G304 -- ImageVectorOverwrite is a feature. In reality files can be read from the Pod's file system only.
	if err != nil {
		return nil, err
	}

	return ReadComponentOverwrite(buf)
}
