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

package botanist

import (
	"context"
	"fmt"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/features"
	gardenletfeatures "github.com/gardener/gardener/pkg/gardenlet/features"
	corebackupentry "github.com/gardener/gardener/pkg/operation/botanist/component/backupentry"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// DefaultCoreBackupEntry creates the default deployer for the core.gardener.cloud/v1beta1.BackupEntry resource.
func (b *Botanist) DefaultCoreBackupEntry() corebackupentry.Interface {
	ownerRef := metav1.NewControllerRef(b.Shoot.GetInfo(), gardencorev1beta1.SchemeGroupVersion.WithKind("Shoot"))
	ownerRef.BlockOwnerDeletion = pointer.Bool(false)

	return corebackupentry.New(
		b.Logger,
		b.K8sGardenClient.Client(),
		&corebackupentry.Values{
			Namespace:      b.Shoot.GetInfo().Namespace,
			Name:           b.Shoot.BackupEntryName,
			ShootPurpose:   b.Shoot.GetInfo().Spec.Purpose,
			OwnerReference: ownerRef,
			SeedName:       b.Shoot.GetInfo().Spec.SeedName,
			BucketName:     string(b.Seed.GetInfo().UID),
		},
		corebackupentry.DefaultInterval,
		corebackupentry.DefaultTimeout,
	)
}

// DeployBackupEntry deploys the BackupEntry resource in the Garden cluster and triggers the restore operation in case
// the Shoot is in the restore phase of the control plane migration.
func (b *Botanist) DeployBackupEntry(ctx context.Context) error {
	if b.isRestorePhase() {
		return b.Shoot.Components.BackupEntry.Restore(ctx, b.GetShootState())
	}
	return b.Shoot.Components.BackupEntry.Deploy(ctx)
}

// SourceBackupEntry creates a deployer for a core.gardener.cloud/v1beta1.BackupEntry resource which will be used
// as source when copying etcd backups.
func (b *Botanist) SourceBackupEntry() corebackupentry.Interface {
	ownerRef := metav1.NewControllerRef(b.Shoot.GetInfo(), gardencorev1beta1.SchemeGroupVersion.WithKind("Shoot"))
	ownerRef.BlockOwnerDeletion = pointer.Bool(false)

	return corebackupentry.New(
		b.Logger,
		b.K8sGardenClient.Client(),
		&corebackupentry.Values{
			Namespace:      b.Shoot.GetInfo().Namespace,
			Name:           fmt.Sprintf("%s-%s", v1beta1constants.BackupSourcePrefix, b.Shoot.BackupEntryName),
			ShootPurpose:   b.Shoot.GetInfo().Spec.Purpose,
			OwnerReference: ownerRef,
			SeedName:       b.Shoot.GetInfo().Spec.SeedName,
		},
		corebackupentry.DefaultInterval,
		corebackupentry.DefaultTimeout,
	)
}

// DeploySourceBackupEntry deploys the source BackupEntry and sets its bucketName to be equal to the bucketName of the shoot's original
// BackupEntry if the source BackupEntry doesn't already exist.
func (b *Botanist) DeploySourceBackupEntry(ctx context.Context) error {
	bucketName := b.Shoot.Components.BackupEntry.GetActualBucketName()
	if _, err := b.Shoot.Components.SourceBackupEntry.Get(ctx); err == nil {
		bucketName = b.Shoot.Components.SourceBackupEntry.GetActualBucketName()
	} else if client.IgnoreNotFound(err) != nil {
		return err
	}

	b.Shoot.Components.SourceBackupEntry.SetBucketName(bucketName)
	return b.Shoot.Components.SourceBackupEntry.Deploy(ctx)
}

// DestroySourceBackupEntry destroys the source BackupEntry. It returns nil if the CopyEtcdBackupsDuringControlPlaneMigration feature gate
// is disabled or the Seed backup is not enabled or the Shoot is in restore phase.
func (b *Botanist) DestroySourceBackupEntry(ctx context.Context) error {
	if !gardenletfeatures.FeatureGate.Enabled(features.CopyEtcdBackupsDuringControlPlaneMigration) ||
		b.Seed.GetInfo().Spec.Backup == nil || !b.isRestorePhase() {
		return nil
	}

	return b.Shoot.Components.SourceBackupEntry.Destroy(ctx)
}
