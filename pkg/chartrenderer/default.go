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

	"k8s.io/client-go/kubernetes"

	"k8s.io/helm/pkg/chartutil"
	"k8s.io/helm/pkg/engine"
	chartapi "k8s.io/helm/pkg/proto/hapi/chart"
	"k8s.io/helm/pkg/timeconv"
)

const notesFileSuffix = "NOTES.txt"

// DefaultChartRenderer is a struct which contains the chart render engine and a Kubernetes client.
// The chart render is used to render the Helm charts into a RenderedChart struct from which the
// resulting manifest can be generated.
type DefaultChartRenderer struct {
	client       kubernetes.Interface
	renderer     *engine.Engine
	capabilities *chartutil.Capabilities
}

// New creates a new DefaultChartRenderer object. It requires a Kubernetes client as input which will be
// injected in the Tiller environment.
func New(client kubernetes.Interface) (ChartRenderer, error) {
	sv, err := client.Discovery().ServerVersion()
	if err != nil {
		return nil, fmt.Errorf("failed to get kubernetes server version %v", err)
	}
	return &DefaultChartRenderer{
		client:       client,
		renderer:     engine.New(),
		capabilities: &chartutil.Capabilities{KubeVersion: sv},
	}, nil
}

// Render loads the chart from the given location <chartPath> and calls the Render() function
// to convert it into a ChartRelease object.
func (r *DefaultChartRenderer) Render(chartPath, releaseName, namespace string, values map[string]interface{}) (*RenderedChart, error) {
	chart, err := chartutil.Load(chartPath)
	if err != nil {
		return nil, fmt.Errorf("can't create load chart from path %s:, %s", chartPath, err)
	}
	return r.renderRelease(chart, releaseName, namespace, values)
}

// RenderArchive loads the chart from the given location <chartPath> and calls the Render() function
// to convert it into a ChartRelease object.
func (r *DefaultChartRenderer) RenderArchive(archive []byte, releaseName, namespace string, values map[string]interface{}) (*RenderedChart, error) {
	chart, err := chartutil.LoadArchive(bytes.NewReader(archive))
	if err != nil {
		return nil, fmt.Errorf("can't create load chart from archive: %s", err)
	}
	return r.renderRelease(chart, releaseName, namespace, values)
}

// Manifest returns the manifest of the rendered chart as byte array.
func (c *RenderedChart) Manifest() []byte {
	// Aggregate all valid manifests into one big doc.
	b := bytes.NewBuffer(nil)

	for k, v := range c.Files {
		b.WriteString("\n---\n# Source: " + k + "\n")
		b.WriteString(v)
	}
	return b.Bytes()
}

// ManifestAsString returns the manifest of the rendered chart as string.
func (c *RenderedChart) ManifestAsString() string {
	return string(c.Manifest())
}

// FileContent returns explicitly the content of the provided <filename>.
func (c *RenderedChart) FileContent(filename string) string {
	if content, ok := c.Files[fmt.Sprintf("%s/templates/%s", c.ChartName, filename)]; ok {
		return content
	}
	return ""
}

func (r *DefaultChartRenderer) renderRelease(chart *chartapi.Chart, releaseName, namespace string, values map[string]interface{}) (*RenderedChart, error) {
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

func (r *DefaultChartRenderer) renderResources(ch *chartapi.Chart, values chartutil.Values) (*RenderedChart, error) {
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

	return &RenderedChart{
		ChartName: ch.Metadata.Name,
		Files:     files,
	}, nil
}
