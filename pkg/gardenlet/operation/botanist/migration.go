// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"
	"fmt"

	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/utils/flow"
)

// MigrateExtensionResourcesInParallel migrates extension CRs.
// CRs with kind "Extension" are handled separately and are not migrated by this function.
func (b *Botanist) MigrateExtensionResourcesInParallel(ctx context.Context) (err error) {
	return b.runParallelTaskForEachComponent(ctx, b.Shoot.GetExtensionComponentsForParallelMigration(), func(c component.DeployMigrateWaiter) func(context.Context) error {
		return c.Migrate
	})
}

// WaitUntilExtensionResourcesMigrated waits until extension CRs have been successfully migrated.
// CRs with kind "Extension" are handled separately and are not waited by this function.
func (b *Botanist) WaitUntilExtensionResourcesMigrated(ctx context.Context) error {
	return b.runParallelTaskForEachComponent(ctx, b.Shoot.GetExtensionComponentsForParallelMigration(), func(c component.DeployMigrateWaiter) func(context.Context) error {
		return c.WaitMigrate
	})
}

// DestroyExtensionResourcesInParallel deletes extension CRs from the Shoot namespace.
// CRs with kind "Extension" are handled separately and are not deleted by this function.
func (b *Botanist) DestroyExtensionResourcesInParallel(ctx context.Context) error {
	return b.runParallelTaskForEachComponent(ctx, b.Shoot.GetExtensionComponentsForParallelMigration(), func(c component.DeployMigrateWaiter) func(context.Context) error {
		return c.Destroy
	})
}

// WaitUntilExtensionResourcesDeleted waits until extension CRs have been deleted from the Shoot namespace.
// CRs with kind "Extension" are handled separately and are not waited by this function.
func (b *Botanist) WaitUntilExtensionResourcesDeleted(ctx context.Context) error {
	return b.runParallelTaskForEachComponent(ctx, b.Shoot.GetExtensionComponentsForParallelMigration(), func(c component.DeployMigrateWaiter) func(context.Context) error {
		return c.WaitCleanup
	})
}

// DestroyDNSRecords deletes all DNSRecord resources from the Shoot namespace.
func (b *Botanist) DestroyDNSRecords(ctx context.Context) error {
	return b.runParallelTaskForEachComponent(ctx, b.Shoot.GetDNSRecordComponentsForMigration(), func(c component.DeployMigrateWaiter) func(context.Context) error {
		return c.Destroy
	})
}

func (b *Botanist) runParallelTaskForEachComponent(ctx context.Context, components []component.DeployMigrateWaiter, fn func(component.DeployMigrateWaiter) func(context.Context) error) error {
	var fns []flow.TaskFn
	for _, component := range components {
		fns = append(fns, fn(component))
	}
	return flow.Parallel(fns...)(ctx)
}

// IsCopyOfBackupsRequired check if etcd backups need to be copied between seeds.
func (b *Botanist) IsCopyOfBackupsRequired(ctx context.Context) (bool, error) {
	if b.Seed.GetInfo().Spec.Backup == nil || !b.IsRestorePhase() {
		return false, nil
	}

	// First we check if the etcd-main Etcd resource has been created. This is only true if backups have been copied.
	if _, err := b.Shoot.Components.ControlPlane.EtcdMain.Get(ctx); client.IgnoreNotFound(err) != nil {
		return false, err
	} else if err == nil {
		return false, nil
	}

	backupEntry, err := b.Shoot.Components.BackupEntry.Get(ctx)
	if err != nil {
		return false, fmt.Errorf("error while retrieving BackupEntry: %w", err)
	}

	// If the Shoot's original BackupEntry has not been switched to the destination Seed's BackupBucket, then backup copying has not been started yet
	// and the source BackupEntry has not been created.
	if backupEntry.Spec.BucketName != string(b.Seed.GetInfo().UID) {
		return true, nil
	}

	sourceBackupEntry, err := b.Shoot.Components.SourceBackupEntry.Get(ctx)
	if err != nil {
		return false, fmt.Errorf("error while retrieving source BackupEntry: %w", err)
	}

	// If the source BackupEntry exists, then the Shoot's original BackupEntry must have had its bucketName switched to the BackupBucket of the
	// destination Seed and the source BackupEntry's bucketName must point to the BackupBucket of the source seed. Otherwise copy of backups is
	// impossible and data loss will occur.
	if sourceBackupEntry.Spec.BucketName == backupEntry.Spec.BucketName {
		return false, fmt.Errorf("backups have not been copied and source and target backupentry point to the same bucket: %s. ", sourceBackupEntry.Spec.BucketName)
	}

	return true, nil
}

// IsRestorePhase returns true when the shoot is in phase 'restore'.
func (b *Botanist) IsRestorePhase() bool {
	return v1beta1helper.ShootHasOperationType(b.Shoot.GetInfo().Status.LastOperation, gardencorev1beta1.LastOperationTypeRestore)
}

// ShallowDeleteMachineResources deletes all machine-related resources by forcefully removing their finalizers.
func (b *Botanist) ShallowDeleteMachineResources(ctx context.Context) error {
	var taskFns []flow.TaskFn

	for _, v := range []struct {
		objectKind    string
		objectList    client.ObjectList
		listOptions   []client.ListOption
		keepResources bool
	}{
		{
			objectKind: "Machine",
			objectList: &machinev1alpha1.MachineList{},
		},
		{
			objectKind: "MachineSet",
			objectList: &machinev1alpha1.MachineSetList{},
		},
		{
			objectKind: "MachineDeployment",
			objectList: &machinev1alpha1.MachineDeploymentList{},
		},
		{
			objectKind: "MachineClass",
			objectList: &machinev1alpha1.MachineClassList{},
		},
		{
			objectKind:  "Secret",
			objectList:  &corev1.SecretList{},
			listOptions: []client.ListOption{client.MatchingLabels(map[string]string{v1beta1constants.GardenerPurpose: v1beta1constants.GardenPurposeMachineClass})},
		},
		{
			objectKind:    "Secret",
			objectList:    &corev1.SecretList{},
			listOptions:   []client.ListOption{client.MatchingLabels(map[string]string{v1beta1constants.GardenerPurpose: v1beta1constants.SecretNameCloudProvider})},
			keepResources: true,
		},
	} {
		log := b.Logger.WithValues("kind", v.objectKind)
		log.Info("Shallow deleting all objects of kind")

		if err := b.SeedClientSet.Client().List(ctx, v.objectList, append(v.listOptions, client.InNamespace(b.Shoot.ControlPlaneNamespace))...); err != nil {
			return err
		}

		if err := meta.EachListItem(v.objectList, func(obj runtime.Object) error {
			object := obj.(client.Object)
			keep := v.keepResources

			taskFns = append(taskFns, func(ctx context.Context) error {
				log.Info("Removing machine-controller-manager finalizers from object", "object", client.ObjectKeyFromObject(object))
				if err := controllerutils.RemoveFinalizers(ctx, b.SeedClientSet.Client(), object, "machine.sapcloud.io/machine-controller-manager", "machine.sapcloud.io/machine-controller"); err != nil {
					return fmt.Errorf("failed to remove machine-controller-manager finalizers from secret %s: %w", client.ObjectKeyFromObject(object), err)
				}
				if keep {
					return nil
				}
				return client.IgnoreNotFound(b.SeedClientSet.Client().Delete(ctx, object))
			})

			return nil
		}); err != nil {
			return fmt.Errorf("failed computing task functions for shallow deletion of all %ss: %w", v.objectKind, err)
		}
	}

	return flow.Parallel(taskFns...)(ctx)
}
