// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package chart

import (
	"context"
	"embed"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/chartrenderer"
	kubernetesclient "github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/imagevector"
)

// Interface represents a Helm chart that can be applied and deleted.
type Interface interface {
	// Apply applies this chart in the given namespace using the given ChartApplier. Before applying the chart,
	// it collects its values, injecting images and merging the given values as needed.
	Apply(context.Context, kubernetesclient.ChartApplier, string, imagevector.ImageVector, string, string, map[string]any) error
	// Render renders this chart in the given namespace using the given chartRenderer. Before rendering the chart,
	// it collects its values, injecting images and merging the given values as needed.
	Render(chartrenderer.Interface, string, imagevector.ImageVector, string, string, map[string]any) (string, []byte, error)
	// Delete deletes this chart's objects from the given namespace.
	Delete(context.Context, client.Client, string) error
}

// Chart represents a Helm chart (and its sub-charts) that can be applied and deleted.
type Chart struct {
	Name       string
	Path       string
	EmbeddedFS embed.FS
	Images     []string
	Objects    []*Object
	SubCharts  []*Chart
}

// Object represents an object deployed by a Chart.
type Object struct {
	Type client.Object
	Name string
}

// Apply applies this chart in the given namespace using the given ChartApplier. Before applying the chart,
// it collects its values, injecting images and merging the given values as needed.
func (c *Chart) Apply(
	ctx context.Context,
	chartApplier kubernetesclient.ChartApplier,
	namespace string,
	imageVector imagevector.ImageVector,
	runtimeVersion, targetVersion string,
	additionalValues map[string]any,
) error {
	// Get values with injected images
	values, err := c.injectImages(imageVector, runtimeVersion, targetVersion)
	if err != nil {
		return err
	}

	// Apply chart
	if err := chartApplier.ApplyFromEmbeddedFS(ctx, c.EmbeddedFS, c.Path, namespace, c.Name, kubernetesclient.Values(utils.MergeMaps(values, additionalValues))); err != nil {
		return fmt.Errorf("could not apply embedded chart '%s' in namespace '%s': %w", c.Name, namespace, err)
	}

	return nil
}

// Render renders this chart in the given namespace using the given chartRenderer. Before rendering the chart,
// it collects its values, injecting images and merging the given values as needed.
func (c *Chart) Render(
	chartRenderer chartrenderer.Interface,
	namespace string,
	imageVector imagevector.ImageVector,
	runtimeVersion, targetVersion string,
	additionalValues map[string]any,
) (
	string,
	[]byte,
	error,
) {
	// Get values with injected images
	values, err := c.injectImages(imageVector, runtimeVersion, targetVersion)
	if err != nil {
		return "", nil, err
	}

	// Render chart
	var rc *chartrenderer.RenderedChart
	rc, err = chartRenderer.RenderEmbeddedFS(c.EmbeddedFS, c.Path, c.Name, namespace, utils.MergeMaps(values, additionalValues))
	if err != nil {
		return "", nil, fmt.Errorf("could not render chart '%s' in namespace '%s': %w", c.Name, namespace, err)
	}

	return rc.ChartName, rc.Manifest(), nil
}

// injectImages collects returns a values map with injected images, including sub-charts.
func (c *Chart) injectImages(
	imageVector imagevector.ImageVector,
	runtimeVersion, targetVersion string,
) (
	map[string]any,
	error,
) {
	// Inject images
	values := make(map[string]any)
	var err error
	if len(c.Images) > 0 {
		values, err = InjectImages(values, imageVector, c.Images, imagevector.RuntimeVersion(runtimeVersion), imagevector.TargetVersion(targetVersion))
		if err != nil {
			return nil, fmt.Errorf("could not inject chart '%s' images: %w", c.Name, err)
		}
	}

	// Add subchart values
	for _, sc := range c.SubCharts {
		scValues, err := sc.injectImages(imageVector, runtimeVersion, targetVersion)
		if err != nil {
			return nil, err
		}
		values[sc.Name] = scValues
	}

	return values, nil
}

// Delete deletes this chart's objects from the given namespace using the given client.
func (c *Chart) Delete(ctx context.Context, client client.Client, namespace string) error {
	// Delete objects
	for _, o := range c.Objects {
		if err := o.Delete(ctx, client, namespace); err != nil {
			return fmt.Errorf("could not delete chart object: %w", err)
		}
	}

	// Delete subchart objects
	for _, sc := range c.SubCharts {
		if err := sc.Delete(ctx, client, namespace); err != nil {
			return err
		}
	}

	return nil
}

// Delete deletes this object from the given namespace using the given client.
func (o *Object) Delete(ctx context.Context, c client.Client, namespace string) error {
	obj := o.Type.DeepCopyObject().(client.Object)
	key := objectKey(namespace, o.Name)
	if err := c.Get(ctx, key, obj); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("could not get %T '%s': %w", obj, key.String(), err)
	}
	if err := c.Delete(ctx, obj); err != nil {
		return fmt.Errorf("could not delete %T '%s': %w", obj, key.String(), err)
	}
	return nil
}

func objectKey(namespace, name string) client.ObjectKey {
	return client.ObjectKey{Namespace: namespace, Name: name}
}

// InjectImages finds the images with the given names and opts, makes a shallow copy of the given
// Values and injects a name to image string mapping at the `images` key of that map and returns it.
func InjectImages(values map[string]any, v imagevector.ImageVector, names []string, opts ...imagevector.FindOptionFunc) (map[string]any, error) {
	images, err := imagevector.FindImages(v, names, opts...)
	if err != nil {
		return nil, err
	}

	values = utils.ShallowCopyMapStringInterface(values)
	values["images"] = imagevector.ImageMapToValues(images)
	return values, nil
}
