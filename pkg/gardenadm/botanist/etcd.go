// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	corebackupbucket "github.com/gardener/gardener/pkg/component/garden/backupbucket"
	sharedcomponent "github.com/gardener/gardener/pkg/component/shared"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	backupbucketcontroller "github.com/gardener/gardener/pkg/gardenlet/controller/backupbucket"
	imagevectorutils "github.com/gardener/gardener/pkg/utils/imagevector"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
	"github.com/gardener/gardener/pkg/utils/retry"
)

// DeployEtcdDruid deploys the etcd-druid component.
func (b *AutonomousBotanist) DeployEtcdDruid(ctx context.Context) error {
	var componentImageVectors imagevectorutils.ComponentImageVectors
	if path := os.Getenv(imagevectorutils.ComponentOverrideEnv); path != "" {
		var err error
		componentImageVectors, err = imagevectorutils.ReadComponentOverwriteFile(path)
		if err != nil {
			return fmt.Errorf("failed reading component-specific image vector override: %w", err)
		}
	}

	gardenletConfig := &gardenletconfigv1alpha1.GardenletConfiguration{}
	gardenletconfigv1alpha1.SetObjectDefaults_GardenletConfiguration(gardenletConfig)

	deployer, err := sharedcomponent.NewEtcdDruid(
		b.SeedClientSet.Client(),
		b.Shoot.ControlPlaneNamespace,
		b.Shoot.KubernetesVersion,
		componentImageVectors,
		gardenletConfig.ETCDConfig,
		b.SecretsManager,
		v1beta1constants.SecretNameCACluster,
		v1beta1constants.PriorityClassNameSeedSystem800,
	)
	if err != nil {
		return fmt.Errorf("failed creating etcd-druid deployer: %w", err)
	}

	return deployer.Deploy(ctx)
}

// ReconcileBackupBucket reconciles the core.gardener.cloud/v1beta1.BackupBucket resource for the shoot cluster.
func (b *AutonomousBotanist) ReconcileBackupBucket(ctx context.Context) error {
	backupBucket, err := b.reconcileCoreBackupBucketResource(ctx)
	if err != nil {
		return fmt.Errorf("failed reconciling core.gardener.cloud/v1beta1.BackupBucket resource: %w", err)
	}

	reconciler := &backupbucketcontroller.Reconciler{
		GardenClient:    b.GardenClient,
		SeedClient:      b.SeedClientSet.Client(),
		Clock:           b.Clock,
		Recorder:        &record.FakeRecorder{},
		GardenNamespace: b.Shoot.ControlPlaneNamespace,
	}

	return runReconcilerUntilCondition(ctx, b.Logger, backupbucketcontroller.ControllerName, reconciler, backupBucket, func(ctx context.Context) error {
		extensionsBackupBucket := &extensionsv1alpha1.BackupBucket{ObjectMeta: metav1.ObjectMeta{Name: backupBucket.Name}}
		if err := b.SeedClientSet.Client().Get(ctx, client.ObjectKeyFromObject(extensionsBackupBucket), extensionsBackupBucket); err != nil {
			return fmt.Errorf("failed getting extensions.gardener.cloud/v1beta1.BackupBucket resource: %w", err)
		}
		return health.CheckExtensionObject(extensionsBackupBucket)
	})
}

func (b *AutonomousBotanist) reconcileCoreBackupBucketResource(ctx context.Context) (*gardencorev1beta1.BackupBucket, error) {
	component := corebackupbucket.New(b.Logger, b.GardenClient, &corebackupbucket.Values{
		Name:          string(b.Shoot.GetInfo().Status.UID),
		Config:        v1beta1helper.GetBackupConfigForShoot(b.Shoot.GetInfo(), nil),
		DefaultRegion: b.Shoot.GetInfo().Spec.Region,
		Clock:         b.Clock,
	}, corebackupbucket.DefaultInterval, corebackupbucket.DefaultTimeout)

	if err := component.Deploy(ctx); err != nil {
		return nil, fmt.Errorf("failed reconciling core.gardener.cloud/v1beta1.BackupBucket resource: %w", err)
	}

	return component.Get(ctx)
}

// Some reconcilers do not wait for some conditions to be met. Instead, they stop their reconciliation flow and watch
// for these conditions. Since we cannot use watches with fake clients, we have to simulate this behavior by running
// the reconciler until the condition is met.
func runReconcilerUntilCondition(ctx context.Context, logger logr.Logger, controllerName string, reconciler reconcile.Reconciler, obj client.Object, condition func(context.Context) error) error {
	log := logger.WithName(controllerName+"-reconciler").WithValues("object", client.ObjectKeyFromObject(obj))

	timeoutCtx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	return retry.Until(timeoutCtx, time.Second, func(ctx context.Context) (bool, error) {
		if _, err := reconciler.Reconcile(logf.IntoContext(ctx, log), reconcile.Request{NamespacedName: types.NamespacedName{Name: obj.GetName(), Namespace: obj.GetNamespace()}}); err != nil {
			return retry.MinorError(fmt.Errorf("failed running %s controller for %q: %w", controllerName, client.ObjectKeyFromObject(obj), err))
		}

		if err := condition(ctx); err != nil {
			return retry.MinorError(fmt.Errorf("condition not yet met: %w", err))
		}

		return retry.Ok()
	})
}
