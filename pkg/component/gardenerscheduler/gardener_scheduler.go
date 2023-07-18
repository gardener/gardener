// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package gardenerscheduler

import (
	"context"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/component"
	operatorclient "github.com/gardener/gardener/pkg/operator/client"
	"github.com/gardener/gardener/pkg/utils/flow"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

const (
	managedResourceNameRuntime = "gardener-scheduler-runtime"
	managedResourceNameVirtual = "gardener-scheduler-virtual"
)

// TimeoutWaitForManagedResource is the timeout used while waiting for the ManagedResources to become healthy or
// deleted.
var TimeoutWaitForManagedResource = 5 * time.Minute

// Values contains configuration values for the gardener-scheduler resources.
type Values struct {
}

// New creates a new instance of DeployWaiter for the gardener-scheduler.
func New(client client.Client, namespace string, secretsManager secretsmanager.Interface, values Values) component.DeployWaiter {
	return &gardenerScheduler{
		client:         client,
		namespace:      namespace,
		secretsManager: secretsManager,
		values:         values,
	}
}

type gardenerScheduler struct {
	client         client.Client
	namespace      string
	secretsManager secretsmanager.Interface
	values         Values
}

func (g *gardenerScheduler) Deploy(ctx context.Context) error {
	var (
		runtimeRegistry = managedresources.NewRegistry(operatorclient.RuntimeScheme, operatorclient.RuntimeCodec, operatorclient.RuntimeSerializer)
	)

	runtimeResources, err := runtimeRegistry.AddAllAndSerialize()
	if err != nil {
		return err
	}

	if err := managedresources.CreateForSeed(ctx, g.client, g.namespace, managedResourceNameRuntime, false, runtimeResources); err != nil {
		return err
	}

	var (
		virtualRegistry = managedresources.NewRegistry(operatorclient.VirtualScheme, operatorclient.VirtualCodec, operatorclient.VirtualSerializer)
	)

	virtualResources, err := virtualRegistry.AddAllAndSerialize()
	if err != nil {
		return err
	}

	return managedresources.CreateForShoot(ctx, g.client, g.namespace, managedResourceNameVirtual, managedresources.LabelValueGardener, false, virtualResources)
}

func (g *gardenerScheduler) Wait(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return flow.Parallel(
		func(ctx context.Context) error {
			return managedresources.WaitUntilHealthy(ctx, g.client, g.namespace, managedResourceNameRuntime)
		},
		func(ctx context.Context) error {
			return managedresources.WaitUntilHealthy(ctx, g.client, g.namespace, managedResourceNameVirtual)
		},
	)(timeoutCtx)
}

func (g *gardenerScheduler) Destroy(ctx context.Context) error {
	if err := managedresources.DeleteForShoot(ctx, g.client, g.namespace, managedResourceNameVirtual); err != nil {
		return err
	}

	return managedresources.DeleteForSeed(ctx, g.client, g.namespace, managedResourceNameRuntime)
}

func (g *gardenerScheduler) WaitCleanup(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return flow.Parallel(
		func(ctx context.Context) error {
			return managedresources.WaitUntilDeleted(ctx, g.client, g.namespace, managedResourceNameRuntime)
		},
		func(ctx context.Context) error {
			return managedresources.WaitUntilDeleted(ctx, g.client, g.namespace, managedResourceNameVirtual)
		},
	)(timeoutCtx)
}
