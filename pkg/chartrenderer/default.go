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
	"encoding/json"
	"fmt"
	"path"
	"strings"

	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"

	"k8s.io/helm/pkg/chartutil"
	"k8s.io/helm/pkg/engine"
	"k8s.io/helm/pkg/manifest"
	chartapi "k8s.io/helm/pkg/proto/hapi/chart"
	"k8s.io/helm/pkg/tiller"
	"k8s.io/helm/pkg/timeconv"
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

	capabilities, err := DiscoverCapabilities(disc)
	if err != nil {
		return nil, err
	}

	return &chartRenderer{
		renderer:     engine.New(),
		capabilities: capabilities,
	}, nil
}

// New creates a new chart renderer with the given Engine and Capabilities.
func New(engine *engine.Engine, capabilities *chartutil.Capabilities) Interface {
	return &chartRenderer{
		renderer:     engine,
		capabilities: capabilities,
	}
}

// DiscoverCapabilities discovers the capabilities required for chart renderers using the given
// DiscoveryInterface.
func DiscoverCapabilities(disc discovery.DiscoveryInterface) (*chartutil.Capabilities, error) {
	sv, err := disc.ServerVersion()
	if err != nil {
		return nil, fmt.Errorf("failed to get kubernetes server version %v", err)
	}

	return &chartutil.Capabilities{KubeVersion: sv}, nil
}

// Render loads the chart from the given location <chartPath> and calls the Render() function
// to convert it into a ChartRelease object.
func (r *chartRenderer) Render(chartPath, releaseName, namespace string, values map[string]interface{}) (*RenderedChart, error) {
	chart, err := chartutil.Load(chartPath)
	if err != nil {
		return nil, fmt.Errorf("can't create load chart from path %s:, %s", chartPath, err)
	}
	return r.renderRelease(chart, releaseName, namespace, values)
}

// RenderArchive loads the chart from the given location <chartPath> and calls the Render() function
// to convert it into a ChartRelease object.
func (r *chartRenderer) RenderArchive(archive []byte, releaseName, namespace string, values map[string]interface{}) (*RenderedChart, error) {
	chart, err := chartutil.LoadArchive(bytes.NewReader(archive))
	if err != nil {
		return nil, fmt.Errorf("can't create load chart from archive: %s", err)
	}
	return r.renderRelease(chart, releaseName, namespace, values)
}

func (r *chartRenderer) renderRelease(chart *chartapi.Chart, releaseName, namespace string, values map[string]interface{}) (*RenderedChart, error) {
	chartName := chart.GetMetadata().GetName()

	parsedValues, err := json.Marshal(values)
	if err != nil {
		return nil, fmt.Errorf("can't parse variables for chart %s: ,%s", chartName, err)
	}
	chartConfig := &chartapi.Config{Raw: string(parsedValues)}

	err = chartutil.ProcessRequirementsEnabled(chart, chartConfig)
	if err != nil {
		return nil, fmt.Errorf("can't process requirements for chart %s: ,%s", chartName, err)
	}
	err = chartutil.ProcessRequirementsImportValues(chart)
	if err != nil {
		return nil, fmt.Errorf("can't process requirements for import values for chart %s: ,%s", chartName, err)
	}

	caps := r.capabilities
	revision := 1
	ts := timeconv.Now()
	options := chartutil.ReleaseOptions{
		Name:      releaseName,
		Time:      ts,
		Namespace: namespace,
		Revision:  revision,
		IsInstall: true,
	}

	valuesToRender, err := chartutil.ToRenderValuesCaps(chart, chartConfig, options, caps)
	if err != nil {
		return nil, err
	}
	return r.renderResources(chart, valuesToRender)
}

func (r *chartRenderer) renderResources(ch *chartapi.Chart, values chartutil.Values) (*RenderedChart, error) {
	files, err := r.renderer.Render(ch, values)
	if err != nil {
		return nil, err
	}

	// Remove NODES.txt and partials
	for k := range files {
		if strings.HasSuffix(k, notesFileSuffix) || strings.HasPrefix(path.Base(k), "_") {
			delete(files, k)
		}
	}

	manifests := manifest.SplitManifests(files)
	manifests = tiller.SortByKind(manifests)

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
