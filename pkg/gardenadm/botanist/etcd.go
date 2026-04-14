// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/go-logr/logr"
	"github.com/spf13/afero"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/events"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1beta1helper "github.com/gardener/gardener/pkg/api/core/v1beta1/helper"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/apis/config/gardenlet/v1alpha1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	bootstrapetcd "github.com/gardener/gardener/pkg/component/etcd/bootstrap"
	"github.com/gardener/gardener/pkg/component/etcd/etcd"
	etcdconstants "github.com/gardener/gardener/pkg/component/etcd/etcd/constants"
	corebackupbucket "github.com/gardener/gardener/pkg/component/garden/backupbucket"
	sharedcomponent "github.com/gardener/gardener/pkg/component/shared"
	"github.com/gardener/gardener/pkg/gardenadm/staticpod"
	backupbucketcontroller "github.com/gardener/gardener/pkg/gardenlet/controller/backupbucket"
	backupentrycontroller "github.com/gardener/gardener/pkg/gardenlet/controller/backupentry"
	"github.com/gardener/gardener/pkg/utils"
	imagevectorutils "github.com/gardener/gardener/pkg/utils/imagevector"
	"github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
	"github.com/gardener/gardener/pkg/utils/retry"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

// DeployEtcdDruid deploys the etcd-druid component.
func (b *GardenadmBotanist) DeployEtcdDruid(ctx context.Context) error {
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
	gardenletConfig.ETCDConfig.FeatureGates = map[string]bool{"UpgradeEtcdVersion": true}

	deployer, err := sharedcomponent.NewEtcdDruid(
		b.SeedClientSet.Client(),
		v1beta1constants.GardenNamespace,
		b.Shoot.KubernetesVersion,
		componentImageVectors,
		gardenletConfig.ETCDConfig,
		b.SecretsManager,
		v1beta1constants.SecretNameCACluster,
		v1beta1constants.PriorityClassNameSeedSystem800,
		false,
	)
	if err != nil {
		return fmt.Errorf("failed creating etcd-druid deployer: %w", err)
	}

	return deployer.Deploy(ctx)
}

// ReconcileBackupBucket reconciles the core.gardener.cloud/v1beta1.BackupBucket resource for the shoot cluster.
func (b *GardenadmBotanist) ReconcileBackupBucket(ctx context.Context) error {
	backupBucket, err := b.reconcileCoreBackupBucketResource(ctx)
	if err != nil {
		return fmt.Errorf("failed reconciling core.gardener.cloud/v1beta1.BackupBucket resource: %w", err)
	}

	reconciler := &backupbucketcontroller.Reconciler{
		GardenClient:    b.GardenClient,
		SeedClient:      b.SeedClientSet.Client(),
		Clock:           b.Clock,
		Recorder:        &events.FakeRecorder{},
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

func (b *GardenadmBotanist) reconcileCoreBackupBucketResource(ctx context.Context) (*gardencorev1beta1.BackupBucket, error) {
	component := corebackupbucket.New(b.Logger, b.GardenClient, &corebackupbucket.Values{
		Name:          string(b.Shoot.GetInfo().Status.UID),
		Config:        v1beta1helper.GetBackupConfigForShoot(b.Shoot.GetInfo(), nil),
		DefaultRegion: b.Shoot.GetInfo().Spec.Region,
		Clock:         b.Clock,
		Shoot:         b.Shoot.GetInfo(),
	}, corebackupbucket.DefaultInterval, corebackupbucket.DefaultTimeout)

	if err := component.Deploy(ctx); err != nil {
		return nil, fmt.Errorf("failed reconciling core.gardener.cloud/v1beta1.BackupBucket resource: %w", err)
	}

	return component.Get(ctx)
}

// ReconcileBackupEntry reconciles the core.gardener.cloud/v1beta1.BackupEntry resource for the shoot cluster.
func (b *GardenadmBotanist) ReconcileBackupEntry(ctx context.Context) error {
	backupEntry, err := b.reconcileCoreBackupEntryResource(ctx)
	if err != nil {
		return fmt.Errorf("failed reconciling core.gardener.cloud/v1beta1.BackupEntry resource: %w", err)
	}

	reconciler := &backupentrycontroller.Reconciler{
		GardenClient:    b.GardenClient,
		SeedClient:      b.SeedClientSet.Client(),
		Clock:           b.Clock,
		Recorder:        &events.FakeRecorder{},
		GardenNamespace: b.Shoot.ControlPlaneNamespace,
	}

	return runReconcilerUntilCondition(ctx, b.Logger, backupentrycontroller.ControllerName, reconciler, backupEntry, func(ctx context.Context) error {
		extensionsBackupEntry := &extensionsv1alpha1.BackupEntry{ObjectMeta: metav1.ObjectMeta{Name: backupEntry.Name}}
		if err := b.SeedClientSet.Client().Get(ctx, client.ObjectKeyFromObject(extensionsBackupEntry), extensionsBackupEntry); err != nil {
			return fmt.Errorf("failed getting extensions.gardener.cloud/v1beta1.BackupEntry resource: %w", err)
		}
		return health.CheckExtensionObject(extensionsBackupEntry)
	})
}

func (b *GardenadmBotanist) reconcileCoreBackupEntryResource(ctx context.Context) (*gardencorev1beta1.BackupEntry, error) {
	if err := b.Shoot.Components.BackupEntry.Deploy(ctx); err != nil {
		return nil, fmt.Errorf("failed reconciling core.gardener.cloud/v1beta1.BackupEntry resource: %w", err)
	}

	return b.Shoot.Components.BackupEntry.Get(ctx)
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

// WaitUntilEtcdsReconciled waits until the druid.gardener.cloud/v1alpha1.Etcd resources have been reconciled by
// etcd-druid.
func (b *GardenadmBotanist) WaitUntilEtcdsReconciled(ctx context.Context) error {
	if err := b.WaitUntilEtcdsReady(ctx); err != nil {
		return fmt.Errorf("failed waiting for etcd to become ready: %w", err)
	}

	b.useEtcdManagedByDruid = true
	return nil
}

// FinalizeEtcdBootstrapTransition cleans up no longer needed directories for the bootstrap etcds. Those are not deleted
// automatically.
func (b *GardenadmBotanist) FinalizeEtcdBootstrapTransition(_ context.Context) error {
	for _, dir := range []string{
		filepath.Join(string(filepath.Separator), "var", "lib", bootstrapetcd.Name(v1beta1constants.ETCDRoleMain)),
		filepath.Join(string(filepath.Separator), "var", "lib", bootstrapetcd.Name(v1beta1constants.ETCDRoleEvents)),
	} {
		if err := b.FS.RemoveAll(dir); err != nil {
			return fmt.Errorf("failed cleaning up %s directory: %w", dir, err)
		}
	}

	return nil
}

// ETCDAssets contains node-specific assets for the static ETCD pods.
type ETCDAssets struct {
	ServerSecret, PeerSecret *corev1.Secret
	Config                   *corev1.ConfigMap
}

// ETCDRoleToAssets is a map whose keys are ETCD roles and whose values are the specific assets.
type ETCDRoleToAssets map[string]*ETCDAssets

// FetchSecrets fetches the etcd secrets and assigns them to the assets. Peer secrets are only
// fetched when highAvailabilityEnabled is true since they are only generated for multi-member etcd clusters.
func (e ETCDRoleToAssets) FetchSecrets(ctx context.Context, c client.Client, namespace, hostName string, highAvailabilityEnabled bool) error {
	for role := range e {
		findNewestObject := func(secretType string) (client.Object, error) {
			return kubernetes.NewestObject(ctx, c, &corev1.SecretList{}, nil, client.InNamespace(namespace), client.MatchingLabels{
				secretsmanager.LabelKeyManagedBy: secretsmanager.LabelValueSecretsManager,
				etcdconstants.LabelKeySecretType: secretType,
				etcdconstants.LabelKeyRole:       role,
				etcdconstants.LabelKeyHostName:   hostName,
			})
		}

		serverSecret, err := findNewestObject(etcdconstants.LabelValueSecretTypeServer)
		if err != nil {
			return fmt.Errorf("failed to find server secret for role %q: %w", role, err)
		}
		e[role].ServerSecret = serverSecret.(*corev1.Secret)

		if highAvailabilityEnabled {
			peerSecret, err := findNewestObject(etcdconstants.LabelValueSecretTypePeer)
			if err != nil {
				return fmt.Errorf("failed to find peer secret for role %q: %w", role, err)
			}
			e[role].PeerSecret = peerSecret.(*corev1.Secret)
		}
	}

	return nil
}

// FetchConfigMaps fetches the etcd ConfigMaps and assigns them to the assets.
func (e ETCDRoleToAssets) FetchConfigMaps(ctx context.Context, c client.Client, namespace string) error {
	for role := range e {
		configMap := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: etcd.ConfigMapName(role), Namespace: namespace}}
		if err := c.Get(ctx, client.ObjectKeyFromObject(configMap), configMap); err != nil {
			return fmt.Errorf("failed to fetch the configuration ConfigMap %s: %w", client.ObjectKeyFromObject(configMap), err)
		}
		e[role].Config = configMap
	}

	return nil
}

func (e ETCDRoleToAssets) loop(fnVolume func(dir string) error, fnAssetData func(path, key string, value []byte) error) error {
	for role, assets := range e {
		volumes := map[string]map[string][]byte{
			etcdconstants.VolumeNameServerTLS:     assets.ServerSecret.Data,
			etcdconstants.VolumeNameConfiguration: toBinaryData(assets.Config.Data),
		}
		if assets.PeerSecret != nil {
			volumes[etcdconstants.VolumeNamePeerTLS] = assets.PeerSecret.Data
		}

		for volumeName, data := range volumes {
			dir := staticpod.HostPath(etcd.Name(role), volumeName)

			if fnVolume != nil {
				if err := fnVolume(dir); err != nil {
					return err
				}
			}

			for key, value := range data {
				path := filepath.Join(dir, key)

				if fnAssetData != nil {
					if err := fnAssetData(path, key, value); err != nil {
						return err
					}
				}
			}
		}
	}

	return nil
}

func toBinaryData(data map[string]string) map[string][]byte {
	out := make(map[string][]byte, len(data))
	for k, v := range data {
		out[k] = []byte(v)
	}
	return out
}

func (e ETCDRoleToAssets) allFiles(hostName string) []extensionsv1alpha1.File {
	out := make([]extensionsv1alpha1.File, 0, 3*len(e))

	_ = e.loop(nil, func(path, _ string, value []byte) error {
		out = append(out, extensionsv1alpha1.File{
			Path:        path,
			Permissions: ptr.To[uint32](0600),
			Content:     extensionsv1alpha1.FileContent{Inline: &extensionsv1alpha1.FileContentInline{Encoding: "b64", Data: utils.EncodeBase64(value)}},
			HostName:    &hostName,
		})
		return nil
	})

	return out
}

// WriteToDisk writes all node-specific ETCD assets to the disk.
func (e ETCDRoleToAssets) WriteToDisk(fs afero.Afero) error {
	return e.loop(
		func(dir string) error { return fs.MkdirAll(dir, os.ModeDir) },
		func(path, _ string, value []byte) error { return fs.WriteFile(path, value, 0640) },
	)
}

// HostNameToETCDAssets maps a control plane nodes to their ETCD assets. The keys are the hostnames, the values are the
// ETCD assets.
type HostNameToETCDAssets map[string]ETCDRoleToAssets

// AppendToFiles appends the node-specific ETCD assets to the provided files.
func (h HostNameToETCDAssets) AppendToFiles(files []extensionsv1alpha1.File) []extensionsv1alpha1.File {
	out := slices.Clone(files)

	for hostName, etcdRoleToAssets := range h {
		out = append(out, etcdRoleToAssets.allFiles(hostName)...)
	}

	return out
}
