// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

	resources []string
)

func init() {
	resources = append(resources,
		backupBucketCRD,
		backupEntryCRD,
		bastionCRD,
		clusterCRD,
		containerRuntimeCRD,
		controlPlaneCRD,
		dnsRecordCRD,
		extensionCRD,
		infrastructureCRD,
		networkCRD,
		operatingSystemConfigCRD,
		workerCRD,
	)
}

type crd struct {
	applier kubernetes.Applier
}

// NewCRD can be used to deploy extensions CRDs.
func NewCRD(a kubernetes.Applier) component.DeployWaiter {
	return &crd{
		applier: a,
	}
}

// Deploy creates and updates the CRD definitions for the gardener extensions.
func (c *crd) Deploy(ctx context.Context) error {
	var fns []flow.TaskFn

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
