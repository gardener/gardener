// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/utils/flow"
)

var (
	//go:embed templates/crd-backupbucket.tpl.yaml
	backupBucketCRD string
	//go:embed templates/crd-backupentry.tpl.yaml
	backupEntryCRD string
	//go:embed templates/crd-bastion.tpl.yaml
	bastionCRD string
	//go:embed templates/crd-cluster.tpl.yaml
	clusterCRD string
	//go:embed templates/crd-containerruntime.tpl.yaml
	containerRuntimeCRD string
	//go:embed templates/crd-controlplane.tpl.yaml
	controlPlaneCRD string
	//go:embed templates/crd-dnsentry.tpl.yaml
	dnsEntryCRD string
	//go:embed templates/crd-dnsowner.tpl.yaml
	dnsOwnerCRD string
	//go:embed templates/crd-dnsprovider.tpl.yaml
	dnsProviderCRD string
	//go:embed templates/crd-dnsrecord.tpl.yaml
	dnsRecordCRD string
	//go:embed templates/crd-extension.tpl.yaml
	extensionCRD string
	//go:embed templates/crd-infrastructure.tpl.yaml
	infrastructureCRD string
	//go:embed templates/crd-managedresources.tpl.yaml
	managedResourcesCRD string
	//go:embed templates/crd-network.tpl.yaml
	networkCRD string
	//go:embed templates/crd-operatingsystemconfig.tpl.yaml
	operatingSystemConfigCRD string
	//go:embed templates/crd-worker.tpl.yaml
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
		dnsEntryCRD,
		dnsOwnerCRD,
		dnsProviderCRD,
		dnsRecordCRD,
		extensionCRD,
		infrastructureCRD,
		managedResourcesCRD,
		networkCRD,
		operatingSystemConfigCRD,
		workerCRD,
	)
}

type extensionCRDs struct {
	applier kubernetes.Applier
}

// NewExtensionsCRD can be used to deploy extensions CRDs.
func NewExtensionsCRD(a kubernetes.Applier) component.DeployWaiter {
	return &extensionCRDs{
		applier: a,
	}
}

// Deploy creates and updates the CRD definitions for the gardener extensions.
func (c *extensionCRDs) Deploy(ctx context.Context) error {
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
func (c *extensionCRDs) Destroy(ctx context.Context) error {
	return nil
}

// Wait does nothing
func (c *extensionCRDs) Wait(ctx context.Context) error {
	return nil
}

// WaitCleanup does nothing
func (c *extensionCRDs) WaitCleanup(ctx context.Context) error {
	return nil
}
