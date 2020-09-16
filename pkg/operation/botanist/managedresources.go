// SPDX-FileCopyrightText: 2018 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"
	"fmt"
	"time"

	"github.com/gardener/gardener/pkg/operation/common"
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
		client.MatchingLabels{common.ManagedResourceLabelKeyOrigin: common.ManagedResourceLabelValueGardener},
	)
}

// WaitUntilManagedResourcesDeleted waits until all managed resources labeled with `origin=gardener` are gone or the context is cancelled.
func (b *Botanist) WaitUntilManagedResourcesDeleted(ctx context.Context) error {
	return b.waitUntilManagedResourceAreDeleted(ctx, client.InNamespace(b.Shoot.SeedNamespace), client.MatchingLabels{common.ManagedResourceLabelKeyOrigin: common.ManagedResourceLabelValueGardener})
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
		if err := kutil.TryUpdate(ctx, k8sretry.DefaultBackoff, b.K8sSeedClient.DirectClient(), &resource, func() error {
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
