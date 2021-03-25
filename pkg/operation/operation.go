// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package operation

import (
	"context"
	"fmt"
	"strings"

	"github.com/gardener/gardener/charts"
	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	gardencorev1alpha1helper "github.com/gardener/gardener/pkg/apis/core/v1alpha1/helper"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/operation/etcdencryption"
	"github.com/gardener/gardener/pkg/operation/garden"
	"github.com/gardener/gardener/pkg/operation/seed"
	"github.com/gardener/gardener/pkg/operation/shoot"
	"github.com/gardener/gardener/pkg/utils/chart"
	"github.com/gardener/gardener/pkg/utils/flow"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// NewBuilder returns a new Builder.
func NewBuilder() *Builder {
	return &Builder{
		configFunc: func() (*config.GardenletConfiguration, error) {
			return nil, fmt.Errorf("config is required but not set")
		},
		gardenFunc: func(map[string]*corev1.Secret) (*garden.Garden, error) {
			return nil, fmt.Errorf("garden object is required but not set")
		},
		gardenerInfoFunc: func() (*gardencorev1beta1.Gardener, error) {
			return nil, fmt.Errorf("gardener info is required but not set")
		},
		gardenClusterIdentityFunc: func() (string, error) {
			return "", fmt.Errorf("garden cluster identity is required but not set")
		},
		imageVectorFunc: func() (imagevector.ImageVector, error) {
			return nil, fmt.Errorf("image vector is required but not set")
		},
		loggerFunc: func() (*logrus.Entry, error) {
			return nil, fmt.Errorf("logger is required but not set")
		},
		secretsFunc: func() (map[string]*corev1.Secret, error) {
			return nil, fmt.Errorf("secrets map is required but not set")
		},
		seedFunc: func(context.Context) (*seed.Seed, error) {
			return nil, fmt.Errorf("seed object is required but not set")
		},
		shootFunc: func(context.Context, client.Client, *garden.Garden, *seed.Seed) (*shoot.Shoot, error) {
			return nil, fmt.Errorf("shoot object is required but not set")
		},
		chartsRootPathFunc: func() string {
			return charts.Path
		},
	}
}

// WithConfig sets the configFunc attribute at the Builder.
func (b *Builder) WithConfig(cfg *config.GardenletConfiguration) *Builder {
	b.configFunc = func() (*config.GardenletConfiguration, error) { return cfg, nil }
	return b
}

// WithGarden sets the gardenFunc attribute at the Builder.
func (b *Builder) WithGarden(g *garden.Garden) *Builder {
	b.gardenFunc = func(_ map[string]*corev1.Secret) (*garden.Garden, error) { return g, nil }
	return b
}

// WithGardenFrom sets the gardenFunc attribute at the Builder which will build a new Garden object.
func (b *Builder) WithGardenFrom(k8sGardenCoreInformers gardencoreinformers.Interface, namespace string) *Builder {
	b.gardenFunc = func(secrets map[string]*corev1.Secret) (*garden.Garden, error) {
		return garden.
			NewBuilder().
			WithProjectFromLister(k8sGardenCoreInformers.Projects().Lister(), namespace).
			WithInternalDomainFromSecrets(secrets).
			WithDefaultDomainsFromSecrets(secrets).
			Build()
	}
	return b
}

// WithGardenerInfo sets the gardenerInfoFunc attribute at the Builder.
func (b *Builder) WithGardenerInfo(gardenerInfo *gardencorev1beta1.Gardener) *Builder {
	b.gardenerInfoFunc = func() (*gardencorev1beta1.Gardener, error) { return gardenerInfo, nil }
	return b
}

// WithGardenClusterIdentity sets the identity of the Garden cluster as attribute at the Builder.
func (b *Builder) WithGardenClusterIdentity(gardenClusterIdentity string) *Builder {
	b.gardenClusterIdentityFunc = func() (string, error) { return gardenClusterIdentity, nil }
	return b
}

// WithImageVector sets the imageVectorFunc attribute at the Builder.
func (b *Builder) WithImageVector(imageVector imagevector.ImageVector) *Builder {
	b.imageVectorFunc = func() (imagevector.ImageVector, error) { return imageVector, nil }
	return b
}

// WithLogger sets the loggerFunc attribute at the Builder.
func (b *Builder) WithLogger(logger *logrus.Entry) *Builder {
	b.loggerFunc = func() (*logrus.Entry, error) { return logger, nil }
	return b
}

// WithSecrets sets the secretsFunc attribute at the Builder.
func (b *Builder) WithSecrets(secrets map[string]*corev1.Secret) *Builder {
	b.secretsFunc = func() (map[string]*corev1.Secret, error) { return secrets, nil }
	return b
}

// WithSeed sets the seedFunc attribute at the Builder.
func (b *Builder) WithSeed(s *seed.Seed) *Builder {
	b.seedFunc = func(_ context.Context) (*seed.Seed, error) { return s, nil }
	return b
}

// WithSeedFrom sets the seedFunc attribute at the Builder which will build a new Seed object.
func (b *Builder) WithSeedFrom(k8sGardenCoreInformers gardencoreinformers.Interface, seedName string) *Builder {
	b.seedFunc = func(ctx context.Context) (*seed.Seed, error) {
		return seed.
			NewBuilder().
			WithSeedObjectFromLister(k8sGardenCoreInformers.Seeds().Lister(), seedName).
			Build()
	}
	return b
}

// WithShoot sets the shootFunc attribute at the Builder.
func (b *Builder) WithShoot(s *shoot.Shoot) *Builder {
	b.shootFunc = func(_ context.Context, _ client.Client, _ *garden.Garden, _ *seed.Seed) (*shoot.Shoot, error) {
		return s, nil
	}
	return b
}

// WithChartsRootPath sets the ChartsRootPath attribute at the Builder.
// Mainly used for testing. Optional.
func (b *Builder) WithChartsRootPath(chartsRootPath string) *Builder {
	b.chartsRootPathFunc = func() string { return chartsRootPath }
	return b
}

// WithShootFrom sets the shootFunc attribute at the Builder which will build a new Shoot object.
func (b *Builder) WithShootFrom(k8sGardenCoreInformers gardencoreinformers.Interface, gardenClient kubernetes.Interface, s *gardencorev1beta1.Shoot) *Builder {
	b.shootFunc = func(ctx context.Context, c client.Client, gardenObj *garden.Garden, seedObj *seed.Seed) (*shoot.Shoot, error) {
		return shoot.
			NewBuilder().
			WithShootObject(s).
			WithCloudProfileObjectFromReader(gardenClient.APIReader()).
			WithShootSecretFromReader(gardenClient.APIReader()).
			WithProjectName(gardenObj.Project.Name).
			WithDisableDNS(!seedObj.Info.Spec.Settings.ShootDNS.Enabled).
			WithInternalDomain(gardenObj.InternalDomain).
			WithDefaultDomains(gardenObj.DefaultDomains).
			Build(ctx, c)
	}
	return b
}

// WithShootFromCluster sets the shootFunc attribute at the Builder which will build a new Shoot object constructed from the cluster resource.
// The shoot status is still taken from the passed `shoot`, though.
func (b *Builder) WithShootFromCluster(gardenClient, seedClient kubernetes.Interface, s *gardencorev1beta1.Shoot) *Builder {
	b.shootFunc = func(ctx context.Context, c client.Client, gardenObj *garden.Garden, seedObj *seed.Seed) (*shoot.Shoot, error) {
		shootNamespace := shoot.ComputeTechnicalID(gardenObj.Project.Name, s)

		shoot, err := shoot.
			NewBuilder().
			WithShootObjectFromCluster(seedClient, shootNamespace).
			WithCloudProfileObjectFromCluster(seedClient, shootNamespace).
			WithShootSecretFromReader(gardenClient.APIReader()).
			WithProjectName(gardenObj.Project.Name).
			WithDisableDNS(!seedObj.Info.Spec.Settings.ShootDNS.Enabled).
			WithInternalDomain(gardenObj.InternalDomain).
			WithDefaultDomains(gardenObj.DefaultDomains).
			Build(ctx, c)
		if err != nil {
			return nil, err
		}
		shoot.Info.Status = s.Status
		return shoot, nil
	}
	return b
}

// Build initializes a new Operation object.
func (b *Builder) Build(ctx context.Context, clientMap clientmap.ClientMap) (*Operation, error) {
	operation := &Operation{
		ClientMap: clientMap,
		CheckSums: make(map[string]string),
	}

	gardenClient, err := clientMap.GetClient(ctx, keys.ForGarden())
	if err != nil {
		return nil, fmt.Errorf("failed to get garden client: %w", err)
	}
	operation.K8sGardenClient = gardenClient

	config, err := b.configFunc()
	if err != nil {
		return nil, err
	}
	operation.Config = config

	secretsMap, err := b.secretsFunc()
	if err != nil {
		return nil, err
	}
	secrets := make(map[string]*corev1.Secret)
	for k, v := range secretsMap {
		secrets[k] = v
	}
	operation.Secrets = secrets

	garden, err := b.gardenFunc(secrets)
	if err != nil {
		return nil, err
	}
	operation.Garden = garden

	gardenerInfo, err := b.gardenerInfoFunc()
	if err != nil {
		return nil, err
	}
	operation.GardenerInfo = gardenerInfo

	gardenClusterIdentity, err := b.gardenClusterIdentityFunc()
	if err != nil {
		return nil, err
	}
	operation.GardenClusterIdentity = gardenClusterIdentity

	imageVector, err := b.imageVectorFunc()
	if err != nil {
		return nil, err
	}
	operation.ImageVector = imageVector

	logger, err := b.loggerFunc()
	if err != nil {
		return nil, err
	}
	operation.Logger = logger

	seed, err := b.seedFunc(ctx)
	if err != nil {
		return nil, err
	}
	operation.Seed = seed

	shoot, err := b.shootFunc(ctx, gardenClient.Client(), garden, seed)
	if err != nil {
		return nil, err
	}
	operation.Shoot = shoot

	// Get the ManagedSeed object for this shoot, if it exists.
	// Also read the managed seed API server settings from the managed-seed-api-server annotation.
	operation.ManagedSeed, err = kutil.GetManagedSeed(ctx, gardenClient.GardenSeedManagement(), shoot.Info.Namespace, shoot.Info.Name)
	if err != nil {
		return nil, fmt.Errorf("could not get managed seed for shoot %s/%s: %w", shoot.Info.Namespace, shoot.Info.Name, err)
	}
	operation.ManagedSeedAPIServer, err = gardencorev1beta1helper.ReadManagedSeedAPIServer(shoot.Info)
	if err != nil {
		return nil, fmt.Errorf("could not read managed seed API server settings of shoot %s/%s: %+v", shoot.Info.Namespace, shoot.Info.Name, err)
	}

	// If the managed-seed-api-server annotation is not present, try to read the managed seed API server settings
	// from the use-as-seed annotation. This is done to avoid re-annotating a shoot annotated with the use-as-seed annotation
	// by the shooted seed registration controller.
	if operation.ManagedSeedAPIServer == nil {
		shootedSeed, err := gardencorev1beta1helper.ReadShootedSeed(shoot.Info)
		if err != nil {
			return nil, fmt.Errorf("could not read managed seed API server settings of shoot %s/%s: %+v", shoot.Info.Namespace, shoot.Info.Name, err)
		}
		if shootedSeed != nil {
			operation.ManagedSeedAPIServer = shootedSeed.APIServer
		}
	}

	operation.ChartsRootPath = b.chartsRootPathFunc()

	return operation, nil
}

// InitializeSeedClients will use the Garden Kubernetes client to read the Seed Secret in the Garden
// cluster which contains a Kubeconfig that can be used to authenticate against the Seed cluster. With it,
// a Kubernetes client as well as a Chart renderer for the Seed cluster will be initialized and attached to
// the already existing Operation object.
func (o *Operation) InitializeSeedClients(ctx context.Context) error {
	if o.K8sSeedClient != nil {
		return nil
	}

	seedClient, err := o.ClientMap.GetClient(ctx, keys.ForSeed(o.Seed.Info))
	if err != nil {
		return fmt.Errorf("failed to get seed client: %w", err)
	}
	o.K8sSeedClient = seedClient
	return nil
}

// InitializeShootClients will use the Seed Kubernetes client to read the gardener Secret in the Seed
// cluster which contains a Kubeconfig that can be used to authenticate against the Shoot cluster. With it,
// a Kubernetes client as well as a Chart renderer for the Shoot cluster will be initialized and attached to
// the already existing Operation object.
func (o *Operation) InitializeShootClients(ctx context.Context) error {
	if o.K8sShootClient != nil {
		return nil
	}

	if o.Shoot.HibernationEnabled {
		// Don't initialize clients for Shoots, that are currently hibernated and their API server is not running
		apiServerRunning, err := o.IsAPIServerRunning(ctx)
		if err != nil {
			return err
		}
		if !apiServerRunning {
			return nil
		}
	}

	shootClient, err := o.ClientMap.GetClient(ctx, keys.ForShoot(o.Shoot.Info))
	if err != nil {
		return err
	}
	o.K8sShootClient = shootClient

	return nil
}

// IsAPIServerRunning checks if the API server of the Shoot currently running (not scaled-down/deleted).
func (o *Operation) IsAPIServerRunning(ctx context.Context) (bool, error) {
	deployment := &appsv1.Deployment{}
	// use API reader here to make sure, we're not reading from a stale cache, when checking if we should initialize a shoot client (e.g. from within the care controller)
	if err := o.K8sSeedClient.APIReader().Get(ctx, kutil.Key(o.Shoot.SeedNamespace, v1beta1constants.DeploymentNameKubeAPIServer), deployment); err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}

	if deployment.GetDeletionTimestamp() != nil {
		return false, nil
	}

	if deployment.Spec.Replicas == nil {
		return false, nil
	}
	return *deployment.Spec.Replicas > 0, nil
}

// GetSecretKeysOfRole returns a list of keys which are present in the Garden Secrets map and which
// are prefixed with <kind>.
func (o *Operation) GetSecretKeysOfRole(kind string) []string {
	return common.GetSecretKeysWithPrefix(kind, o.Secrets)
}

func makeDescription(stats *flow.Stats) string {
	if stats.ProgressPercent() == 0 {
		return "Starting " + stats.FlowName
	}
	if stats.ProgressPercent() == 100 {
		return stats.FlowName + " finished"
	}
	return strings.Join(stats.Running.StringList(), ", ")
}

// ReportShootProgress will update the last operation object in the Shoot manifest `status` section
// by the current progress of the Flow execution.
func (o *Operation) ReportShootProgress(ctx context.Context, stats *flow.Stats) {
	var (
		description    = makeDescription(stats)
		progress       = stats.ProgressPercent()
		lastUpdateTime = metav1.Now()
	)

	newShoot, err := kutil.TryUpdateShootStatus(ctx, o.K8sGardenClient.GardenCore(), retry.DefaultRetry, o.Shoot.Info.ObjectMeta,
		func(shoot *gardencorev1beta1.Shoot) (*gardencorev1beta1.Shoot, error) {
			if shoot.Status.LastOperation == nil {
				return nil, fmt.Errorf("last operation of Shoot %s/%s is unset", shoot.Namespace, shoot.Name)
			}
			if shoot.Status.LastOperation.LastUpdateTime.After(lastUpdateTime.Time) {
				return nil, fmt.Errorf("last operation of Shoot %s/%s was updated mid-air", shoot.Namespace, shoot.Name)
			}
			if description != "" {
				shoot.Status.LastOperation.Description = description
			}
			shoot.Status.LastOperation.Progress = progress
			shoot.Status.LastOperation.LastUpdateTime = lastUpdateTime
			return shoot, nil
		})
	if err != nil {
		o.Logger.Errorf("Could not report shoot progress: %v", err)
		return
	}

	o.Shoot.Info = newShoot
}

// CleanShootTaskErrorAndUpdateStatusLabel removes the error with taskID from the Shoot's status.LastErrors array.
// If the status.LastErrors array is empty then status.LastErrors is also removed. It also re-evaluates the shoot status
// in case the last error list is empty now, and if necessary, updates the status label on the shoot.
func (o *Operation) CleanShootTaskErrorAndUpdateStatusLabel(ctx context.Context, taskID string) {
	updatedShoot, err := kutil.TryUpdateShootStatus(ctx, o.K8sGardenClient.GardenCore(), retry.DefaultRetry, o.Shoot.Info.ObjectMeta,
		func(shoot *gardencorev1beta1.Shoot) (*gardencorev1beta1.Shoot, error) {
			shoot.Status.LastErrors = gardencorev1beta1helper.DeleteLastErrorByTaskID(o.Shoot.Info.Status.LastErrors, taskID)
			return shoot, nil
		},
	)
	if err != nil {
		o.Logger.Errorf("Could not update shoot's %s/%s last errors: %v", o.Shoot.Info.Namespace, o.Shoot.Info.Name, err)
		return
	}
	o.Shoot.Info = updatedShoot

	if len(o.Shoot.Info.Status.LastErrors) == 0 {
		oldObj := o.Shoot.Info.DeepCopy()
		kutil.SetMetaDataLabel(&o.Shoot.Info.ObjectMeta, common.ShootStatus, string(shoot.ComputeStatus(
			o.Shoot.Info.Status.LastOperation,
			o.Shoot.Info.Status.LastErrors,
			o.Shoot.Info.Status.Conditions...,
		)))
		if err := o.K8sGardenClient.Client().Patch(ctx, o.Shoot.Info, client.MergeFrom(oldObj)); err != nil {
			o.Logger.Errorf("Could not update shoot's %s/%s status label after removing an erroneous task: %v", o.Shoot.Info.Namespace, o.Shoot.Info.Name, err)
			return
		}
	}
}

// SeedVersion is a shorthand for the kubernetes version of the K8sSeedClient.
func (o *Operation) SeedVersion() string {
	return o.K8sSeedClient.Version()
}

// ShootVersion is a shorthand for the desired kubernetes version of the operation's shoot.
func (o *Operation) ShootVersion() string {
	return o.Shoot.Info.Spec.Kubernetes.Version
}

// InjectSeedSeedImages injects images that shall run on the Seed and target the Seed's Kubernetes version.
func (o *Operation) InjectSeedSeedImages(values map[string]interface{}, names ...string) (map[string]interface{}, error) {
	return chart.InjectImages(values, o.ImageVector, names, imagevector.RuntimeVersion(o.SeedVersion()), imagevector.TargetVersion(o.SeedVersion()))
}

// InjectSeedShootImages injects images that shall run on the Seed but target the Shoot's Kubernetes version.
func (o *Operation) InjectSeedShootImages(values map[string]interface{}, names ...string) (map[string]interface{}, error) {
	return chart.InjectImages(values, o.ImageVector, names, imagevector.RuntimeVersion(o.SeedVersion()), imagevector.TargetVersion(o.ShootVersion()))
}

// InjectShootShootImages injects images that shall run on the Shoot and target the Shoot's Kubernetes version.
func (o *Operation) InjectShootShootImages(values map[string]interface{}, names ...string) (map[string]interface{}, error) {
	return chart.InjectImages(values, o.ImageVector, names, imagevector.RuntimeVersion(o.ShootVersion()), imagevector.TargetVersion(o.ShootVersion()))
}

// EnsureShootStateExists creates the ShootState resource for the corresponding shoot and sets its ownerReferences to the Shoot.
func (o *Operation) EnsureShootStateExists(ctx context.Context) error {
	shootState := &gardencorev1alpha1.ShootState{
		ObjectMeta: metav1.ObjectMeta{
			Name:      o.Shoot.Info.Name,
			Namespace: o.Shoot.Info.Namespace,
		},
	}
	ownerReference := metav1.NewControllerRef(o.Shoot.Info, gardencorev1beta1.SchemeGroupVersion.WithKind("Shoot"))
	blockOwnerDeletion := false
	ownerReference.BlockOwnerDeletion = &blockOwnerDeletion

	_, err := controllerutil.CreateOrUpdate(ctx, o.K8sGardenClient.DirectClient(), shootState, func() error {
		shootState.OwnerReferences = []metav1.OwnerReference{*ownerReference}
		return nil
	})
	if err != nil {
		return err
	}

	o.ShootState = shootState
	gardenerResourceList := gardencorev1alpha1helper.GardenerResourceDataList(shootState.Spec.Gardener)
	o.Shoot.ETCDEncryption, err = etcdencryption.GetEncryptionConfig(gardenerResourceList)
	return err
}

// SaveGardenerResourcesInShootState saves the provided GardenerResourcesDataList in the ShootState's `gardener` field
func (o *Operation) SaveGardenerResourcesInShootState(ctx context.Context, resourceList gardencorev1alpha1helper.GardenerResourceDataList) error {
	shootState := o.ShootState.DeepCopy()
	shootState.Spec.Gardener = resourceList
	if err := o.K8sGardenClient.Client().Patch(ctx, shootState, client.MergeFrom(o.ShootState)); err != nil {
		return err
	}
	o.ShootState = shootState
	return nil
}

// DeleteClusterResourceFromSeed deletes the `Cluster` extension resource for the shoot in the seed cluster.
func (o *Operation) DeleteClusterResourceFromSeed(ctx context.Context) error {
	if err := o.InitializeSeedClients(ctx); err != nil {
		o.Logger.Errorf("Could not initialize a new Kubernetes client for the seed cluster: %s", err.Error())
		return err
	}

	return client.IgnoreNotFound(o.K8sSeedClient.Client().Delete(ctx, &extensionsv1alpha1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: o.Shoot.SeedNamespace}}))
}

// SwitchBackupEntryToTargetSeed changes the BackupEntry in the Garden cluster to the Target Seed and removes it from the Source Seed
func (o *Operation) SwitchBackupEntryToTargetSeed(ctx context.Context) error {
	var (
		name              = common.GenerateBackupEntryName(o.Shoot.Info.Status.TechnicalID, o.Shoot.Info.Status.UID)
		gardenBackupEntry = &gardencorev1beta1.BackupEntry{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: o.Shoot.Info.Namespace,
			},
		}
	)

	return kutil.TryUpdate(ctx, retry.DefaultBackoff, o.K8sGardenClient.DirectClient(), gardenBackupEntry, func() error {
		gardenBackupEntry.Spec.SeedName = o.Shoot.Info.Spec.SeedName
		return nil
	})
}

// ComputeGrafanaHosts computes the host for both grafanas.
func (o *Operation) ComputeGrafanaHosts() []string {
	return []string{
		o.ComputeGrafanaOperatorsHost(),
		o.ComputeGrafanaUsersHost(),
	}
}

// ComputePrometheusHosts computes the hosts for prometheus.
func (o *Operation) ComputePrometheusHosts() []string {
	return []string{
		o.ComputePrometheusHost(),
	}
}

// ComputeAlertManagerHosts computes the host for alert manager.
func (o *Operation) ComputeAlertManagerHosts() []string {
	return []string{
		o.ComputeAlertManagerHost(),
	}
}

// ComputeGrafanaOperatorsHost computes the host for users Grafana.
func (o *Operation) ComputeGrafanaOperatorsHost() string {
	return o.ComputeIngressHost(common.GrafanaOperatorsPrefix)
}

// ComputeGrafanaUsersHost computes the host for operators Grafana.
func (o *Operation) ComputeGrafanaUsersHost() string {
	return o.ComputeIngressHost(common.GrafanaUsersPrefix)
}

// ComputeAlertManagerHost computes the host for alert manager.
func (o *Operation) ComputeAlertManagerHost() string {
	return o.ComputeIngressHost(common.AlertManagerPrefix)
}

// ComputePrometheusHost computes the host for prometheus.
func (o *Operation) ComputePrometheusHost() string {
	return o.ComputeIngressHost(common.PrometheusPrefix)
}

// ComputeIngressHost computes the host for a given prefix.
func (o *Operation) ComputeIngressHost(prefix string) string {
	shortID := strings.Replace(o.Shoot.Info.Status.TechnicalID, shoot.TechnicalIDPrefix, "", 1)
	return fmt.Sprintf("%s-%s.%s", prefix, shortID, o.Seed.IngressDomain())
}
