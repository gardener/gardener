// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"
	"fmt"

	druidv1alpha1 "github.com/gardener/etcd-druid/api/core/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	etcdcopybackupstask "github.com/gardener/gardener/pkg/component/etcd/copybackupstask"
)

// NewEtcdCopyBackupsTask is a function exposed for testing.
var NewEtcdCopyBackupsTask = etcdcopybackupstask.New

// DefaultEtcdCopyBackupsTask creates the default deployer for the EtcdCopyBackupsTask resource.
func (b *Botanist) DefaultEtcdCopyBackupsTask() etcdcopybackupstask.Interface {
	return NewEtcdCopyBackupsTask(
		b.Logger,
		b.SeedClientSet.Client(),
		&etcdcopybackupstask.Values{
			Name:      b.Shoot.GetInfo().Name,
			Namespace: b.Shoot.ControlPlaneNamespace,
			WaitForFinalSnapshot: &druidv1alpha1.WaitForFinalSnapshotSpec{
				Enabled: true,
				Timeout: &metav1.Duration{Duration: etcdcopybackupstask.DefaultTimeout},
			},
		},
		etcdcopybackupstask.DefaultInterval,
		etcdcopybackupstask.DefaultSevereThreshold,
		etcdcopybackupstask.DefaultTimeout,
	)
}

// DeployEtcdCopyBackupsTask sets the target and destination object stores of the EtcdCopyBackupsTask resource and deploys it.
func (b *Botanist) DeployEtcdCopyBackupsTask(ctx context.Context) error {
	if err := b.Shoot.Components.ControlPlane.EtcdCopyBackupsTask.Destroy(ctx); err != nil {
		return err
	}
	if err := b.Shoot.Components.ControlPlane.EtcdCopyBackupsTask.WaitCleanup(ctx); err != nil {
		return err
	}

	sourceBackupEntryName := fmt.Sprintf("%s-%s", v1beta1constants.BackupSourcePrefix, b.Shoot.BackupEntryName)
	sourceBackupEntry := &extensionsv1alpha1.BackupEntry{}
	if err := b.SeedClientSet.Client().Get(ctx, client.ObjectKey{Name: sourceBackupEntryName}, sourceBackupEntry); err != nil {
		return err
	}
	sourceSecretName := fmt.Sprintf("%s-%s", v1beta1constants.BackupSourcePrefix, v1beta1constants.BackupSecretName)
	sourceSecret := &corev1.Secret{}
	if err := b.SeedClientSet.Client().Get(ctx, client.ObjectKey{Namespace: b.Shoot.ControlPlaneNamespace, Name: sourceSecretName}, sourceSecret); err != nil {
		return err
	}
	secret := &corev1.Secret{}
	if err := b.SeedClientSet.Client().Get(ctx, client.ObjectKey{Namespace: b.Shoot.ControlPlaneNamespace, Name: v1beta1constants.BackupSecretName}, secret); err != nil {
		return err
	}

	sourceProvider := druidv1alpha1.StorageProvider(sourceBackupEntry.Spec.Type)
	provider := druidv1alpha1.StorageProvider(b.Seed.GetInfo().Spec.Backup.Provider)
	sourceContainer := string(sourceSecret.Data[v1beta1constants.DataKeyBackupBucketName])
	container := string(secret.Data[v1beta1constants.DataKeyBackupBucketName])

	b.Shoot.Components.ControlPlane.EtcdCopyBackupsTask.SetSourceStore(druidv1alpha1.StoreSpec{
		Provider:  &sourceProvider,
		SecretRef: &corev1.SecretReference{Name: sourceSecret.Name},
		Prefix:    fmt.Sprintf("%s/etcd-%s", b.Shoot.BackupEntryName, v1beta1constants.ETCDRoleMain),
		Container: &sourceContainer,
	})
	b.Shoot.Components.ControlPlane.EtcdCopyBackupsTask.SetTargetStore(druidv1alpha1.StoreSpec{
		Provider:  &provider,
		SecretRef: &corev1.SecretReference{Name: secret.Name},
		Prefix:    fmt.Sprintf("%s/etcd-%s", b.Shoot.BackupEntryName, v1beta1constants.ETCDRoleMain),
		Container: &container,
	})

	return b.Shoot.Components.ControlPlane.EtcdCopyBackupsTask.Deploy(ctx)
}
