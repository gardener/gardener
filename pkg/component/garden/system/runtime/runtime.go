// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package runtime

import (
	"context"
	"time"

	schedulingv1 "k8s.io/api/scheduling/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/utils/managedresources"
)

// ManagedResourceName is the name of the ManagedResource containing the resource specifications.
const ManagedResourceName = "garden-system"

// New creates a new instance of DeployWaiter for runtime garden system resources.
func New(client client.Client, namespace string) component.DeployWaiter {
	return &gardenSystem{
		client:    client,
		namespace: namespace,
	}
}

type gardenSystem struct {
	client    client.Client
	namespace string
}

func (g *gardenSystem) Deploy(ctx context.Context) error {
	data, err := g.computeResourcesData()
	if err != nil {
		return err
	}

	return managedresources.CreateForSeed(ctx, g.client, g.namespace, ManagedResourceName, false, data)
}

func (g *gardenSystem) Destroy(ctx context.Context) error {
	return managedresources.DeleteForSeed(ctx, g.client, g.namespace, ManagedResourceName)
}

// TimeoutWaitForManagedResource is the timeout used while waiting for the ManagedResources to become healthy
// or deleted.
var TimeoutWaitForManagedResource = 2 * time.Minute

func (g *gardenSystem) Wait(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilHealthy(timeoutCtx, g.client, g.namespace, ManagedResourceName)
}

func (g *gardenSystem) WaitCleanup(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilDeleted(timeoutCtx, g.client, g.namespace, ManagedResourceName)
}

func (g *gardenSystem) computeResourcesData() (map[string][]byte, error) {
	var (
		registry = managedresources.NewRegistry(kubernetes.SeedScheme, kubernetes.SeedCodec, kubernetes.SeedSerializer)
	)

	if err := addPriorityClasses(registry); err != nil {
		return nil, err
	}

	return registry.SerializedObjects()
}

// remember to update docs/development/priority-classes.md when making changes here
var managedPriorityClasses = []struct {
	name        string
	value       int32
	description string
}{
	{v1beta1constants.PriorityClassNameGardenSystem500, 999999500, "PriorityClass for Garden system components"},
	{v1beta1constants.PriorityClassNameGardenSystem400, 999999400, "PriorityClass for Garden system components"},
	{v1beta1constants.PriorityClassNameGardenSystem300, 999999300, "PriorityClass for Garden system components"},
	{v1beta1constants.PriorityClassNameGardenSystem200, 999999200, "PriorityClass for Garden system components"},
	{v1beta1constants.PriorityClassNameGardenSystem100, 999999100, "PriorityClass for Garden system components"},
}

func addPriorityClasses(registry *managedresources.Registry) error {
	for _, class := range managedPriorityClasses {
		if err := registry.Add(&schedulingv1.PriorityClass{
			ObjectMeta: metav1.ObjectMeta{
				Name: class.name,
			},
			Description:   class.description,
			GlobalDefault: false,
			Value:         class.value,
		}); err != nil {
			return err
		}
	}

	return nil
}
