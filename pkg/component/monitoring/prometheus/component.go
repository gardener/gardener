// Copyright 2024 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package prometheus

import (
	"context"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/utils/managedresources"
)

// Values contains configuration values for the prometheus resources.
type Values struct {
	// Name is the name of the prometheus. It will be used for the resource names of Prometheus and ManagedResource.
	Name string
}

// New creates a new instance of DeployWaiter for the prometheus.
func New(client client.Client, namespace string, values Values) component.DeployWaiter {
	return &prometheus{
		client:    client,
		namespace: namespace,
		values:    values,
	}
}

type prometheus struct {
	client    client.Client
	namespace string
	values    Values
}

func (p *prometheus) Deploy(ctx context.Context) error {
	registry := managedresources.NewRegistry(kubernetes.SeedScheme, kubernetes.SeedCodec, kubernetes.SeedSerializer)

	resources, err := registry.AddAllAndSerialize()
	if err != nil {
		return err
	}

	return managedresources.CreateForSeed(ctx, p.client, p.namespace, p.name(), false, resources)
}

func (p *prometheus) Destroy(ctx context.Context) error {
	return managedresources.DeleteForSeed(ctx, p.client, p.namespace, p.name())
}

// TimeoutWaitForManagedResource is the timeout used while waiting for the ManagedResources to become healthy or
// deleted.
var TimeoutWaitForManagedResource = 5 * time.Minute

func (p *prometheus) Wait(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilHealthy(timeoutCtx, p.client, p.namespace, p.name())
}

func (p *prometheus) WaitCleanup(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilDeleted(timeoutCtx, p.client, p.namespace, p.name())
}

func (p *prometheus) name() string {
	return "prometheus-" + p.values.Name
}
