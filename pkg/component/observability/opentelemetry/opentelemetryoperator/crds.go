// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package opentelemetryoperator

import (
	"context"
	_ "embed"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/utils/flow"
)

var (
	//go:embed assets/crd-opentelemetry.io_opentelemetrycollectors.yaml
	openTelemetryOpenTelemetryCollectorCRD string
	//go:embed assets/crd-opentelemetry.io_instrumentations.yaml
	openTelemetryInstrumentationCRD string
	//go:embed assets/crd-opentelemetry.io_opampbridges.yaml
	openTelemetryOpenTelemetryCollectorBridgeCRD string
	//go:embed assets/crd-opentelemetry.io_targetallocators.yaml
	openTelemetryOpenTelemetryTargetAllocatorCRD string

	resources []string
)

func init() {
	resources = append(resources,
		openTelemetryOpenTelemetryCollectorCRD,
		openTelemetryInstrumentationCRD,
		openTelemetryOpenTelemetryCollectorBridgeCRD,
		openTelemetryOpenTelemetryTargetAllocatorCRD,
	)
}

type crds struct {
	applier kubernetes.Applier
}

// NewCRDs can be used to deploy OpenTelemetry Operator CRDS
func NewCRDs(a kubernetes.Applier) component.DeployWaiter {
	return &crds{
		applier: a,
	}
}

// Deploy creates and updates the CRD definitions for the OpenTelemetry Operator.
func (c *crds) Deploy(ctx context.Context) error {
	var fns []flow.TaskFn

	for _, resource := range resources {
		r := resource
		fns = append(fns, func(ctx context.Context) error {
			return c.applier.ApplyManifest(ctx, kubernetes.NewManifestReader([]byte(r)), kubernetes.DefaultMergeFuncs)
		})
	}

	return flow.Parallel(fns...)(ctx)
}

// Destroy deletes the CRDs for the OpenTelemetry Operator.
func (c *crds) Destroy(ctx context.Context) error {
	var fns []flow.TaskFn

	for _, resource := range resources {
		r := resource
		fns = append(fns, func(ctx context.Context) error {
			return client.IgnoreNotFound(c.applier.DeleteManifest(ctx, kubernetes.NewManifestReader([]byte(r))))
		})
	}

	return flow.Parallel(fns...)(ctx)
}

// Wait does nothing
func (c *crds) Wait(_ context.Context) error {
	return nil
}

// WaitCleanup does nothing
func (c *crds) WaitCleanup(_ context.Context) error {
	return nil
}
