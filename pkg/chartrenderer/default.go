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

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path"
	"strings"

	"k8s.io/helm/pkg/engine"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"k8s.io/client-go/discovery"
	"k8s.io/helm/pkg/chartutil"
	chartapi "k8s.io/helm/pkg/proto/hapi/chart"
	"k8s.io/helm/pkg/timeconv"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const notesFileSuffix = "NOTES.txt"

// DefaultChartRenderer is a struct which contains the chart render engine and a Kubernetes client.
// The chart render is used to render the Helm charts into a RenderedChart struct from which the
// resulting manifest can be generated.
type DefaultChartRenderer struct {
	client   kubernetes.Client
	renderer *engine.Engine
}

// New creates a new DefaultChartRenderer object. It requires a Kubernetes client as input which will be
// injected in the Tiller environment.
func New(client kubernetes.Client) ChartRenderer {
	return &DefaultChartRenderer{
		client:   client,
		renderer: engine.New(),
	}
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

// Manifest retrurns the manifest of the rendered chart as byte arrray
func (c *RenderedChart) Manifest() []byte {
	// Aggregate all valid manifests into one big doc.
	b := bytes.NewBuffer(nil)

	for k, v := range c.Files {
		b.WriteString("\n---\n# Source: " + k + "\n")
		b.WriteString(v)
	}
	return b.Bytes()
}

// ManifestAsString retrurns the manifest of the rendered chart as string
func (c *RenderedChart) ManifestAsString() string {
	return string(c.Manifest())
}

// GetVersionSet retrieves a set of available k8s API versions
func GetVersionSet(client discovery.ServerGroupsInterface) (chartutil.VersionSet, error) {
	groups, err := client.ServerGroups()
	if err != nil {
		return chartutil.DefaultVersionSet, err
	}

	// FIXME: The Kubernetes test fixture for cli appears to always return nil
	// for calls to Discovery().ServerGroups(). So in this case, we return
	// the default API list. This is also a safe value to return in any other
	// odd-ball case.
	if groups == nil {
		return chartutil.DefaultVersionSet, nil
	}

	versions := metav1.ExtractGroupVersions(groups)
	return chartutil.NewVersionSet(versions...), nil
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

	caps, err := capabilities(r.client.GetClientset().Discovery())
	if err != nil {
		return nil, err
	}

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
		Files: files,
	}, nil
}

// capabilities builds a Capabilities from discovery information.
func capabilities(disc discovery.DiscoveryInterface) (*chartutil.Capabilities, error) {
	sv, err := disc.ServerVersion()
	if err != nil {
		return nil, err
	}
	vs, err := GetVersionSet(disc)
	if err != nil {
		return nil, fmt.Errorf("Could not get apiVersions from Kubernetes: %s", err)
	}
	return &chartutil.Capabilities{
		APIVersions: vs,
		KubeVersion: sv,
	}, nil
}
