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

package networkpolicies

import (
	"context"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/utils/managedresources"
)

// ManagedResourceControlName is the name of the of the global-network-policies managed resource.
const ManagedResourceControlName = "global-network-policies"

// NewBootstrapper creates a new instance of DeployWaiter for the network policies.
// Deprecated.
// TODO(rfrankze): Remove this component in a future release.
func NewBootstrapper(client client.Client, namespace string) component.DeployWaiter {
	return &bootstrapper{
		client:    client,
		namespace: namespace,
	}
}

type bootstrapper struct {
	client    client.Client
	namespace string
}

func (b *bootstrapper) Deploy(ctx context.Context) error {
	return b.Destroy(ctx)
}

func (b *bootstrapper) Destroy(ctx context.Context) error {
	return managedresources.DeleteForSeed(ctx, b.client, b.namespace, ManagedResourceControlName)
}

// TimeoutWaitForManagedResource is the timeout used while waiting for the ManagedResources to become healthy
// or deleted.
var TimeoutWaitForManagedResource = 2 * time.Minute

func (b *bootstrapper) Wait(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilHealthy(timeoutCtx, b.client, b.namespace, ManagedResourceControlName)
}

func (b *bootstrapper) WaitCleanup(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilDeleted(timeoutCtx, b.client, b.namespace, ManagedResourceControlName)
}
