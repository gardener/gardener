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

package plutono

import (
	"context"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/utils/managedresources"
)

const (
	// ManagedResourceName is the name of the ManagedResource containing the resource specifications.
	ManagedResourceName = "plutono"
)

// Values is a set of configuration values for the plutono component.
type Values struct {
	// Image is the container image used for plutono.
	Image string
}

// New creates a new instance of DeployWaiter for plutono.
func New(
	client client.Client,
	namespace string,
	values Values,
) component.DeployWaiter {
	return &plutono{
		client:    client,
		namespace: namespace,
		values:    values,
	}
}

type plutono struct {
	client    client.Client
	namespace string
	values    Values
}

func (p *plutono) Deploy(ctx context.Context) error {
	data, err := p.computeResourcesData()
	if err != nil {
		return err
	}
	return managedresources.CreateForSeed(ctx, p.client, p.namespace, ManagedResourceName, false, data)
}

func (p *plutono) Destroy(ctx context.Context) error {
	return managedresources.DeleteForSeed(ctx, p.client, p.namespace, ManagedResourceName)
}

// TimeoutWaitForManagedResource is the timeout used while waiting for the ManagedResources to become healthy
// or deleted.
var TimeoutWaitForManagedResource = 2 * time.Minute

func (p *plutono) Wait(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilHealthy(timeoutCtx, p.client, p.namespace, ManagedResourceName)
}

func (p *plutono) WaitCleanup(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilDeleted(timeoutCtx, p.client, p.namespace, ManagedResourceName)
}

func (p *plutono) computeResourcesData() (map[string][]byte, error) {
	return nil, nil
}
