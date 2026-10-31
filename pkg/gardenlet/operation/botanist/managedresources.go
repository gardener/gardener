// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/go-multierror"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	errorsutils "github.com/gardener/gardener/pkg/utils/errors"
	utilclient "github.com/gardener/gardener/pkg/utils/kubernetes/client"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	"github.com/gardener/gardener/pkg/utils/retry"
)

// DeleteManagedResources deletes all managed resources labeled with `origin=gardener`
// from the Shoot namespace in the Seed. If a managed resource is stuck in deletion for
// longer than the configured grace period, it is annotated for force
// deletion so that the gardener-resource-manager forcefully cleans up the underlying
// resources.
func (b *Botanist) DeleteManagedResources(ctx context.Context) error {
	var (
		c       = b.SeedClientSet.Client()
		ensurer = utilclient.DefaultGoneEnsurer()
		cleaner = utilclient.NewCleaner(b.Clock, &managedresources.ForceDeletionFinalizer{})
		ops     = utilclient.NewCleanOps(ensurer, cleaner)
	)

	cleanOptions, err := b.getCleanOptions(
		GracePeriodFiveMinutes,
		FinalizeAfterFiveMinutes,
		v1beta1constants.AnnotationShootCleanupManagedResourcesFinalizeGracePeriodSeconds,
		1,
	)
	if err != nil {
		return err
	}

	return cleanResourceFn(ops, b.Logger, c, &resourcesv1alpha1.ManagedResourceList{},
		utilclient.ListWith{
			client.InNamespace(b.Shoot.ControlPlaneNamespace),
			client.MatchingLabels{managedresources.LabelKeyOrigin: managedresources.LabelValueGardener},
		},
		cleanOptions,
	)(ctx)
}

// DeleteManagedResourcesAndWait deletes all managed resources labeled with `origin=gardener`
// from the Shoot namespace in the Seed without applying force deletion.
func (b *Botanist) DeleteManagedResourcesAndWait(ctx context.Context) error {
	return b.SeedClientSet.Client().DeleteAllOf(
		ctx,
		&resourcesv1alpha1.ManagedResource{},
		client.InNamespace(b.Shoot.ControlPlaneNamespace),
		client.MatchingLabels{managedresources.LabelKeyOrigin: managedresources.LabelValueGardener},
	)
}

// WaitUntilManagedResourcesDeleted waits until all managed resources labeled with `origin=gardener` are gone or the context is cancelled.
func (b *Botanist) WaitUntilManagedResourcesDeleted(ctx context.Context) error {
	return managedresources.WaitUntilListDeleted(ctx, b.SeedClientSet.Client(), &resourcesv1alpha1.ManagedResourceList{},
		client.InNamespace(b.Shoot.ControlPlaneNamespace),
		client.MatchingLabels{managedresources.LabelKeyOrigin: managedresources.LabelValueGardener},
	)
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
