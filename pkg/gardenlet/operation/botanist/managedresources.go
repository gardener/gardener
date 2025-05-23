// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/go-multierror"
	"sigs.k8s.io/controller-runtime/pkg/client"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	errorsutils "github.com/gardener/gardener/pkg/utils/errors"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	"github.com/gardener/gardener/pkg/utils/retry"
)

// DeleteManagedResources deletes all managed resources labeled with `origin=gardener` from the Shoot namespace in the Seed.
func (b *Botanist) DeleteManagedResources(ctx context.Context) error {
	return b.SeedClientSet.Client().DeleteAllOf(
		ctx,
		&resourcesv1alpha1.ManagedResource{},
		client.InNamespace(b.Shoot.ControlPlaneNamespace),
		client.MatchingLabels{managedresources.LabelKeyOrigin: managedresources.LabelValueGardener},
	)
}

// WaitUntilManagedResourcesDeleted waits until all managed resources labeled with `origin=gardener` are gone or the context is cancelled.
func (b *Botanist) WaitUntilManagedResourcesDeleted(ctx context.Context) error {
	return b.waitUntilManagedResourceAreDeleted(ctx, client.InNamespace(b.Shoot.ControlPlaneNamespace), client.MatchingLabels{managedresources.LabelKeyOrigin: managedresources.LabelValueGardener})
}

// WaitUntilShootManagedResourcesDeleted waits until all managed resources that are describing shoot resources are deleted or the context is cancelled.
func (b *Botanist) WaitUntilShootManagedResourcesDeleted(ctx context.Context) error {
	return retry.Until(ctx, time.Second*5, func(ctx context.Context) (done bool, err error) {
		mrList := &resourcesv1alpha1.ManagedResourceList{}
		if err := b.SeedClientSet.Client().List(ctx, mrList, client.InNamespace(b.Shoot.ControlPlaneNamespace)); err != nil {
			return retry.SevereError(err)
		}

		allErrs := &multierror.Error{
			ErrorFormat: errorsutils.NewErrorFormatFuncWithPrefix("error while waiting for all shoot managed resources to be deleted: "),
		}
		for _, mr := range mrList.Items {
			if mr.Spec.Class == nil || *mr.Spec.Class == "" {
				allErrs = multierror.Append(allErrs, fmt.Errorf("shoot managed resource %s/%s still exists", mr.Namespace, mr.Name))
			}
		}

		if err := allErrs.ErrorOrNil(); err != nil {
			return retry.MinorError(err)
		}

		return retry.Ok()
	})
}

func (b *Botanist) waitUntilManagedResourceAreDeleted(ctx context.Context, listOpt ...client.ListOption) error {
	return managedresources.WaitUntilListDeleted(ctx, b.SeedClientSet.Client(), &resourcesv1alpha1.ManagedResourceList{}, listOpt...)
}

// KeepObjectsForManagedResources sets ManagedResource.Spec.KeepObjects to true.
func (b *Botanist) KeepObjectsForManagedResources(ctx context.Context) error {
	managedResources := &resourcesv1alpha1.ManagedResourceList{}
	if err := b.SeedClientSet.Client().List(ctx, managedResources, client.InNamespace(b.Shoot.ControlPlaneNamespace), client.MatchingLabels{managedresources.LabelKeyOrigin: managedresources.LabelValueGardener}); err != nil {
		return fmt.Errorf("failed to list all managed resource, %w", err)
	}

	for _, resource := range managedResources.Items {
		if err := managedresources.SetKeepObjects(ctx, b.SeedClientSet.Client(), resource.Namespace, resource.Name, true); err != nil {
			return err
		}
	}

	return nil
}
