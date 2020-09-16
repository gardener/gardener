// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package terraformer

import (
	"fmt"

	"github.com/gardener/gardener/pkg/chartrenderer"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/helm/pkg/manifest"
)

func mkManifest(name string, content string) manifest.Manifest {
	return manifest.Manifest{
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
				Manifests: []manifest.Manifest{
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
