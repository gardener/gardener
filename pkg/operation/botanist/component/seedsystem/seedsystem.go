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

	schedulingv1 "k8s.io/api/scheduling/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ManagedResourceName is the name of the ManagedResource containing the resource specifications.
const ManagedResourceName = "system"

// Values is a set of configuration values for the system resources.
type Values struct {
	// ReserveExcessCapacity contains configuration for the deployment of the excess capacity reservation resources.
	ReserveExcessCapacity ReserveExcessCapacityValues
}

// ReserveExcessCapacityValues contains configuration for the deployment of the excess capacity reservation resources.
type ReserveExcessCapacityValues struct {
	// Enabled specifies whether excess capacity reservation should be enabled.
	Enabled bool
	// Image is the container image.
	Image string
}

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

	if s.values.ReserveExcessCapacity.Enabled {
		if err := s.addReserveExcessCapacityResources(registry); err != nil {
			return nil, err
		}
	}

	return registry.SerializedObjects(), nil
}

func (s *seedSystem) addReserveExcessCapacityResources(registry *managedresources.Registry) error {
	var (
		priorityClass = &schedulingv1.PriorityClass{
			ObjectMeta: metav1.ObjectMeta{
				Name: "gardener-reserve-excess-capacity",
			},
			Value:         -5,
			GlobalDefault: false,
			Description:   "This class is used to reserve excess resource capacity on a cluster",
		}
	)

	return registry.Add(
		priorityClass,
	)
}
