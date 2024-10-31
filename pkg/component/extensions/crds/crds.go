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
	crddeployer "github.com/gardener/gardener/pkg/component/crddeployer"
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
	generalCRDNamesToManifest, err = crddeployer.MakeCRDNameMap(generalCRDs)
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
	shootCRDNamesToManifest, err = crddeployer.MakeCRDNameMap(shootCRDs)
	utilruntime.Must(err)
}

type crd struct {
	generalCRDDeployer component.DeployWaiter
	shootCRDDeployer   component.DeployWaiter
	client             client.Client
	applier            kubernetes.Applier
	includeGeneralCRDs bool
	includeShootCRDs   bool
}

// NewCRD can be used to deploy extensions CRDs.
func NewCRD(client client.Client, applier kubernetes.Applier, includeGeneralCRDs, includeShootCRDs bool) (component.DeployWaiter, error) {
	generalCRDDeployer, err := crddeployer.NewCRDDeployer(client, applier, maps.Values(generalCRDNamesToManifest))
	if err != nil {
		return nil, err
	}

	shootCRDDeployer, err := crddeployer.NewCRDDeployer(client, applier, maps.Values(shootCRDNamesToManifest))
	if err != nil {
		return nil, err
	}

	return &crd{
		generalCRDDeployer: generalCRDDeployer,
		shootCRDDeployer:   shootCRDDeployer,
		client:             client,
		applier:            applier,
		includeGeneralCRDs: includeGeneralCRDs,
		includeShootCRDs:   includeShootCRDs,
	}, nil
}

// Deploy creates and updates the CRD definitions for the gardener extensions.
func (c *crd) Deploy(ctx context.Context) error {
	var fns []flow.TaskFn

	if c.includeGeneralCRDs {
		fns = append(fns, c.generalCRDDeployer.Deploy)
	}
	if c.includeShootCRDs {
		fns = append(fns, c.shootCRDDeployer.Deploy)
	}

	return flow.Parallel(fns...)(ctx)
}

// Destroy deletes the CRD manifests.
func (c *crd) Destroy(ctx context.Context) error {
	var fns []flow.TaskFn

	if c.includeGeneralCRDs {
		fns = append(fns, c.generalCRDDeployer.Destroy)
	}
	if c.includeShootCRDs {
		fns = append(fns, c.shootCRDDeployer.Destroy)
	}

	return flow.Parallel(fns...)(ctx)
}

// Wait waits until the CRDs are ready or times out.
func (c *crd) Wait(ctx context.Context) error {
	var fns []flow.TaskFn

	if c.includeGeneralCRDs {
		fns = append(fns, c.generalCRDDeployer.Wait)
	}
	if c.includeShootCRDs {
		fns = append(fns, c.shootCRDDeployer.Wait)
	}

	return flow.Parallel(fns...)(ctx)
}

// WaitCleanup waits until the CRDs are gone or times out.
func (c *crd) WaitCleanup(ctx context.Context) error {
	var fns []flow.TaskFn

	if c.includeGeneralCRDs {
		fns = append(fns, c.generalCRDDeployer.Wait)
	}
	if c.includeShootCRDs {
		fns = append(fns, c.shootCRDDeployer.Wait)
	}

	return flow.Parallel(fns...)(ctx)
}
