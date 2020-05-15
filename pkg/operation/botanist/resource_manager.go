// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package botanist

import (
	"context"
	"fmt"
	"time"

	"github.com/gardener/gardener/pkg/operation/botanist/constants"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/retry"

	resourcesv1alpha1 "github.com/gardener/gardener-resource-manager/pkg/apis/resources/v1alpha1"
	k8sretry "k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// DeleteManagedResources deletes all managed resources labeled with `origin=gardener` from the Shoot namespace in the Seed.
func (b *Botanist) DeleteManagedResources(ctx context.Context) error {
	return b.K8sSeedClient.Client().DeleteAllOf(
		ctx,
		&resourcesv1alpha1.ManagedResource{},
		client.InNamespace(b.Shoot.SeedNamespace),
		client.MatchingLabels{constants.ManagedResourceLabelKeyOrigin: constants.ManagedResourceLabelValueGardener},
	)
}

// WaitUntilManagedResourcesDeleted waits until all managed resources labeled with `origin=gardener` are gone or the context is cancelled.
func (b *Botanist) WaitUntilManagedResourcesDeleted(ctx context.Context) error {
	return b.waitUntilManagedResourceAreDeleted(ctx, client.InNamespace(b.Shoot.SeedNamespace), client.MatchingLabels{constants.ManagedResourceLabelKeyOrigin: constants.ManagedResourceLabelValueGardener})
}

// WaitUntilAllManagedResourcesDeleted waits until all managed resources are gone or the context is cancelled.
func (b *Botanist) WaitUntilAllManagedResourcesDeleted(ctx context.Context) error {
	return b.waitUntilManagedResourceAreDeleted(ctx, client.InNamespace(b.Shoot.SeedNamespace))
}
func (b *Botanist) waitUntilManagedResourceAreDeleted(ctx context.Context, listOpt ...client.ListOption) error {
	return retry.Until(ctx, 5*time.Second, func(ctx context.Context) (done bool, err error) {
		managedResources := &resourcesv1alpha1.ManagedResourceList{}
		if err := b.K8sSeedClient.Client().List(ctx,
			managedResources,
			listOpt...); err != nil {
			return retry.SevereError(err)
		}

		if len(managedResources.Items) == 0 {
			return retry.Ok()
		}

		names := make([]string, 0, len(managedResources.Items))
		for _, resource := range managedResources.Items {
			names = append(names, resource.Name)
		}

		b.Logger.Infof("Waiting until all managed resources have been deleted in the shoot cluster...")
		return retry.MinorError(fmt.Errorf("not all managed resources have been deleted in the shoot cluster (still existing: %s)", names))
	})
}

// KeepManagedResourcesObjects sets ManagedResource.Spec.KeepObjects to true.
func (b *Botanist) KeepManagedResourcesObjects(ctx context.Context) error {
	managedResources := &resourcesv1alpha1.ManagedResourceList{}
	if err := b.K8sSeedClient.Client().List(ctx,
		managedResources,
		client.InNamespace(b.Shoot.SeedNamespace),
	); err != nil {
		return fmt.Errorf("failed to list all managed resource, %v", err)
	}

	if len(managedResources.Items) == 0 {
		return nil
	}

	for _, resource := range managedResources.Items {
		if err := kutil.TryUpdate(ctx, k8sretry.DefaultRetry, b.K8sSeedClient.Client(), &resource, func() error {
			keepObj := true
			resource.Spec.KeepObjects = &keepObj
			return nil
		}); client.IgnoreNotFound(err) != nil {
			return fmt.Errorf("failed to update managed resource %q, %v", resource.GetName(), err)
		}
	}
	return nil
}

// DeleteAllManagedResourcesObjects deletes all managed resources from the Shoot namespace in the Seed.
func (b *Botanist) DeleteAllManagedResourcesObjects(ctx context.Context) error {
	return b.K8sSeedClient.Client().DeleteAllOf(
		ctx,
		&resourcesv1alpha1.ManagedResource{},
		client.InNamespace(b.Shoot.SeedNamespace),
	)
}
