// Copyright 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"path/filepath"
	"strings"

	helmchart "helm.sh/helm/v3/pkg/chart"
	helmloader "helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/engine"
	"helm.sh/helm/v3/pkg/releaseutil"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	"k8s.io/helm/pkg/ignore"
)

const notesFileSuffix = "NOTES.txt"

// chartRenderer is a struct which contains the chart render engine and a Kubernetes client.
// The chart render is used to render the Helm charts into a RenderedChart struct from which the
// resulting manifest can be generated.
type chartRenderer struct {
	renderer     *engine.Engine
	capabilities *chartutil.Capabilities
}

// NewForConfig creates a new ChartRenderer object. It requires a Kubernetes client as input which will be
// injected in the Tiller environment.
func NewForConfig(cfg *rest.Config) (Interface, error) {
	disc, err := discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		return nil, err
	}

	sv, err := disc.ServerVersion()
	if err != nil {
		return nil, fmt.Errorf("failed to get kubernetes server version %w", err)
	}

	return NewWithServerVersion(sv), nil
}

// NewWithServerVersion creates a new chart renderer with the given server version.
func NewWithServerVersion(serverVersion *version.Info) Interface {
	return &chartRenderer{
		renderer: &engine.Engine{},
		capabilities: &chartutil.Capabilities{KubeVersion: chartutil.KubeVersion{
			Version: serverVersion.GitVersion,
			Major:   serverVersion.Major,
			Minor:   serverVersion.Minor,
		}},
	}
}

// RenderArchive loads the chart from the given location <chartPath> and calls the renderRelease() function
// to convert it into a ChartRelease object.
func (r *chartRenderer) RenderArchive(archive []byte, releaseName, namespace string, values interface{}) (*RenderedChart, error) {
	chart, err := helmloader.LoadArchive(bytes.NewReader(archive))
	if err != nil {
		return nil, fmt.Errorf("can't load chart from archive: %s", err)
	}
	return r.renderRelease(chart, releaseName, namespace, values)
}

// RenderEmbeddedFS loads the chart from the given embed.FS and calls the renderRelease() function
// to convert it into a ChartRelease object.
func (r *chartRenderer) RenderEmbeddedFS(embeddedFS embed.FS, chartPath, releaseName, namespace string, values interface{}) (*RenderedChart, error) {
	chart, err := loadEmbeddedFS(embeddedFS, chartPath)
	if err != nil {
		return nil, fmt.Errorf("can't load chart %q from embedded file system: %w", chartPath, err)
	}
	return r.renderRelease(chart, releaseName, namespace, values)
}

func (r *chartRenderer) renderRelease(chart *helmchart.Chart, releaseName, namespace string, values interface{}) (*RenderedChart, error) {
	parsedValues, err := json.Marshal(values)
	if err != nil {
		return nil, fmt.Errorf("can't parse variables for chart %s: ,%s", chart.Metadata.Name, err)
	}
	valuesCopy, err := chartutil.ReadValues(parsedValues)
	if err != nil {
		return nil, fmt.Errorf("can't read variables for chart %s: ,%s", chart.Metadata.Name, err)
	}

	if err := chartutil.ProcessDependencies(chart, valuesCopy); err != nil {
		return nil, fmt.Errorf("can't process chart %s: ,%s", chart.Metadata.Name, err)
	}

	caps := r.capabilities
	revision := 1
	options := chartutil.ReleaseOptions{
		Name:      releaseName,
		Namespace: namespace,
		Revision:  revision,
		IsInstall: true,
	}

	valuesToRender, err := chartutil.ToRenderValues(chart, valuesCopy, options, caps)
	if err != nil {
		return nil, err
	}
	return r.renderResources(chart, valuesToRender)
}

func (r *chartRenderer) renderResources(ch *helmchart.Chart, values chartutil.Values) (*RenderedChart, error) {
	files, err := r.renderer.Render(ch, values)
	if err != nil {
		return nil, err
	}

	// Remove NOTES.txt and partials
	for k := range files {
		if strings.HasSuffix(k, notesFileSuffix) || strings.HasPrefix(path.Base(k), "_") {
			delete(files, k)
		}
	}

	_, manifests, err := releaseutil.SortManifests(files, chartutil.DefaultVersionSet, releaseutil.InstallOrder)
	if err != nil {
		return nil, err
	}

	return &RenderedChart{
		ChartName: ch.Metadata.Name,
		Manifests: manifests,
	}, nil
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

// AsSecretData returns all rendered manifests that is capable for used as data of a secret
func (c *RenderedChart) AsSecretData() map[string][]byte {
	data := make(map[string][]byte, len(c.Files()))
	for fileName, fileContent := range c.Files() {
		if len(fileContent) != 0 {
			key := strings.ReplaceAll(fileName, "/", "_")
			data[key] = []byte(fileContent)
		}
	}
	return data
}

// loadEmbeddedFS is a copy of chartutil.LoadDir with the difference that it uses an embed.FS.
func loadEmbeddedFS(embeddedFS embed.FS, chartPath string) (*helmchart.Chart, error) {
	var (
		rules = ignore.Empty()
		files []*helmloader.BufferedFile
	)

	if helmIgnore, err := embeddedFS.ReadFile(filepath.Join(chartPath, ignore.HelmIgnore)); err == nil {
		r, err := ignore.Parse(bytes.NewReader(helmIgnore))
		if err != nil {
			return nil, err
		}
		rules = r
	}

	if err := fs.WalkDir(embeddedFS, chartPath, func(path string, dirEntry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		fileInfo, err := dirEntry.Info()
		if err != nil {
			return err
		}

		normalizedPath := strings.TrimPrefix(strings.TrimPrefix(path, chartPath), "/")
		if normalizedPath == "" {
			// No need to process top level. Avoid bug with helmignore .* matching
			// empty names. See issue 1779.
			return nil
		}
		// Normalize to / since it will also work on Windows
		normalizedPath = filepath.ToSlash(normalizedPath)

		if dirEntry.IsDir() {
			// Directory-based ignore rules should involve skipping the entire
			// contents of that directory.
			if rules.Ignore(normalizedPath, fileInfo) {
				return filepath.SkipDir
			}
			return nil
		}

		// If a .helmignore file matches, skip this file.
		if rules.Ignore(normalizedPath, fileInfo) {
			return nil
		}

		// Irregular files include devices, sockets, and other uses of files that
		// are not regular files. In Go they have a file mode type bit set.
		// See https://golang.org/pkg/os/#FileMode for examples.
		if !fileInfo.Mode().IsRegular() {
			return fmt.Errorf("cannot load irregular file %s as it has file mode type bits set", path)
		}

		data, err := embeddedFS.ReadFile(path)
		if err != nil {
			return fmt.Errorf("error reading %s: %s", normalizedPath, err)
		}
		files = append(files, &helmloader.BufferedFile{Name: normalizedPath, Data: data})

		return nil
	}); err != nil {
		return nil, err
	}

	return helmloader.LoadFiles(files)
}
