// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package crds

import (
	"context"
	_ "embed"

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
	generalCRDs = append(generalCRDs,
		backupBucketCRD,
		dnsRecordCRD,
		extensionCRD,
	)
	shootCRDs = append(shootCRDs,
		backupEntryCRD,
		bastionCRD,
		clusterCRD,
		containerRuntimeCRD,
		controlPlaneCRD,
		infrastructureCRD,
		networkCRD,
		operatingSystemConfigCRD,
		workerCRD,
	)
}

type crd struct {
	applier            kubernetes.Applier
	excludeGeneralCRDs bool
	excludeShootCRDs   bool
}

// NewCRD can be used to deploy extensions CRDs.
func NewCRD(a kubernetes.Applier, excludeGeneralCRDs, excludeShootCRDs bool) component.DeployWaiter {
	return &crd{
		applier:            a,
		excludeGeneralCRDs: excludeGeneralCRDs,
		excludeShootCRDs:   excludeShootCRDs,
	}
}

// Deploy creates and updates the CRD definitions for the gardener extensions.
func (c *crd) Deploy(ctx context.Context) error {
	var fns []flow.TaskFn

	var resources []string
	if !c.excludeGeneralCRDs {
		resources = append(resources, generalCRDs...)
	}
	if !c.excludeShootCRDs {
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
func (c *crd) Destroy(_ context.Context) error {
	return nil
}

// Wait does nothing
func (c *crd) Wait(_ context.Context) error {
	return nil
}

// WaitCleanup does nothing
func (c *crd) WaitCleanup(_ context.Context) error {
	return nil
}
