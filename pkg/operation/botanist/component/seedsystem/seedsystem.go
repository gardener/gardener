// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package seedsystem

import (
	"context"
	"time"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/utils/managedresources"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ManagedResourceName is the name of the ManagedResource containing the resource specifications.
const ManagedResourceName = "system"

// Values is a set of configuration values for the system resources.
type Values struct{}

// New creates a new instance of DeployWaiter for seed system resources.
func New(
	client client.Client,
	namespace string,
	values Values,
) component.DeployWaiter {
	return &seedSystem{
		client:    client,
		namespace: namespace,
		values:    values,
	}
}

type seedSystem struct {
	client    client.Client
	namespace string
	values    Values
}

func (s *seedSystem) Deploy(ctx context.Context) error {
	data, err := s.computeResourcesData()
	if err != nil {
		return err
	}

	return managedresources.CreateForSeed(ctx, s.client, s.namespace, ManagedResourceName, false, data)
}

func (s *seedSystem) Destroy(ctx context.Context) error {
	return managedresources.DeleteForSeed(ctx, s.client, s.namespace, ManagedResourceName)
}

// TimeoutWaitForManagedResource is the timeout used while waiting for the ManagedResources to become healthy
// or deleted.
var TimeoutWaitForManagedResource = 2 * time.Minute

func (s *seedSystem) Wait(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilHealthy(timeoutCtx, s.client, s.namespace, ManagedResourceName)
}

func (s *seedSystem) WaitCleanup(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilDeleted(timeoutCtx, s.client, s.namespace, ManagedResourceName)
}

func (s *seedSystem) computeResourcesData() (map[string][]byte, error) {
	var (
		registry = managedresources.NewRegistry(kubernetes.ShootScheme, kubernetes.ShootCodec, kubernetes.ShootSerializer)
	)

	return registry.AddAllAndSerialize()
}
