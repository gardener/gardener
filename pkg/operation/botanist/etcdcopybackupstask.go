// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/operation/botanist/component/etcdcopybackupstask"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	druidv1alpha1 "github.com/gardener/etcd-druid/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

// NewEtcdCopyBackupsTask is a function exposed for testing.
var NewEtcdCopyBackupsTask = etcdcopybackupstask.New

// DefaultEtcdCopyBackupsTask creates the default deployer for the EtcdCopyBackupsTask resource.
func (b *Botanist) DefaultEtcdCopyBackupsTask() etcdcopybackupstask.Interface {
	return NewEtcdCopyBackupsTask(
		b.Logger,
		b.K8sSeedClient.Client(),
		&etcdcopybackupstask.Values{
			Name:      b.Shoot.GetInfo().Name,
			Namespace: b.Shoot.SeedNamespace,
			WaitForFinalSnapshot: &druidv1alpha1.WaitForFinalSnapshotSpec{
				Enabled: true,
				Timeout: &metav1.Duration{Duration: etcdcopybackupstask.DefaultWaitForFinalSnapshotTimeout},
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
	if err := b.K8sSeedClient.Client().Get(ctx, kutil.Key(sourceBackupEntryName), sourceBackupEntry); err != nil {
		return err
	}
	sourceSecretName := fmt.Sprintf("%s-%s", v1beta1constants.BackupSourcePrefix, v1beta1constants.BackupSecretName)
	sourceSecret := &corev1.Secret{}
	if err := b.K8sSeedClient.Client().Get(ctx, kutil.Key(b.Shoot.SeedNamespace, sourceSecretName), sourceSecret); err != nil {
		return err
	}
	secret := &corev1.Secret{}
	if err := b.K8sSeedClient.Client().Get(ctx, kutil.Key(b.Shoot.SeedNamespace, v1beta1constants.BackupSecretName), secret); err != nil {
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
