// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	corebackupentry "github.com/gardener/gardener/pkg/component/garden/backupentry"
)

// DefaultCoreBackupEntry creates the default deployer for the core.gardener.cloud/v1beta1.BackupEntry resource.
func (b *Botanist) DefaultCoreBackupEntry() corebackupentry.Interface {
	ownerRef := metav1.NewControllerRef(b.Shoot.GetInfo(), gardencorev1beta1.SchemeGroupVersion.WithKind("Shoot"))
	ownerRef.BlockOwnerDeletion = ptr.To(false)

	return corebackupentry.New(
		b.Logger,
		b.GardenClient,
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
	if b.IsRestorePhase() {
		return b.Shoot.Components.BackupEntry.Restore(ctx, b.Shoot.GetShootState())
	}
	return b.Shoot.Components.BackupEntry.Deploy(ctx)
}

// SourceBackupEntry creates a deployer for a core.gardener.cloud/v1beta1.BackupEntry resource which will be used
// as source when copying etcd backups.
func (b *Botanist) SourceBackupEntry() corebackupentry.Interface {
	ownerRef := metav1.NewControllerRef(b.Shoot.GetInfo(), gardencorev1beta1.SchemeGroupVersion.WithKind("Shoot"))
	ownerRef.BlockOwnerDeletion = ptr.To(false)

	return corebackupentry.New(
		b.Logger,
		b.GardenClient,
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

// DestroySourceBackupEntry destroys the source BackupEntry.
func (b *Botanist) DestroySourceBackupEntry(ctx context.Context) error {
	return b.Shoot.Components.SourceBackupEntry.Destroy(ctx)
}
