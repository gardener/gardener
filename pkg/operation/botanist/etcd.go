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
	"hash/crc32"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/features"
	gardenletfeatures "github.com/gardener/gardener/pkg/gardenlet/features"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/operation/botanist/component/etcd"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/flow"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	hvpav1alpha1 "github.com/gardener/hvpa-controller/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// NewEtcd is a function exposed for testing.
var NewEtcd = etcd.New

// DefaultEtcd returns a deployer for the etcd.
func (b *Botanist) DefaultEtcd(role string, class etcd.Class) (etcd.Interface, error) {
	defragmentationSchedule, err := determineDefragmentationSchedule(b.Shoot.GetInfo(), b.ManagedSeed, class)
	if err != nil {
		return nil, err
	}

	e := NewEtcd(
		b.K8sSeedClient.Client(),
		b.Logger,
		b.Shoot.SeedNamespace,
		role,
		class,
		b.Shoot.HibernationEnabled,
		b.Seed.GetValidVolumeSize("10Gi"),
		&defragmentationSchedule,
	)

	hvpaEnabled := gardenletfeatures.FeatureGate.Enabled(features.HVPA)
	if b.ManagedSeed != nil {
		hvpaEnabled = gardenletfeatures.FeatureGate.Enabled(features.HVPAForShootedSeed)
	}

	scaleDownUpdateMode := hvpav1alpha1.UpdateModeMaintenanceWindow
	if (class == etcd.ClassImportant && b.Shoot.Purpose == gardencorev1beta1.ShootPurposeProduction) ||
		(metav1.HasAnnotation(b.Shoot.GetInfo().ObjectMeta, v1beta1constants.ShootAlphaControlPlaneScaleDownDisabled)) {
		scaleDownUpdateMode = hvpav1alpha1.UpdateModeOff
	}

	e.SetHVPAConfig(&etcd.HVPAConfig{
		Enabled:               hvpaEnabled,
		MaintenanceTimeWindow: *b.Shoot.GetInfo().Spec.Maintenance.TimeWindow,
		ScaleDownUpdateMode:   &scaleDownUpdateMode,
	})

	return e, nil
}

// DeployEtcd deploys the etcd main and events.
func (b *Botanist) DeployEtcd(ctx context.Context) error {
	secrets := etcd.Secrets{
		CA:     component.Secret{Name: etcd.SecretNameCA, Checksum: b.LoadCheckSum(etcd.SecretNameCA)},
		Server: component.Secret{Name: etcd.SecretNameServer, Checksum: b.LoadCheckSum(etcd.SecretNameServer)},
		Client: component.Secret{Name: etcd.SecretNameClient, Checksum: b.LoadCheckSum(etcd.SecretNameClient)},
	}

	b.Shoot.Components.ControlPlane.EtcdMain.SetSecrets(secrets)
	b.Shoot.Components.ControlPlane.EtcdEvents.SetSecrets(secrets)

	if b.Seed.GetInfo().Spec.Backup != nil {
		secret := &corev1.Secret{}
		if err := b.K8sSeedClient.Client().Get(ctx, kutil.Key(b.Shoot.SeedNamespace, v1beta1constants.BackupSecretName), secret); err != nil {
			return err
		}

		snapshotSchedule, err := determineBackupSchedule(b.Shoot.GetInfo())
		if err != nil {
			return err
		}

		b.Shoot.Components.ControlPlane.EtcdMain.SetBackupConfig(&etcd.BackupConfig{
			Provider:             b.Seed.GetInfo().Spec.Backup.Provider,
			SecretRefName:        v1beta1constants.BackupSecretName,
			Prefix:               b.Shoot.BackupEntryName,
			Container:            string(secret.Data[v1beta1constants.DataKeyBackupBucketName]),
			FullSnapshotSchedule: snapshotSchedule,
		})
	}

	return flow.Parallel(
		b.Shoot.Components.ControlPlane.EtcdMain.Deploy,
		b.Shoot.Components.ControlPlane.EtcdEvents.Deploy,
	)(ctx)
}

// WaitUntilEtcdsReady waits until both etcd-main and etcd-events are ready.
func (b *Botanist) WaitUntilEtcdsReady(ctx context.Context) error {
	return flow.Parallel(
		b.Shoot.Components.ControlPlane.EtcdMain.Wait,
		b.Shoot.Components.ControlPlane.EtcdEvents.Wait,
	)(ctx)
}

// DestroyEtcd destroys the etcd main and events.
func (b *Botanist) DestroyEtcd(ctx context.Context) error {
	return flow.Parallel(
		b.Shoot.Components.ControlPlane.EtcdMain.Destroy,
		b.Shoot.Components.ControlPlane.EtcdEvents.Destroy,
	)(ctx)
}

// WaitUntilEtcdsDeleted waits until both etcd-main and etcd-events are deleted.
func (b *Botanist) WaitUntilEtcdsDeleted(ctx context.Context) error {
	return flow.Parallel(
		b.Shoot.Components.ControlPlane.EtcdMain.WaitCleanup,
		b.Shoot.Components.ControlPlane.EtcdEvents.WaitCleanup,
	)(ctx)
}

// SnapshotEtcd executes into the etcd-main pod and triggers a full snapshot.
func (b *Botanist) SnapshotEtcd(ctx context.Context) error {
	return b.Shoot.Components.ControlPlane.EtcdMain.Snapshot(ctx, kubernetes.NewPodExecutor(b.K8sSeedClient.RESTConfig()))
}

// ScaleETCDToZero scales ETCD main and events replicas to zero.
func (b *Botanist) ScaleETCDToZero(ctx context.Context) error {
	return b.scaleETCD(ctx, 0)
}

// ScaleETCDToOne scales ETCD main and events replicas to one.
func (b *Botanist) ScaleETCDToOne(ctx context.Context) error {
	return b.scaleETCD(ctx, 1)
}

func (b *Botanist) scaleETCD(ctx context.Context, replicas int) error {
	for _, etcd := range []string{v1beta1constants.ETCDEvents, v1beta1constants.ETCDMain} {
		if err := kubernetes.ScaleEtcd(ctx, b.K8sSeedClient.Client(), kutil.Key(b.Shoot.SeedNamespace, etcd), replicas); client.IgnoreNotFound(err) != nil {
			return err
		}
	}
	return nil
}

func determineBackupSchedule(shoot *gardencorev1beta1.Shoot) (string, error) {
	schedule := "%d %d * * *"

	return determineSchedule(shoot, schedule, func(maintenanceTimeWindow *utils.MaintenanceTimeWindow, shootUID types.UID) string {
		// Randomize the snapshot timing daily but within last hour.
		// The 15 minutes buffer is set to snapshot upload time before actual maintenance window start.
		snapshotWindowBegin := maintenanceTimeWindow.Begin().Add(-1, -15, 0)
		randomMinutes := int(crc32.ChecksumIEEE([]byte(shootUID)) % 60)
		snapshotTime := snapshotWindowBegin.Add(0, randomMinutes, 0)
		return fmt.Sprintf(schedule, snapshotTime.Minute(), snapshotTime.Hour())
	})
}

func determineDefragmentationSchedule(shoot *gardencorev1beta1.Shoot, managedSeed *seedmanagementv1alpha1.ManagedSeed, class etcd.Class) (string, error) {
	schedule := "%d %d */3 * *"
	if managedSeed != nil && class == etcd.ClassImportant {
		// defrag important etcds of shooted seeds daily in the maintenance window
		schedule = "%d %d * * *"
	}

	return determineSchedule(shoot, schedule, func(maintenanceTimeWindow *utils.MaintenanceTimeWindow, shootUID types.UID) string {
		// Randomize the defragmentation timing but within the maintenance window.
		maintenanceWindowBegin := maintenanceTimeWindow.Begin()
		windowInMinutes := uint32(maintenanceTimeWindow.Duration().Minutes())
		randomMinutes := int(crc32.ChecksumIEEE([]byte(shootUID)) % windowInMinutes)
		maintenanceTime := maintenanceWindowBegin.Add(0, randomMinutes, 0)
		return fmt.Sprintf(schedule, maintenanceTime.Minute(), maintenanceTime.Hour())
	})
}

func determineSchedule(shoot *gardencorev1beta1.Shoot, schedule string, f func(*utils.MaintenanceTimeWindow, types.UID) string) (string, error) {
	var (
		begin, end string
		shootUID   types.UID
	)

	if shoot.Spec.Maintenance != nil && shoot.Spec.Maintenance.TimeWindow != nil {
		begin = shoot.Spec.Maintenance.TimeWindow.Begin
		end = shoot.Spec.Maintenance.TimeWindow.End
		shootUID = shoot.Status.UID
	}

	if len(begin) != 0 && len(end) != 0 {
		maintenanceTimeWindow, err := utils.ParseMaintenanceTimeWindow(begin, end)
		if err != nil {
			return "", err
		}

		if !maintenanceTimeWindow.Equal(utils.AlwaysTimeWindow) {
			return f(maintenanceTimeWindow, shootUID), nil
		}
	}

	creationMinute := shoot.CreationTimestamp.Minute()
	creationHour := shoot.CreationTimestamp.Hour()
	return fmt.Sprintf(schedule, creationMinute, creationHour), nil
}
