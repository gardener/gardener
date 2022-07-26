// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"

	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// AddProjectNamespace adds an index for core.ProjectNamespace to the given indexer.
func AddProjectNamespace(ctx context.Context, indexer client.FieldIndexer) error {
	if err := indexer.IndexField(ctx, &gardencorev1beta1.Project{}, core.ProjectNamespace, func(obj client.Object) []string {
		project, ok := obj.(*gardencorev1beta1.Project)
		if !ok {
			return []string{""}
		}
		return []string{pointer.StringDeref(project.Spec.Namespace, "")}
	}); err != nil {
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

// AddBackupBucketSeedName adds an index for core.BackupBucketSeedName to the given indexer.
func AddBackupBucketSeedName(ctx context.Context, indexer client.FieldIndexer) error {
	if err := indexer.IndexField(ctx, &gardencorev1beta1.BackupBucket{}, core.BackupBucketSeedName, func(obj client.Object) []string {
		backupBucket, ok := obj.(*gardencorev1beta1.BackupBucket)
		if !ok {
			return []string{""}
		}
		return []string{pointer.StringDeref(backupBucket.Spec.SeedName, "")}
	}); err != nil {
		return fmt.Errorf("failed to add indexer for %s to BackupBucket Informer: %w", core.BackupBucketSeedName, err)
	}
	return nil
}

// AddControllerInstallationSeedRefName adds an index for core.ControllerInstallationSeedRefName to the given indexer.
func AddControllerInstallationSeedRefName(ctx context.Context, indexer client.FieldIndexer) error {
	if err := indexer.IndexField(ctx, &gardencorev1beta1.ControllerInstallation{}, core.SeedRefName, func(obj client.Object) []string {
		controllerInstallation, ok := obj.(*gardencorev1beta1.ControllerInstallation)
		if !ok {
			return []string{""}
		}
		return []string{controllerInstallation.Spec.SeedRef.Name}
	}); err != nil {
		return fmt.Errorf("failed to add indexer for %s to ControllerInstallation Informer: %w", core.SeedRefName, err)
	}
	return nil
}
