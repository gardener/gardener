// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package terraformer

import (
	"fmt"

	"github.com/gardener/gardener/pkg/chartrenderer"
)

// TerraformFiles contains all files necessary for initializing a Terraformer.
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
//
// It errors if a file is not contained in the chart.
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
