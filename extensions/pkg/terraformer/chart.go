// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package terraformer

import (
	"fmt"

	"github.com/gardener/gardener/pkg/chartrenderer"
)

// TerraformFiles contains all files necessary for initializing a Terraformer.
//
// Deprecated: This type is deprecated and will be removed after v1.154 has been released.
type TerraformFiles struct {
	Main      string
	Variables string
	TFVars    []byte
}

func nonEmptyFileContent(release *chartrenderer.RenderedChart, filename string) (string, error) {
	content := release.FileContent(filename)
	if len(content) == 0 {
		return "", fmt.Errorf("empty %s file", filename)
	}

	return content, nil
}

// ExtractTerraformFiles extracts TerraformFiles from the given RenderedChart.
// It errors if a file is not contained in the chart.
//
// Deprecated: This function is deprecated and will be removed after v1.154 has been released.
func ExtractTerraformFiles(release *chartrenderer.RenderedChart) (*TerraformFiles, error) {
	main, err := nonEmptyFileContent(release, MainKey)
	if err != nil {
		return nil, err
	}

	variables, err := nonEmptyFileContent(release, VariablesKey)
	if err != nil {
		return nil, err
	}

	tfVars, err := nonEmptyFileContent(release, TFVarsKey)
	if err != nil {
		return nil, err
	}

	return &TerraformFiles{
		Main:      main,
		Variables: variables,
		TFVars:    []byte(tfVars),
	}, nil
}
