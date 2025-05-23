// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
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
	ApplyFromArchive(ctx context.Context, archive []byte, namespace, name string, opts ...ApplyOption) error
	DeleteFromArchive(ctx context.Context, archive []byte, namespace, name string, opts ...DeleteOption) error
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
	applyOpts := getApplyOptions(opts...)

	reader, err := c.newManifestReaderFromEmbeddedFS(embeddedFS, chartPath, namespace, name, applyOpts.Values)
	if err != nil {
		return err
	}

	return c.apply(ctx, reader, namespace, applyOpts)
}

func (c *chartApplier) DeleteFromEmbeddedFS(ctx context.Context, embeddedFS embed.FS, chartPath, namespace, name string, opts ...DeleteOption) error {
	deleteOpts := getDeleteOptions(opts...)

	reader, err := c.newManifestReaderFromEmbeddedFS(embeddedFS, chartPath, namespace, name, deleteOpts.Values)
	if err != nil {
		return err
	}

	return c.delete(ctx, reader, namespace, deleteOpts)
}

func (c *chartApplier) ApplyFromArchive(ctx context.Context, archive []byte, namespace, name string, opts ...ApplyOption) error {
	applyOpts := getApplyOptions(opts...)

	release, err := c.RenderArchive(archive, name, namespace, applyOpts.Values)
	if err != nil {
		return err
	}

	return c.apply(ctx, NewManifestReader(release.Manifest()), namespace, applyOpts)
}

func (c *chartApplier) DeleteFromArchive(ctx context.Context, archive []byte, namespace, name string, opts ...DeleteOption) error {
	deleteOpts := getDeleteOptions(opts...)

	release, err := c.RenderArchive(archive, name, namespace, deleteOpts.Values)
	if err != nil {
		return err
	}

	return c.delete(ctx, NewManifestReader(release.Manifest()), namespace, deleteOpts)
}

func (c *chartApplier) apply(ctx context.Context, reader UnstructuredReader, namespace string, applyOpts *ApplyOptions) error {
	if applyOpts.ForceNamespace {
		reader = NewNamespaceSettingReader(reader, namespace)
	}

	return c.ApplyManifest(ctx, reader, applyOpts.MergeFuncs)
}

func (c *chartApplier) delete(ctx context.Context, reader UnstructuredReader, namespace string, deleteOpts *DeleteOptions) error {
	if deleteOpts.ForceNamespace {
		reader = NewNamespaceSettingReader(reader, namespace)
	}

	deleteManifestOpts := []DeleteManifestOption{}

	for _, tf := range deleteOpts.TolerateErrorFuncs {
		if tf != nil {
			deleteManifestOpts = append(deleteManifestOpts, tf)
		}
	}

	return c.DeleteManifest(ctx, reader, deleteManifestOpts...)
}

func (c *chartApplier) newManifestReaderFromEmbeddedFS(embeddedFS embed.FS, chartPath, namespace, name string, values any) (UnstructuredReader, error) {
	release, err := c.RenderEmbeddedFS(embeddedFS, chartPath, name, namespace, values)
	if err != nil {
		return nil, err
	}

	return NewManifestReader(release.Manifest()), nil
}

func getApplyOptions(opts ...ApplyOption) *ApplyOptions {
	applyOpts := &ApplyOptions{}

	for _, o := range opts {
		if o != nil {
			o.MutateApplyOptions(applyOpts)
		}
	}

	if len(applyOpts.MergeFuncs) == 0 {
		applyOpts.MergeFuncs = DefaultMergeFuncs
	}

	return applyOpts
}

func getDeleteOptions(opts ...DeleteOption) *DeleteOptions {
	deleteOpts := &DeleteOptions{}

	for _, o := range opts {
		if o != nil {
			o.MutateDeleteOptions(deleteOpts)
		}
	}

	return deleteOpts
}
