// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package fluentoperator

import (
	"context"
	_ "embed"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/utils/flow"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

var (
	//go:embed assets/crd-fluentbit.fluent.io_clusterfilters.yaml
	fluentBitClusterFilterCRD string
	//go:embed assets/crd-fluentbit.fluent.io_clusterfluentbitconfigs.yaml
	fluentBitClusterFBConfigCRD string
	//go:embed assets/crd-fluentbit.fluent.io_clusterinputs.yaml
	fluentBitClusterInputCRD string
	//go:embed assets/crd-fluentbit.fluent.io_clusteroutputs.yaml
	fluentBitClusterOutputCRD string
	//go:embed assets/crd-fluentbit.fluent.io_clusterparsers.yaml
	fluentBitClusterParserCRD string
	//go:embed assets/crd-fluentbit.fluent.io_fluentbits.yaml
	fluentBitCRD string
	//go:embed assets/crd-fluentbit.fluent.io_collectors.yaml
	fluentBitCollectorCRD string
	//go:embed assets/crd-fluentbit.fluent.io_fluentbitconfigs.yaml
	fluentBitConfigCRD string
	//go:embed assets/crd-fluentbit.fluent.io_filters.yaml
	fluentBitFilterCRD string
	//go:embed assets/crd-fluentbit.fluent.io_parsers.yaml
	fluentBitParserCRD string
	//go:embed assets/crd-fluentbit.fluent.io_outputs.yaml
	fluentBitOutputCRD string
	//go:embed assets/crd-fluentbit.fluent.io_clustermultilineparsers.yaml
	fluentBitClusterMultilineParserCRD string
	//go:embed assets/crd-fluentbit.fluent.io_multilineparsers.yaml
	fluentBitMultilineParserCRD string

	resources []string
)

func init() {
	resources = append(resources,
		fluentBitClusterFilterCRD,
		fluentBitClusterFBConfigCRD,
		fluentBitClusterInputCRD,
		fluentBitClusterOutputCRD,
		fluentBitClusterParserCRD,
		fluentBitCRD,
		fluentBitCollectorCRD,
		fluentBitConfigCRD,
		fluentBitFilterCRD,
		fluentBitParserCRD,
		fluentBitOutputCRD,
		fluentBitClusterMultilineParserCRD,
		fluentBitMultilineParserCRD,
	)
}

type crds struct {
	client  client.Client
	applier kubernetes.Applier
}

// NewCRDs can be used to deploy Fluent Operator CRDs.
func NewCRDs(client client.Client, a kubernetes.Applier) component.DeployWaiter {
	return &crds{
		client:  client,
		applier: a,
	}
}

// Deploy creates and updates the CRD definitions for the Fluent Operator.
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

// Destroy deletes the CRDs for the Fluent Operator.
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

// Wait signals whether a CRD is ready or needs more time to be deployed.
func (c *crds) Wait(ctx context.Context) error {
	return kubernetesutils.WaitUntilCRDManifestsReady(ctx, c.client, resources)
}

// WaitCleanup for destruction to finish and component to be fully removed. crdDeployer does not need to wait for cleanup.
func (c *crds) WaitCleanup(_ context.Context) error {
	return nil
}
