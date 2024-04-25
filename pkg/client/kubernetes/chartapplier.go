// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package kubernetes

import (
	"context"
	"embed"

	"k8s.io/client-go/rest"

	"github.com/gardener/gardener/pkg/chartrenderer"
)

// ChartApplier is an interface that describes needed methods that render and apply
// Helm charts in Kubernetes clusters.
type ChartApplier interface {
	chartrenderer.Interface
	ApplyFromEmbeddedFS(ctx context.Context, embeddedFS embed.FS, chartPath, namespace, name string, opts ...ApplyOption) error
	DeleteFromEmbeddedFS(ctx context.Context, embeddedFS embed.FS, chartPath, namespace, name string, opts ...DeleteOption) error
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

func (c *chartApplier) ApplyFromEmbeddedFS(ctx context.Context, embeddedFS embed.FS, chartPath, namespace, name string, opts ...ApplyOption) error {
	applyOpts := &ApplyOptions{}

	for _, o := range opts {
		if o != nil {
			o.MutateApplyOptions(applyOpts)
		}
	}

	if len(applyOpts.MergeFuncs) == 0 {
		applyOpts.MergeFuncs = DefaultMergeFuncs
	}

	manifestReader, err := c.newManifestReader(embeddedFS, chartPath, namespace, name, applyOpts.Values)
	if err != nil {
		return err
	}

	if applyOpts.ForceNamespace {
		manifestReader = NewNamespaceSettingReader(manifestReader, namespace)
	}

	return c.ApplyManifest(ctx, manifestReader, applyOpts.MergeFuncs)
}

func (c *chartApplier) DeleteFromEmbeddedFS(ctx context.Context, embeddedFS embed.FS, chartPath, namespace, name string, opts ...DeleteOption) error {
	deleteOpts := &DeleteOptions{}

	for _, o := range opts {
		if o != nil {
			o.MutateDeleteOptions(deleteOpts)
		}
	}

	manifestReader, err := c.newManifestReader(embeddedFS, chartPath, namespace, name, deleteOpts.Values)
	if err != nil {
		return err
	}

	if deleteOpts.ForceNamespace {
		manifestReader = NewNamespaceSettingReader(manifestReader, namespace)
	}

	deleteManifestOpts := []DeleteManifestOption{}

	for _, tf := range deleteOpts.TolerateErrorFuncs {
		if tf != nil {
			deleteManifestOpts = append(deleteManifestOpts, tf)
		}
	}

	return c.DeleteManifest(ctx, manifestReader, deleteManifestOpts...)
}

func (c *chartApplier) newManifestReader(embeddedFS embed.FS, chartPath, namespace, name string, values interface{}) (UnstructuredReader, error) {
	var (
		release *chartrenderer.RenderedChart
		err     error
	)

	release, err = c.RenderEmbeddedFS(embeddedFS, chartPath, name, namespace, values)
	if err != nil {
		return nil, err
	}

	return NewManifestReader(release.Manifest()), nil
}
