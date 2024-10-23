// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package crds

import (
	"context"
	_ "embed"

	"golang.org/x/exp/maps"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/utils/flow"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
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

	generalCRDNamesToManifest map[string]string
	shootCRDNamesToManifest   map[string]string
)

func init() {
	var err error
	generalCRDs := []string{
		backupBucketCRD,
		dnsRecordCRD,
		extensionCRD,
	}
	generalCRDNamesToManifest, err = kubernetesutils.MakeCRDNameMap(generalCRDs)
	utilruntime.Must(err)

	shootCRDs := []string{
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
	shootCRDNamesToManifest, err = kubernetesutils.MakeCRDNameMap(shootCRDs)
	utilruntime.Must(err)
}

type crd struct {
	client             client.Client
	applier            kubernetes.Applier
	includeGeneralCRDs bool
	includeShootCRDs   bool
}

// NewCRD can be used to deploy extensions CRDs.
func NewCRD(client client.Client, a kubernetes.Applier, includeGeneralCRDs, includeShootCRDs bool) component.DeployWaiter {
	return &crd{
		client:             client,
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
		resources = append(resources, maps.Values(generalCRDNamesToManifest)...)
	}
	if c.includeShootCRDs {
		resources = append(resources, maps.Values(shootCRDNamesToManifest)...)
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
		resources = append(resources, maps.Values(generalCRDNamesToManifest)...)
	}
	if c.includeShootCRDs {
		resources = append(resources, maps.Values(shootCRDNamesToManifest)...)
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
func (c *crd) Wait(ctx context.Context) error {
	var names []string
	if c.includeGeneralCRDs {
		names = append(names, maps.Keys(generalCRDNamesToManifest)...)
	}
	if c.includeShootCRDs {
		names = append(names, maps.Keys(shootCRDNamesToManifest)...)
	}
	return kubernetesutils.WaitUntilCRDManifestsReady(ctx, c.client, names)
}

// WaitCleanup does nothing
func (c *crd) WaitCleanup(_ context.Context) error {
	return nil
}
