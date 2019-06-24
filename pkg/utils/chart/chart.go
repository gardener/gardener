// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package chart

import (
	"context"

	"github.com/gardener/gardener/pkg/chartrenderer"
	gardenerkubernetes "github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/imagevector"

	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Chart represents a Helm chart (and its sub-charts) that can be applied and deleted.
type Chart struct {
	Name      string
	Path      string
	Images    []string
	Objects   []*Object
	SubCharts []*Chart
}

// Object represents an object deployed by a Chart.
type Object struct {
	Type runtime.Object
	Name string
}

// Apply applies this chart in the given namespace using the given ChartApplier. Before applying the chart,
// it collects its values, injecting images and merging the given values as needed.
func (c *Chart) Apply(
	ctx context.Context,
	chartApplier gardenerkubernetes.ChartApplier,
	namespace string,
	imageVector imagevector.ImageVector,
	runtimeVersion, targetVersion string,
	additionalValues map[string]interface{},
) error {

	// Get values with injected images
	values, err := c.injectImages(imageVector, runtimeVersion, targetVersion)
	if err != nil {
		return err
	}

	// Apply chart
	err = chartApplier.ApplyChart(ctx, c.Path, namespace, c.Name, values, additionalValues)
	if err != nil {
		return errors.Wrapf(err, "could not apply chart '%s' in namespace '%s'", c.Name, namespace)
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
	additionalValues map[string]interface{},
) (string, []byte, error) {

	// Get values with injected images
	values, err := c.injectImages(imageVector, runtimeVersion, targetVersion)
	if err != nil {
		return "", nil, err
	}

	// Apply chart
	rc, err := chartRenderer.Render(c.Path, c.Name, namespace, utils.MergeMaps(values, additionalValues))
	if err != nil {
		return "", nil, errors.Wrapf(err, "could not render chart '%s' in namespace '%s'", c.Name, namespace)
	}
	return rc.ChartName, rc.Manifest(), nil
}

// injectImages collects returns a values map with injected images, including sub-charts.
func (c *Chart) injectImages(
	imageVector imagevector.ImageVector,
	runtimeVersion, targetVersion string,
) (map[string]interface{}, error) {

	// Inject images
	values := make(map[string]interface{})
	var err error
	if len(c.Images) > 0 {
		values, err = InjectImages(values, imageVector, c.Images, imagevector.RuntimeVersion(runtimeVersion), imagevector.TargetVersion(targetVersion))
		if err != nil {
			return nil, errors.Wrapf(err, "could not inject chart '%s' images", c.Name)
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
			return errors.Wrap(err, "could not delete chart '%s' object")
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
func (o *Object) Delete(ctx context.Context, client client.Client, namespace string) error {
	obj := o.Type.DeepCopyObject()
	kind := obj.GetObjectKind().GroupVersionKind().Kind
	key := objectKey(namespace, o.Name)
	if err := client.Get(ctx, key, obj); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return errors.Wrapf(err, "could not get %s '%s'", kind, key.String())
	}
	if err := client.Delete(ctx, obj); err != nil {
		return errors.Wrapf(err, "could not delete %s '%s'", kind, key.String())
	}
	return nil
}

func objectKey(namespace, name string) client.ObjectKey {
	return client.ObjectKey{Namespace: namespace, Name: name}
}

// CopyValues creates a shallow copy of the given Values.
func CopyValues(values map[string]interface{}) map[string]interface{} {
	copiedValues := make(map[string]interface{}, len(values))
	for k, v := range values {
		copiedValues[k] = v
	}
	return copiedValues
}

// ImageMapToValues transforms the given image name to image mapping into chart Values.
func ImageMapToValues(m map[string]*imagevector.Image) map[string]interface{} {
	out := make(map[string]interface{}, len(m))
	for k, v := range m {
		out[k] = v.String()
	}
	return out
}

// InjectImages finds the images with the given names and opts, makes a shallow copy of the given
// Values and injects a name to image string mapping at the `images` key of that map and returns it.
func InjectImages(values map[string]interface{}, v imagevector.ImageVector, names []string, opts ...imagevector.FindOptionFunc) (map[string]interface{}, error) {
	images, err := imagevector.FindImages(v, names, opts...)
	if err != nil {
		return nil, err
	}

	values = CopyValues(values)
	values["images"] = ImageMapToValues(images)
	return values, nil
}
