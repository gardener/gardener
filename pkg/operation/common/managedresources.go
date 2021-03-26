// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package common

import (
	"context"

	"github.com/gardener/gardener/pkg/utils/managedresources"

	"github.com/gardener/gardener-resource-manager/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// DeployManagedResourceForShoot deploys a ManagedResource CR for the shoot's gardener-resource-manager.
func DeployManagedResourceForShoot(ctx context.Context, c client.Client, name, namespace string, keepObjects bool, data map[string][]byte) error {
	return deployManagedResource(ctx, c, name, namespace, data, managedresources.NewForShoot(c, name, namespace, keepObjects))
}

// DeleteManagedResourceForShoot deploys a ManagedResource CR for the shoot's gardener-resource-manager.
func DeleteManagedResourceForShoot(ctx context.Context, c client.Client, name, namespace string) error {
	return managedresources.Delete(ctx, c, namespace, name, true)
}

// DeployManagedResourceForSeed deploys a ManagedResource CR for the seed's gardener-resource-manager.
func DeployManagedResourceForSeed(ctx context.Context, c client.Client, name, namespace string, keepObjects bool, data map[string][]byte) error {
	return deployManagedResource(ctx, c, name, namespace, data, managedresources.NewForSeed(c, name, namespace, keepObjects))
}

// DeleteManagedResourceForSeed deploys a ManagedResource CR for the seed's gardener-resource-manager.
func DeleteManagedResourceForSeed(ctx context.Context, c client.Client, name, namespace string) error {
	return managedresources.Delete(ctx, c, namespace, name, true)
}

func deployManagedResource(ctx context.Context, c client.Client, name, namespace string, data map[string][]byte, managedResource *manager.ManagedResource) error {
	secretName, secret := managedresources.NewSecret(c, name, namespace, data, true)

	if err := secret.Reconcile(ctx); err != nil {
		return err
	}

	return managedResource.WithSecretRef(secretName).Reconcile(ctx)
}
