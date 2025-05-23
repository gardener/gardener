// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package chartrenderer

import (
	"embed"

	"helm.sh/helm/v3/pkg/releaseutil"
)

// Interface is an interface for rendering Helm Charts from path, name, namespace and values.
type Interface interface {
	RenderEmbeddedFS(embeddedFS embed.FS, chartPath, releaseName, namespace string, values any) (*RenderedChart, error)
	RenderArchive(archive []byte, releaseName, namespace string, values any) (*RenderedChart, error)
}

// RenderedChart holds a map of rendered templates file with template file name as key and
// rendered template as value.
type RenderedChart struct {
	ChartName string
	Manifests []releaseutil.Manifest
}
