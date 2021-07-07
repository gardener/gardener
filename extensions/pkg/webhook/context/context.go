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

package context

import (
	"context"
	"fmt"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"

	"sigs.k8s.io/controller-runtime/pkg/client"
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
