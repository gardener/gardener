// Copyright 2018 The Gardener Authors.
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

// ChartRenderer is an interface for rendering Helm Charts from path, name, namespace and values.
type ChartRenderer interface {
	Render(chartPath, releaseName, namespace string, values map[string]interface{}) (*RenderedChart, error)
}

// RenderedChart holds a map of rendered templates file with template file name as key and
// rendered template as value.
type RenderedChart struct {
	Files map[string]string
}
