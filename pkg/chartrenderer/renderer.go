// SPDX-FileCopyrightText: 2018 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package chartrenderer

import (
	"k8s.io/helm/pkg/manifest"
)

// Interface is an interface for rendering Helm Charts from path, name, namespace and values.
type Interface interface {
	Render(chartPath, releaseName, namespace string, values interface{}) (*RenderedChart, error)
	RenderArchive(archive []byte, releaseName, namespace string, values interface{}) (*RenderedChart, error)
}

// RenderedChart holds a map of rendered templates file with template file name as key and
// rendered template as value.
type RenderedChart struct {
	ChartName string
	Manifests []manifest.Manifest
}
