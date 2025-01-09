// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package crds

import (
	_ "embed"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/crddeployer"
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
)

// NewCRD can be used to deploy extensions CRDs.
func NewCRD(client client.Client, applier kubernetes.Applier, includeGeneralCRDs, includeShootCRDs bool) (component.DeployWaiter, error) {
	var (
		crds        []string
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
	)

	if includeGeneralCRDs {
		crds = append(crds, generalCRDs...)
	}
	if includeShootCRDs {
		crds = append(crds, shootCRDs...)
	}

	return crddeployer.New(client, applier, crds, false)
}
