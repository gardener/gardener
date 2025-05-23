// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package terraformer_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"helm.sh/helm/v3/pkg/releaseutil"

	. "github.com/gardener/gardener/extensions/pkg/terraformer"
	"github.com/gardener/gardener/pkg/chartrenderer"
)

func mkManifest(name string, content string) releaseutil.Manifest {
	return releaseutil.Manifest{
		Name:    "/templates/" + name,
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
