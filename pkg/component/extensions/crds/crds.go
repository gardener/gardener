// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package crds

import (
	"context"
	_ "embed"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/utils/flow"
)

var (
	//go:embed assets/crd-extensions.gardener.cloud_backupbuckets.yaml
	backupBucketCRD string
	//go:embed assets/crd-extensions.gardener.cloud_backupentries.yaml
	backupEntryCRD string
	//go:embed assets/crd-extensions.gardener.cloud_bastions.yaml
	bastionCRD string
	//go:embed assets/crd-extensions.gardener.cloud_clusters.yaml
	clusterCRD string
	//go:embed assets/crd-extensions.gardener.cloud_containerruntimes.yaml
	containerRuntimeCRD string
	//go:embed assets/crd-extensions.gardener.cloud_controlplanes.yaml
	controlPlaneCRD string
	//go:embed assets/crd-extensions.gardener.cloud_dnsrecords.yaml
	dnsRecordCRD string
	//go:embed assets/crd-extensions.gardener.cloud_extensions.yaml
	extensionCRD string
	//go:embed assets/crd-extensions.gardener.cloud_infrastructures.yaml
	infrastructureCRD string
	//go:embed assets/crd-extensions.gardener.cloud_networks.yaml
	networkCRD string
	//go:embed assets/crd-extensions.gardener.cloud_operatingsystemconfigs.yaml
	operatingSystemConfigCRD string
	//go:embed assets/crd-extensions.gardener.cloud_workers.yaml
	workerCRD string

	generalCRDs []string
	shootCRDs   []string
)

func init() {
	generalCRDs = []string{
		backupBucketCRD,
		dnsRecordCRD,
		extensionCRD,
	}

	shootCRDs = []string{
		backupEntryCRD,
		bastionCRD,
		clusterCRD,
		containerRuntimeCRD,
		controlPlaneCRD,
		infrastructureCRD,
		networkCRD,
		operatingSystemConfigCRD,
		workerCRD,
	}
}

type crd struct {
	applier            kubernetes.Applier
	includeGeneralCRDs bool
	includeShootCRDs   bool
}

// NewCRD can be used to deploy extensions CRDs.
func NewCRD(a kubernetes.Applier, includeGeneralCRDs, includeShootCRDs bool) component.DeployWaiter {
	return &crd{
		applier:            a,
		includeGeneralCRDs: includeGeneralCRDs,
		includeShootCRDs:   includeShootCRDs,
	}
}

// Deploy creates and updates the CRD definitions for the gardener extensions.
func (c *crd) Deploy(ctx context.Context) error {
	var fns []flow.TaskFn

	var resources []string
	if c.includeGeneralCRDs {
		resources = append(resources, generalCRDs...)
	}
	if c.includeShootCRDs {
		resources = append(resources, shootCRDs...)
	}

	for _, resource := range resources {
		r := resource
		fns = append(fns, func(ctx context.Context) error {
			return c.applier.ApplyManifest(ctx, kubernetes.NewManifestReader([]byte(r)), kubernetes.DefaultMergeFuncs)
		})
	}

	return flow.Parallel(fns...)(ctx)
}

// Destroy does nothing
func (c *crd) Destroy(ctx context.Context) error {
	var fns []flow.TaskFn

	var resources []string
	if c.includeGeneralCRDs {
		resources = append(resources, generalCRDs...)
	}
	if c.includeShootCRDs {
		resources = append(resources, shootCRDs...)
	}

	for _, resource := range resources {
		r := resource
		fns = append(fns, func(ctx context.Context) error {
			return client.IgnoreNotFound(c.applier.DeleteManifest(ctx, kubernetes.NewManifestReader([]byte(r))))
		})
	}

	return flow.Parallel(fns...)(ctx)
}

// Wait does nothing
func (c *crd) Wait(_ context.Context) error {
	return nil
}

// WaitCleanup does nothing
func (c *crd) WaitCleanup(_ context.Context) error {
	return nil
}
