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

package kubernetes

import (
	"context"

	"github.com/gardener/gardener/pkg/chartrenderer"

	"k8s.io/client-go/rest"
)

// ChartApplier is an interface that describes needed methods that render and apply
// Helm charts in Kubernetes clusters.
type ChartApplier interface {
	chartrenderer.Interface
	Apply(ctx context.Context, chartPath, namespace, name string, opts ...ApplyOption) error
	Delete(ctx context.Context, chartPath, namespace, name string, opts ...DeleteOption) error
}

// chartApplier is a structure that contains a chart renderer and a manifest applier.
type chartApplier struct {
	chartrenderer.Interface
	Applier
}

// NewChartApplier returns a new chart applier.
func NewChartApplier(renderer chartrenderer.Interface, applier Applier) ChartApplier {
	return &chartApplier{renderer, applier}
}

// NewChartApplierForConfig returns a new chart applier based on the given REST config.
func NewChartApplierForConfig(config *rest.Config) (ChartApplier, error) {
	renderer, err := chartrenderer.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	applier, err := NewApplierForConfig(config)
	if err != nil {
		return nil, err
	}
	return NewChartApplier(renderer, applier), nil
}

// Apply takes a path to a chart <chartPath>, name of the release <name>,
// release's namespace <namespace> and renders the template based value.
// The resulting manifest will be applied to the cluster the Kubernetes client has been created for.
// <options> can be used to enchance the existing functionality.
func (c *chartApplier) Apply(ctx context.Context, chartPath, namespace, name string, opts ...ApplyOption) error {
	applyOpts := &ApplyOptions{}

	for _, o := range opts {
		if o != nil {
			o.MutateApplyOptions(applyOpts)
		}
	}

	if len(applyOpts.MergeFuncs) == 0 {
		applyOpts.MergeFuncs = DefaultMergeFuncs
	}

	manifestReader, err := c.manifestReader(chartPath, namespace, name, applyOpts.Values)
	if err != nil {
		return err
	}

	if applyOpts.ForceNamespace {
		manifestReader = NewNamespaceSettingReader(manifestReader, namespace)
	}

	return c.ApplyManifest(ctx, manifestReader, applyOpts.MergeFuncs)
}

// Delete takes a path to a chart <chartPath>, name of the release <name>,
// release's namespace <namespace> and renders the template.
// The resulting manifest will be deleted from the cluster the Kubernetes client has been created for.
func (c *chartApplier) Delete(ctx context.Context, chartPath, namespace, name string, opts ...DeleteOption) error {
	deleteOpts := &DeleteOptions{}

	for _, o := range opts {
		if o != nil {
			o.MutateDeleteOptions(deleteOpts)
		}
	}

	manifestReader, err := c.manifestReader(chartPath, namespace, name, deleteOpts.Values)
	if err != nil {
		return err
	}

	if deleteOpts.ForceNamespace {
		manifestReader = NewNamespaceSettingReader(manifestReader, namespace)
	}

	return c.DeleteManifest(ctx, manifestReader)
}

func (c *chartApplier) manifestReader(chartPath, namespace, name string, values interface{}) (UnstructuredReader, error) {
	release, err := c.Render(chartPath, name, namespace, values)
	if err != nil {
		return nil, err
	}
	return NewManifestReader(release.Manifest()), nil
}
