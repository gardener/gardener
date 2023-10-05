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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"helm.sh/helm/v3/pkg/releaseutil"
)

func mkManifest(name string, content string) releaseutil.Manifest {
	return releaseutil.Manifest{
		Name:    fmt.Sprintf("/templates/%s", name),
		Content: content,
	}
}

var _ = Describe("Chart", func() {
	Describe("#ExtractTerraformFiles", func() {
		It("should extract the terraform files", func() {
			var (
				mainContent      = "main"
				variablesContent = "variables"
				tfVarsContent    = "tfVars"
			)

			files, err := ExtractTerraformFiles(&chartrenderer.RenderedChart{
				Manifests: []releaseutil.Manifest{
					mkManifest(MainKey, mainContent),
					mkManifest(VariablesKey, variablesContent),
					mkManifest(TFVarsKey, tfVarsContent),
				},
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(files).To(Equal(&TerraformFiles{
				Main:      mainContent,
				Variables: variablesContent,
				TFVars:    []byte(tfVarsContent),
			}))
		})
	})
})
