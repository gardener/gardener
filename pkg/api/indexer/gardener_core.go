// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package indexer

import (
	"context"
	"fmt"

	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

// ProjectNamespaceIndexerFunc extracts the .spec.namespace field of a Project.
func ProjectNamespaceIndexerFunc(obj client.Object) []string {
	project, ok := obj.(*gardencorev1beta1.Project)
	if !ok {
		return []string{""}
	}
	return []string{ptr.Deref(project.Spec.Namespace, "")}
}

// BackupBucketSeedNameIndexerFunc extracts the .spec.seedName field of a BackupBucket.
func BackupBucketSeedNameIndexerFunc(obj client.Object) []string {
	backupBucket, ok := obj.(*gardencorev1beta1.BackupBucket)
	if !ok {
		return []string{""}
	}
	return []string{ptr.Deref(backupBucket.Spec.SeedName, "")}
}

// BackupEntryBucketNameIndexerFunc extracts the .spec.bucketName field of a BackupEntry.
func BackupEntryBucketNameIndexerFunc(obj client.Object) []string {
	backupEntry, ok := obj.(*gardencorev1beta1.BackupEntry)
	if !ok {
		return []string{""}
	}
	return []string{backupEntry.Spec.BucketName}
}

// ControllerInstallationSeedRefNameIndexerFunc extracts the .spec.seedRef.name field of a ControllerInstallation.
func ControllerInstallationSeedRefNameIndexerFunc(obj client.Object) []string {
	controllerInstallation, ok := obj.(*gardencorev1beta1.ControllerInstallation)
	if !ok {
		return []string{""}
	}
	return []string{controllerInstallation.Spec.SeedRef.Name}
}

// ControllerInstallationRegistrationRefNameIndexerFunc extracts the .spec.registrationRef.name field of a ControllerInstallation.
func ControllerInstallationRegistrationRefNameIndexerFunc(obj client.Object) []string {
	controllerInstallation, ok := obj.(*gardencorev1beta1.ControllerInstallation)
	if !ok {
		return []string{""}
	}
	return []string{controllerInstallation.Spec.RegistrationRef.Name}
}

// InternalSecretTypeIndexerFunc extracts the .type field of an InternalSecret.
func InternalSecretTypeIndexerFunc(obj client.Object) []string {
	internalSecret, ok := obj.(*gardencorev1beta1.InternalSecret)
	if !ok {
		return []string{""}
	}
	return []string{string(internalSecret.Type)}
}

// NamespacedCloudProfileParentRefNameIndexerFunc extracts the .spec.parent.name field of a NamespacedCloudProfile.
func NamespacedCloudProfileParentRefNameIndexerFunc(obj client.Object) []string {
	namespacedCloudProfile, ok := obj.(*gardencorev1beta1.NamespacedCloudProfile)
	if !ok {
		return []string{""}
	}
	return []string{namespacedCloudProfile.Spec.Parent.Name}
}

// AddProjectNamespace adds an index for core.ProjectNamespace to the given indexer.
func AddProjectNamespace(ctx context.Context, indexer client.FieldIndexer) error {
	if err := indexer.IndexField(ctx, &gardencorev1beta1.Project{}, core.ProjectNamespace, ProjectNamespaceIndexerFunc); err != nil {
		return fmt.Errorf("failed to add indexer for %s to Project Informer: %w", core.ProjectNamespace, err)
	}
	return nil
}

// AddShootSeedName adds an index for core.ShootSeedName to the given indexer.
func AddShootSeedName(ctx context.Context, indexer client.FieldIndexer) error {
	if err := indexer.IndexField(ctx, &gardencorev1beta1.Shoot{}, core.ShootSeedName, func(obj client.Object) []string {
		shoot, ok := obj.(*gardencorev1beta1.Shoot)
		if !ok {
			return []string{""}
		}
		return []string{ptr.Deref(shoot.Spec.SeedName, "")}
	}); err != nil {
		return fmt.Errorf("failed to add indexer for %s to Shoot Informer: %w", core.ShootSeedName, err)
	}
	return nil
}

// AddShootStatusSeedName adds an index for core.ShootStatusSeedName to the given indexer.
func AddShootStatusSeedName(ctx context.Context, indexer client.FieldIndexer) error {
	if err := indexer.IndexField(ctx, &gardencorev1beta1.Shoot{}, core.ShootStatusSeedName, func(obj client.Object) []string {
		shoot, ok := obj.(*gardencorev1beta1.Shoot)
		if !ok {
			return []string{""}
		}
		return []string{ptr.Deref(shoot.Status.SeedName, "")}
	}); err != nil {
		return fmt.Errorf("failed to add indexer for %s to Shoot Informer: %w", core.ShootStatusSeedName, err)
	}
	return nil
}

// AddBackupBucketSeedName adds an index for core.BackupBucketSeedName to the given indexer.
func AddBackupBucketSeedName(ctx context.Context, indexer client.FieldIndexer) error {
	if err := indexer.IndexField(ctx, &gardencorev1beta1.BackupBucket{}, core.BackupBucketSeedName, BackupBucketSeedNameIndexerFunc); err != nil {
		return fmt.Errorf("failed to add indexer for %s to BackupBucket Informer: %w", core.BackupBucketSeedName, err)
	}
	return nil
}

// AddBackupEntrySeedName adds an index for core.BackupEntrySeedName to the given indexer.
func AddBackupEntrySeedName(ctx context.Context, indexer client.FieldIndexer) error {
	if err := indexer.IndexField(ctx, &gardencorev1beta1.BackupEntry{}, core.BackupEntrySeedName, func(obj client.Object) []string {
		backupEntry, ok := obj.(*gardencorev1beta1.BackupEntry)
		if !ok {
			return []string{""}
		}
		return []string{ptr.Deref(backupEntry.Spec.SeedName, "")}
	}); err != nil {
		return fmt.Errorf("failed to add indexer for %s to BackupEntry Informer: %w", core.BackupEntrySeedName, err)
	}
	return nil
}

// AddBackupEntryBucketName adds an index for core.BackupEntryBucketName to the given indexer.
func AddBackupEntryBucketName(ctx context.Context, indexer client.FieldIndexer) error {
	if err := indexer.IndexField(ctx, &gardencorev1beta1.BackupEntry{}, core.BackupEntryBucketName, BackupEntryBucketNameIndexerFunc); err != nil {
		return fmt.Errorf("failed to add indexer for %s to BackupEntry Informer: %w", core.BackupEntryBucketName, err)
	}
	return nil
}

// AddControllerInstallationSeedRefName adds an index for core.ControllerInstallationSeedRefName to the given indexer.
func AddControllerInstallationSeedRefName(ctx context.Context, indexer client.FieldIndexer) error {
	if err := indexer.IndexField(ctx, &gardencorev1beta1.ControllerInstallation{}, core.SeedRefName, ControllerInstallationSeedRefNameIndexerFunc); err != nil {
		return fmt.Errorf("failed to add indexer for %s to ControllerInstallation Informer: %w", core.SeedRefName, err)
	}
	return nil
}

// AddControllerInstallationRegistrationRefName adds an index for core.ControllerInstallationRegistrationRefName to the given indexer.
func AddControllerInstallationRegistrationRefName(ctx context.Context, indexer client.FieldIndexer) error {
	if err := indexer.IndexField(ctx, &gardencorev1beta1.ControllerInstallation{}, core.RegistrationRefName, ControllerInstallationRegistrationRefNameIndexerFunc); err != nil {
		return fmt.Errorf("failed to add indexer for %s to ControllerInstallation Informer: %w", core.RegistrationRefName, err)
	}
	return nil
}

// AddInternalSecretType adds an index for core.InternalSecretType to the given indexer.
func AddInternalSecretType(ctx context.Context, indexer client.FieldIndexer) error {
	if err := indexer.IndexField(ctx, &gardencorev1beta1.InternalSecret{}, core.InternalSecretType, InternalSecretTypeIndexerFunc); err != nil {
		return fmt.Errorf("failed to add indexer for %s to InternalSecret Informer: %w", core.InternalSecretType, err)
	}
	return nil
}

// AddNamespacedCloudProfileParentRefName adds an index for core.NamespacedCloudProfileParentRefName to the given indexer.
func AddNamespacedCloudProfileParentRefName(ctx context.Context, indexer client.FieldIndexer) error {
	if err := indexer.IndexField(ctx, &gardencorev1beta1.NamespacedCloudProfile{}, core.NamespacedCloudProfileParentRefName, NamespacedCloudProfileParentRefNameIndexerFunc); err != nil {
		return fmt.Errorf("failed to add indexer for %s to NamespacedCloudProfile Informer: %w", core.NamespacedCloudProfileParentRefName, err)
	}
	return nil
}
