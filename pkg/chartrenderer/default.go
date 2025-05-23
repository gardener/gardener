// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package chartrenderer

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	helmchart "helm.sh/helm/v3/pkg/chart"
	helmloader "helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/engine"
	"helm.sh/helm/v3/pkg/ignore"
	"helm.sh/helm/v3/pkg/releaseutil"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
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
func (r *chartRenderer) RenderArchive(archive []byte, releaseName, namespace string, values any) (*RenderedChart, error) {
	chart, err := helmloader.LoadArchive(bytes.NewReader(archive))
	if err != nil {
		return nil, fmt.Errorf("can't load chart from archive: %s", err)
	}
	return r.renderRelease(chart, releaseName, namespace, values)
}

// RenderEmbeddedFS loads the chart from the given embed.FS and calls the renderRelease() function
// to convert it into a ChartRelease object.
func (r *chartRenderer) RenderEmbeddedFS(embeddedFS embed.FS, chartPath, releaseName, namespace string, values any) (*RenderedChart, error) {
	chart, err := loadEmbeddedFS(embeddedFS, chartPath)
	if err != nil {
		return nil, fmt.Errorf("can't load chart %q from embedded file system: %w", chartPath, err)
	}
	return r.renderRelease(chart, releaseName, namespace, values)
}

func (r *chartRenderer) renderRelease(chart *helmchart.Chart, releaseName, namespace string, values any) (*RenderedChart, error) {
	parsedValues, err := json.Marshal(values)
	if err != nil {
		return nil, fmt.Errorf("failed to parse values for chart %s: %w", chart.Metadata.Name, err)
	}

	valuesCopy, err := chartutil.ReadValues(parsedValues)
	if err != nil {
		return nil, fmt.Errorf("failed to read values for chart %s: %w", chart.Metadata.Name, err)
	}

	if err := chartutil.ProcessDependencies(chart, valuesCopy); err != nil {
		return nil, fmt.Errorf("failed to process chart %s: %w", chart.Metadata.Name, err)
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
		return nil, fmt.Errorf("failed to sort manifests: %w", err)
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

// Files returns a map representing the files associated with the RenderedChart.
// The map has the file names as keys and their content represented as a nested map.
// The nested map has keys in the format of strings.ToLower(manifest.Head.Kind+"/"+manifest.Head.Metadata.Name)
// and values as the content of the corresponding file.
func (c *RenderedChart) Files() map[string]map[string]string {
	var files = make(map[string]map[string]string)

	for _, manifest := range c.Manifests {
		resourceName := getResourceName(manifest)
		if resourceName == "" {
			continue
		}

		if _, ok := files[manifest.Name]; ok {
			files[manifest.Name][resourceName] = manifest.Content
		} else {
			files[manifest.Name] = map[string]string{resourceName: manifest.Content}
		}
	}
	return files
}

// FileContent returns explicitly the content of the provided <filename>.
func (c *RenderedChart) FileContent(filename string) string {
	var fileContent strings.Builder

	for _, mf := range c.Manifests {
		if mf.Name == fmt.Sprintf("%s/templates/%s", c.ChartName, filename) {
			if fileContent.String() != "" {
				// Add "---" to separate different resources
				fileContent.WriteString("\n---\n")
			}

			fileContent.WriteString(mf.Content)
		}
	}
	return fileContent.String()
}

// validConfigKey is used to check if the key is a valid key for a secret data.
var validConfigKey = regexp.MustCompile(`[^-._a-zA-Z0-9]+`)

// AsSecretData returns all rendered manifests that is capable for used as data of a secret
func (c *RenderedChart) AsSecretData() map[string][]byte {
	data := make(map[string][]byte)
	for fileName, fileContent := range c.Files() {
		multipleResources := len(fileContent) > 1
		for resourceName, resourceContent := range fileContent {
			if len(resourceContent) != 0 {
				// Keep old behaviour if there is only one resource in the file.
				key := strings.ReplaceAll(fileName, "/", "_")
				if multipleResources {
					key = strings.TrimSuffix(fileName, ".yaml") + "/" + resourceName + ".yaml"
					// replace all characters with _ which does not match the regex [-._a-zA-Z0-9]+
					key = validConfigKey.ReplaceAllString(key, "_")
				}
				data[key] = []byte(resourceContent)
			}
		}
	}
	return data
}

// loadEmbeddedFS is a copy of helm.sh/helm/v3/pkg/chart/loader.LoadDir with the difference that it uses an embed.FS.
// Keep this func in sync with https://github.com/helm/helm/blob/v3.14.2/pkg/chart/loader/directory.go#L43-L120.
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

func getResourceName(manifest releaseutil.Manifest) string {
	if manifest.Head != nil && manifest.Head.Metadata != nil {
		return strings.ToLower(manifest.Head.Kind + "/" + manifest.Head.Metadata.Name)
	}
	return ""
}
