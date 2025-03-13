// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/component/etcd/etcd"
	"github.com/gardener/gardener/pkg/component/shared"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
	"github.com/gardener/gardener/pkg/utils/flow"
	"github.com/gardener/gardener/pkg/utils/timewindow"
)

// NewEtcd is a function exposed for testing.
var NewEtcd = etcd.New

// DefaultEtcd returns a deployer for the etcd.
func (b *Botanist) DefaultEtcd(role string, class etcd.Class) (etcd.Interface, error) {
	defragmentationSchedule, err := determineDefragmentationSchedule(b.Shoot.GetInfo(), b.ManagedSeed, class)
	if err != nil {
		return nil, err
	}

	var replicas *int32
	if !b.Shoot.HibernationEnabled {
		replicas = ptr.To(getEtcdReplicas(b.Shoot.GetInfo()))
	}

	var (
		minAllowed      corev1.ResourceList
		storageCapacity string
	)

	switch role {
	case v1beta1constants.ETCDRoleMain:
		if etcd := b.Shoot.GetInfo().Spec.Kubernetes.ETCD; etcd != nil && etcd.Main != nil && etcd.Main.Autoscaling != nil {
			minAllowed = etcd.Main.Autoscaling.MinAllowed
		}
		storageCapacity = "25Gi"
	case v1beta1constants.ETCDRoleEvents:
		if etcd := b.Shoot.GetInfo().Spec.Kubernetes.ETCD; etcd != nil && etcd.Events != nil && etcd.Events.Autoscaling != nil {
			minAllowed = etcd.Events.Autoscaling.MinAllowed
		}
		storageCapacity = "10Gi"
	}

	e := NewEtcd(
		b.Logger,
		b.SeedClientSet.Client(),
		b.Shoot.ControlPlaneNamespace,
		b.SecretsManager,
		etcd.Values{
			Role:                        role,
			Class:                       class,
			Replicas:                    replicas,
			Autoscaling:                 etcd.AutoscalingConfig{MinAllowed: minAllowed},
			StorageCapacity:             b.Seed.GetValidVolumeSize(storageCapacity),
			DefragmentationSchedule:     &defragmentationSchedule,
			CARotationPhase:             v1beta1helper.GetShootCARotationPhase(b.Shoot.GetInfo().Status.Credentials),
			RuntimeKubernetesVersion:    b.Seed.KubernetesVersion,
			MaintenanceTimeWindow:       *b.Shoot.GetInfo().Spec.Maintenance.TimeWindow,
			EvictionRequirement:         getEvictionRequirement(class, b.Shoot),
			PriorityClassName:           v1beta1constants.PriorityClassNameShootControlPlane500,
			HighAvailabilityEnabled:     v1beta1helper.IsHAControlPlaneConfigured(b.Shoot.GetInfo()),
			TopologyAwareRoutingEnabled: b.Shoot.TopologyAwareRoutingEnabled,
		},
	)

	return e, nil
}

func getEvictionRequirement(c etcd.Class, s *shoot.Shoot) *string {
	if c == etcd.ClassImportant && (s.Purpose == gardencorev1beta1.ShootPurposeProduction || s.Purpose == gardencorev1beta1.ShootPurposeInfrastructure) {
		return ptr.To(v1beta1constants.EvictionRequirementNever)
	}
	if metav1.HasAnnotation(s.GetInfo().ObjectMeta, v1beta1constants.ShootAlphaControlPlaneScaleDownDisabled) {
		return ptr.To(v1beta1constants.EvictionRequirementNever)
	}
	return ptr.To(v1beta1constants.EvictionRequirementInMaintenanceWindowOnly)
}

// DeployEtcd deploys the etcd main and events.
func (b *Botanist) DeployEtcd(ctx context.Context) error {
	if b.Seed.GetInfo().Spec.Backup != nil {
		secret := &corev1.Secret{}
		if err := b.SeedClientSet.Client().Get(ctx, client.ObjectKey{Namespace: b.Shoot.ControlPlaneNamespace, Name: v1beta1constants.BackupSecretName}, secret); err != nil {
			return err
		}

		snapshotSchedule, err := determineBackupSchedule(b.Shoot.GetInfo())
		if err != nil {
			return err
		}

		var (
			backupLeaderElection         *gardenletconfigv1alpha1.ETCDBackupLeaderElection
			deltaSnapshotRetentionPeriod *metav1.Duration
		)
		if b.Config != nil && b.Config.ETCDConfig != nil {
			backupLeaderElection = b.Config.ETCDConfig.BackupLeaderElection
			deltaSnapshotRetentionPeriod = b.Config.ETCDConfig.DeltaSnapshotRetentionPeriod
		}

		b.Shoot.Components.ControlPlane.EtcdMain.SetBackupConfig(&etcd.BackupConfig{
			Provider:                     b.Seed.GetInfo().Spec.Backup.Provider,
			SecretRefName:                v1beta1constants.BackupSecretName,
			Prefix:                       b.Shoot.BackupEntryName,
			Container:                    string(secret.Data[v1beta1constants.DataKeyBackupBucketName]),
			FullSnapshotSchedule:         snapshotSchedule,
			LeaderElection:               backupLeaderElection,
			DeltaSnapshotRetentionPeriod: deltaSnapshotRetentionPeriod,
		})
	}

	// Roll out the new peer CA first so that every member in the cluster trusts the old and the new CA.
	// This is required because peer certificates which are used for client and server authentication at the same time,
	// are re-created with the new CA in the `Deploy` step.
	if sets.New(
		gardencorev1beta1.RotationPreparing,
		gardencorev1beta1.RotationPreparingWithoutWorkersRollout,
	).Has(v1beta1helper.GetShootCARotationPhase(b.Shoot.GetInfo().Status.Credentials)) {
		if err := flow.Parallel(
			b.Shoot.Components.ControlPlane.EtcdMain.RolloutPeerCA,
			b.Shoot.Components.ControlPlane.EtcdEvents.RolloutPeerCA,
		)(ctx); err != nil {
			return err
		}
	}

	return b.deployOrRestoreEtcd(ctx)
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
	return shared.SnapshotEtcd(ctx, b.SecretsManager, b.Shoot.Components.ControlPlane.EtcdMain)
}

// ScaleETCDToZero scales ETCD main and events replicas to zero.
func (b *Botanist) ScaleETCDToZero(ctx context.Context) error {
	return b.scaleETCD(ctx, 0)
}

// ScaleUpETCD scales ETCD main and events replicas to the configured replica count.
func (b *Botanist) ScaleUpETCD(ctx context.Context) error {
	return b.scaleETCD(ctx, getEtcdReplicas(b.Shoot.GetInfo()))
}

func (b *Botanist) scaleETCD(ctx context.Context, replicas int32) error {
	if err := b.Shoot.Components.ControlPlane.EtcdMain.Scale(ctx, replicas); err != nil {
		return err
	}
	return b.Shoot.Components.ControlPlane.EtcdEvents.Scale(ctx, replicas)
}

func (b *Botanist) deployOrRestoreEtcd(ctx context.Context) error {
	isRestoreRequired, err := b.isRestorationOfMultiNodeMainEtcdRequired(ctx)
	if err != nil {
		return err
	}

	if isRestoreRequired {
		return b.restoreMultiNodeEtcd(ctx)
	}

	return flow.Parallel(
		b.Shoot.Components.ControlPlane.EtcdMain.Deploy,
		b.Shoot.Components.ControlPlane.EtcdEvents.Deploy,
	)(ctx)
}

func (b *Botanist) isRestorationOfMultiNodeMainEtcdRequired(ctx context.Context) (bool, error) {
	if !b.IsRestorePhase() || !v1beta1helper.IsHAControlPlaneConfigured(b.Shoot.GetInfo()) {
		return false, nil
	}

	etcd, err := b.Shoot.Components.ControlPlane.EtcdMain.Get(ctx)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return true, nil
		}
		return false, err
	}

	// The etcd has already been scaled up to the desired number of replicas
	// and therefore has been restored, then it has been scaled down to 0
	// replicas as part of the reconciliation flow for hibernated shoots.
	if b.Shoot.HibernationEnabled && etcd.Spec.Replicas == 0 {
		return false, nil
	}
	// The etcd has already been scaled up to the desired number of replicas
	// and therefore has been restored.
	if etcd.Spec.Replicas == getEtcdReplicas(b.Shoot.GetInfo()) {
		return false, nil
	}

	return true, nil
}

func (b *Botanist) restoreMultiNodeEtcd(ctx context.Context) error {
	for _, component := range []etcd.Interface{b.Shoot.Components.ControlPlane.EtcdMain, b.Shoot.Components.ControlPlane.EtcdEvents} {
		originalReplicas := component.GetReplicas()
		defer func() {
			// Revert the original replica count for the etcd. This is done in case a step
			// is added to the reconciliation flow that depends on the etcd's replica count.
			component.SetReplicas(originalReplicas)
		}()
		component.SetReplicas(ptr.To[int32](1))
	}

	return flow.Parallel(
		b.Shoot.Components.ControlPlane.EtcdMain.Deploy,
		b.Shoot.Components.ControlPlane.EtcdEvents.Deploy,
	)(ctx)
}

func determineBackupSchedule(shoot *gardencorev1beta1.Shoot) (string, error) {
	return timewindow.DetermineSchedule(
		"%d %d * * *",
		shoot.Spec.Maintenance.TimeWindow.Begin,
		shoot.Spec.Maintenance.TimeWindow.End,
		shoot.Status.UID,
		shoot.CreationTimestamp,
		timewindow.RandomizeWithinFirstHourOfTimeWindow,
	)
}

func determineDefragmentationSchedule(shoot *gardencorev1beta1.Shoot, managedSeed *seedmanagementv1alpha1.ManagedSeed, class etcd.Class) (string, error) {
	scheduleFormat := "%d %d */3 * *"
	if managedSeed != nil && class == etcd.ClassImportant {
		// defrag important etcds of ManagedSeeds daily in the maintenance window
		scheduleFormat = "%d %d * * *"
	}

	return timewindow.DetermineSchedule(
		scheduleFormat,
		shoot.Spec.Maintenance.TimeWindow.Begin,
		shoot.Spec.Maintenance.TimeWindow.End,
		shoot.Status.UID,
		shoot.CreationTimestamp,
		timewindow.RandomizeWithinTimeWindow,
	)
}

func getEtcdReplicas(shoot *gardencorev1beta1.Shoot) int32 {
	if v1beta1helper.IsHAControlPlaneConfigured(shoot) {
		return 3
	}
	return 1
}
