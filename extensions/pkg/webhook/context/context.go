// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package context

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
)

// GardenContext wraps the actual context and cluster object.
type GardenContext interface {
	GetCluster(ctx context.Context) (*extensionscontroller.Cluster, error)
}

type gardenContext struct {
	client  client.Client
	object  client.Object
	cluster *extensionscontroller.Cluster
}

// NewGardenContext creates a context object.
func NewGardenContext(client client.Client, object client.Object) GardenContext {
	return &gardenContext{
		client: client,
		object: object,
	}
}

// NewInternalGardenContext creates a context object from a Cluster object.
func NewInternalGardenContext(cluster *extensionscontroller.Cluster) GardenContext {
	return &gardenContext{
		cluster: cluster,
	}
}

// GetCluster returns the Cluster object.
func (c *gardenContext) GetCluster(ctx context.Context) (*extensionscontroller.Cluster, error) {
	if c.cluster == nil {
		cluster, err := extensionscontroller.GetCluster(ctx, c.client, c.object.GetNamespace())
		if err != nil {
			return nil, fmt.Errorf("could not get cluster for namespace '%s': %w", c.object.GetNamespace(), err)
		}
		c.cluster = cluster
	}
	return c.cluster, nil
}
