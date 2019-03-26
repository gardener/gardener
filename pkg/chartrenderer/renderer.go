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
	"k8s.io/helm/pkg/manifest"
)

// Interface is an interface for rendering Helm Charts from path, name, namespace and values.
type Interface interface {
	Render(chartPath, releaseName, namespace string, values map[string]interface{}) (*RenderedChart, error)
	RenderArchive(archive []byte, releaseName, namespace string, values map[string]interface{}) (*RenderedChart, error)
}

// RenderedChart holds a map of rendered templates file with template file name as key and
// rendered template as value.
type RenderedChart struct {
	ChartName string
	Manifests []manifest.Manifest
}
