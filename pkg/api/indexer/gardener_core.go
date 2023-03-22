// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package indexer

import (
	"context"
	"fmt"

	"k8s.io/utils/pointer"
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
	return []string{pointer.StringDeref(project.Spec.Namespace, "")}
}

// BackupBucketSeedNameIndexerFunc extracts the .spec.seedName field of a BackupBucket.
func BackupBucketSeedNameIndexerFunc(obj client.Object) []string {
	backupBucket, ok := obj.(*gardencorev1beta1.BackupBucket)
	if !ok {
		return []string{""}
	}
	return []string{pointer.StringDeref(backupBucket.Spec.SeedName, "")}
}

// ControllerInstallationSeedRefNameIndexerFunc extracts the .spec.seedRef.name field of a ControllerInstallation.
func ControllerInstallationSeedRefNameIndexerFunc(obj client.Object) []string {
	controllerInstallation, ok := obj.(*gardencorev1beta1.ControllerInstallation)
	if !ok {
		return []string{""}
	}
	return []string{controllerInstallation.Spec.SeedRef.Name}
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
		return []string{pointer.StringDeref(shoot.Spec.SeedName, "")}
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
		return []string{pointer.StringDeref(shoot.Status.SeedName, "")}
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
		return []string{pointer.StringDeref(backupEntry.Spec.SeedName, "")}
	}); err != nil {
		return fmt.Errorf("failed to add indexer for %s to BackupEntry Informer: %w", core.BackupEntrySeedName, err)
	}
	return nil
}

// AddBackupEntryBucketName adds an index for core.BackupEntryBucketName to the given indexer.
func AddBackupEntryBucketName(ctx context.Context, indexer client.FieldIndexer) error {
	if err := indexer.IndexField(ctx, &gardencorev1beta1.BackupEntry{}, core.BackupEntryBucketName, func(obj client.Object) []string {
		backupEntry, ok := obj.(*gardencorev1beta1.BackupEntry)
		if !ok {
			return []string{""}
		}
		return []string{backupEntry.Spec.BucketName}
	}); err != nil {
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
	if err := indexer.IndexField(ctx, &gardencorev1beta1.ControllerInstallation{}, core.RegistrationRefName, func(obj client.Object) []string {
		controllerInstallation, ok := obj.(*gardencorev1beta1.ControllerInstallation)
		if !ok {
			return []string{""}
		}
		return []string{controllerInstallation.Spec.RegistrationRef.Name}
	}); err != nil {
		return fmt.Errorf("failed to add indexer for %s to ControllerInstallation Informer: %w", core.RegistrationRefName, err)
	}
	return nil
}
