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

package clusteridentity

import (
	"context"
	"time"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/utils/managedresources"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const managedResourceControlName = "cluster-identity"

type clusterIdentity struct {
	client    client.Client
	namespace string
	identity  string
}

// New creates new instance of Deployer for a cluster identity.
func New(c client.Client, namespace, identity string) component.DeployWaiter {
	return &clusterIdentity{
		client:    c,
		namespace: namespace,
		identity:  identity,
	}
}

func (c *clusterIdentity) Deploy(ctx context.Context) error {
	var (
		registry = managedresources.NewRegistry(kubernetes.SeedScheme, kubernetes.SeedCodec, kubernetes.SeedSerializer)

		configMap = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      v1beta1constants.ClusterIdentity,
				Namespace: metav1.NamespaceSystem,
			},
			Data: map[string]string{
				v1beta1constants.ClusterIdentity: c.identity,
			},
		}
	)

	resources, err := registry.AddAllAndSerialize(configMap)
	if err != nil {
		return err
	}

	return managedresources.CreateForSeed(ctx, c.client, c.namespace, managedResourceControlName, false, resources)
}

func (c *clusterIdentity) Destroy(ctx context.Context) error {
	return managedresources.DeleteForSeed(ctx, c.client, c.namespace, managedResourceControlName)
}

// TimeoutWaitForManagedResource is the timeout used while waiting for the ManagedResources to become healthy
// or deleted.
var TimeoutWaitForManagedResource = 2 * time.Minute

func (c *clusterIdentity) Wait(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilHealthy(timeoutCtx, c.client, c.namespace, managedResourceControlName)
}

func (c *clusterIdentity) WaitCleanup(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilDeleted(timeoutCtx, c.client, c.namespace, managedResourceControlName)
}
