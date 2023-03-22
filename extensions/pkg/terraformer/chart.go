// Copyright 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
