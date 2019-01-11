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

package fake

import (
	"github.com/gardener/gardener/pkg/chartrenderer"
)

// ChartRenderer is a fake renderer for testing
type ChartRenderer struct {
	renderFunc func() (*chartrenderer.RenderedChart, error)
}

// New creates a new Fake chartRenderer
func New(renderFunc func() (*chartrenderer.RenderedChart, error)) chartrenderer.ChartRenderer {
	return &ChartRenderer{
		renderFunc: renderFunc,
	}
}

// Render renders provided chart in struct
func (r *ChartRenderer) Render(chartPath, releaseName, namespace string, values map[string]interface{}) (*chartrenderer.RenderedChart, error) {
	return r.renderFunc()
}

// RenderArchive renders provided chart archive.
func (r *ChartRenderer) RenderArchive(archive []byte, releaseName, namespace string, values map[string]interface{}) (*chartrenderer.RenderedChart, error) {
	return r.renderFunc()
}
