// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package chartrenderer

import (
	"bytes"
	"fmt"

	"k8s.io/helm/pkg/manifest"
)

// ChartRenderer is an interface for rendering Helm Charts from path, name, namespace and values.
type ChartRenderer interface {
	Render(chartPath, releaseName, namespace string, values map[string]interface{}) (*RenderedChart, error)
	RenderArchive(archive []byte, releaseName, namespace string, values map[string]interface{}) (*RenderedChart, error)
}

// RenderedChart holds a map of rendered templates file with template file name as key and
// rendered template as value.
type RenderedChart struct {
	ChartName string
	Manifests []manifest.Manifest
}

// Manifest returns the manifest of the rendered chart as byte array.
func (c *RenderedChart) Manifest() []byte {
	// Aggregate all valid manifests into one big doc.
	b := bytes.NewBuffer(nil)

	for _, mf := range c.Manifests {
		b.WriteString("\n---\n# Source: " + mf.Name + "\n")
		b.WriteString(mf.Content)
	}
	return b.Bytes()
}

// Files returns all rendered manifests mapping their names to their content.
func (c *RenderedChart) Files() map[string]string {
	var files = make(map[string]string)
	for _, manifest := range c.Manifests {
		files[manifest.Name] = manifest.Content
	}
	return files
}

// FileContent returns explicitly the content of the provided <filename>.
func (c *RenderedChart) FileContent(filename string) string {
	for _, mf := range c.Manifests {
		if mf.Name == fmt.Sprintf("%s/templates/%s", c.ChartName, filename) {
			return mf.Content
		}
	}
	return ""
}
