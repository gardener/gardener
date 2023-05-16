// Copyright 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"regexp"
	"strings"

	"github.com/Masterminds/semver"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config/helper"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/operation/garden"
	"github.com/gardener/gardener/pkg/operation/seed"
	shootpkg "github.com/gardener/gardener/pkg/operation/shoot"
	"github.com/gardener/gardener/pkg/utils/chart"
	"github.com/gardener/gardener/pkg/utils/flow"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	versionutils "github.com/gardener/gardener/pkg/utils/version"
)

// NewBuilder returns a new Builder.
func NewBuilder() *Builder {
	return &Builder{
		configFunc: func() (*config.GardenletConfiguration, error) {
			return nil, fmt.Errorf("config is required but not set")
		},
		gardenFunc: func(context.Context, map[string]*corev1.Secret) (*garden.Garden, error) {
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
		loggerFunc: func() (logr.Logger, error) {
			return logr.Discard(), fmt.Errorf("logger is required but not set")
		},
		secretsFunc: func() (map[string]*corev1.Secret, error) {
			return nil, fmt.Errorf("secrets map is required but not set")
		},
		seedFunc: func(context.Context) (*seed.Seed, error) {
			return nil, fmt.Errorf("seed object is required but not set")
		},
		shootFunc: func(context.Context, client.Reader, *garden.Garden, *seed.Seed) (*shootpkg.Shoot, error) {
			return nil, fmt.Errorf("shoot object is required but not set")
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
	b.gardenFunc = func(context.Context, map[string]*corev1.Secret) (*garden.Garden, error) { return g, nil }
	return b
}

// WithGardenFrom sets the gardenFunc attribute at the Builder which will build a new Garden object.
func (b *Builder) WithGardenFrom(reader client.Reader, namespace string) *Builder {
	b.gardenFunc = func(ctx context.Context, secrets map[string]*corev1.Secret) (*garden.Garden, error) {
		return garden.
			NewBuilder().
			WithProjectFrom(reader, namespace).
			WithInternalDomainFromSecrets(secrets).
			WithDefaultDomainsFromSecrets(secrets).
			Build(ctx)
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
func (b *Builder) WithLogger(log logr.Logger) *Builder {
	b.loggerFunc = func() (logr.Logger, error) { return log, nil }
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
func (b *Builder) WithSeedFrom(gardenClient client.Reader, seedName string) *Builder {
	b.seedFunc = func(ctx context.Context) (*seed.Seed, error) {
		return seed.
			NewBuilder().
			WithSeedObjectFrom(gardenClient, seedName).
			Build(ctx)
	}
	return b
}

// WithShoot sets the shootFunc attribute at the Builder.
func (b *Builder) WithShoot(s *shootpkg.Shoot) *Builder {
	b.shootFunc = func(_ context.Context, _ client.Reader, _ *garden.Garden, _ *seed.Seed) (*shootpkg.Shoot, error) {
		return s, nil
	}
	return b
}

// WithShootFromCluster sets the shootFunc attribute at the Builder which will build a new Shoot object constructed from the cluster resource.
// The shoot status is still taken from the passed `shoot`, though.
func (b *Builder) WithShootFromCluster(gardenClient client.Client, seedClientSet kubernetes.Interface, s *gardencorev1beta1.Shoot) *Builder {
	b.shootFunc = func(ctx context.Context, c client.Reader, gardenObj *garden.Garden, seedObj *seed.Seed) (*shootpkg.Shoot, error) {
		shootNamespace := shootpkg.ComputeTechnicalID(gardenObj.Project.Name, s)

		shoot, err := shootpkg.
			NewBuilder().
			WithShootObjectFromCluster(seedClientSet, shootNamespace).
			WithCloudProfileObjectFromCluster(seedClientSet, shootNamespace).
			WithShootSecretFrom(gardenClient).
			WithSeedObject(seedObj.GetInfo()).
			WithProjectName(gardenObj.Project.Name).
			WithInternalDomain(gardenObj.InternalDomain).
			WithDefaultDomains(gardenObj.DefaultDomains).
			Build(ctx, c)
		if err != nil {
			return nil, err
		}
		// It's OK to modify the value returned by GetInfo() here because at this point there
		// can be no concurrent reads or writes
		shoot.GetInfo().Status = s.Status
		return shoot, nil
	}
	return b
}

// Build initializes a new Operation object.
func (b *Builder) Build(
	ctx context.Context,
	gardenClient client.Client,
	seedClientSet kubernetes.Interface,
	shootClientMap clientmap.ClientMap,
) (
	*Operation,
	error,
) {
	operation := &Operation{
		GardenClient:   gardenClient,
		SeedClientSet:  seedClientSet,
		ShootClientMap: shootClientMap,
	}

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
	operation.secrets = secrets

	garden, err := b.gardenFunc(ctx, secrets)
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

	seedVersion, err := semver.NewVersion(seedClientSet.Version())
	if err != nil {
		return nil, err
	}
	operation.Seed.KubernetesVersion = seedVersion

	shoot, err := b.shootFunc(ctx, gardenClient, garden, seed)
	if err != nil {
		return nil, err
	}
	operation.Shoot = shoot

	// Get the ManagedSeed object for this shoot, if it exists.
	// Also read the managed seed API server settings from the managed-seed-api-server annotation.
	operation.ManagedSeed, err = kubernetesutils.GetManagedSeedWithReader(ctx, gardenClient, shoot.GetInfo().Namespace, shoot.GetInfo().Name)
	if err != nil {
		return nil, fmt.Errorf("could not get managed seed for shoot %s/%s: %w", shoot.GetInfo().Namespace, shoot.GetInfo().Name, err)
	}
	operation.ManagedSeedAPIServer, err = v1beta1helper.ReadManagedSeedAPIServer(shoot.GetInfo())
	if err != nil {
		return nil, fmt.Errorf("could not read managed seed API server settings of shoot %s/%s: %+v", shoot.GetInfo().Namespace, shoot.GetInfo().Name, err)
	}

	return operation, nil
}

// InitializeShootClients will use the Seed Kubernetes client to read the gardener Secret in the Seed
// cluster which contains a Kubeconfig that can be used to authenticate against the Shoot cluster. With it,
// a Kubernetes client as well as a Chart renderer for the Shoot cluster will be initialized and attached to
// the already existing Operation object.
func (o *Operation) InitializeShootClients(ctx context.Context) error {
	return o.initShootClients(ctx, false)
}

// InitializeDesiredShootClients will use the Seed Kubernetes client to read the gardener Secret in the Seed
// cluster which contains a Kubeconfig that can be used to authenticate against the Shoot cluster. With it,
// a Kubernetes client as well as a Chart renderer for the Shoot cluster will be initialized and attached to
// the already existing Operation object.
// In contrast to InitializeShootClients, InitializeDesiredShootClients returns an error if the discovered version
// via the client does not match the desired Kubernetes version from the shoot spec.
// This is especially useful, if the client is initialized after a rolling update of the Kube-Apiserver
// and you want to ensure that the discovered version matches the expected version.
func (o *Operation) InitializeDesiredShootClients(ctx context.Context) error {
	return o.initShootClients(ctx, true)
}

func (o *Operation) initShootClients(ctx context.Context, versionMatchRequired bool) error {
	if o.ShootClientSet != nil {
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

	shootClient, err := o.ShootClientMap.GetClient(ctx, keys.ForShoot(o.Shoot.GetInfo()))
	if err != nil {
		return err
	}

	if versionMatchRequired {
		var (
			shootClientVersion = shootClient.Version()
			kubeVersion        = o.Shoot.GetInfo().Spec.Kubernetes.Version
		)

		ok, err := versionutils.CompareVersions(shootClientVersion, "=", kubeVersion)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("shoot client version %q does not match desired version %q", shootClientVersion, kubeVersion)
		}
	}

	o.ShootClientSet = shootClient

	return nil
}

// IsAPIServerRunning checks if the API server of the Shoot currently running (not scaled-down/deleted).
func (o *Operation) IsAPIServerRunning(ctx context.Context) (bool, error) {
	deployment := &appsv1.Deployment{}
	// use API reader here to make sure, we're not reading from a stale cache, when checking if we should initialize a shoot client (e.g. from within the care controller)
	if err := o.SeedClientSet.APIReader().Get(ctx, kubernetesutils.Key(o.Shoot.SeedNamespace, v1beta1constants.DeploymentNameKubeAPIServer), deployment); err != nil {
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
	return common.FilterEntriesByPrefix(kind, o.AllSecretKeys())
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

	if err := o.Shoot.UpdateInfoStatus(ctx, o.GardenClient, true, func(shoot *gardencorev1beta1.Shoot) error {
		if shoot.Status.LastOperation == nil {
			return fmt.Errorf("last operation of Shoot %s/%s is unset", shoot.Namespace, shoot.Name)
		}
		if shoot.Status.LastOperation.LastUpdateTime.After(lastUpdateTime.Time) {
			return fmt.Errorf("last operation of Shoot %s/%s was updated mid-air", shoot.Namespace, shoot.Name)
		}
		if description != "" {
			shoot.Status.LastOperation.Description = description
		}
		shoot.Status.LastOperation.Progress = progress
		shoot.Status.LastOperation.LastUpdateTime = lastUpdateTime
		return nil
	}); err != nil {
		o.Logger.Error(err, "Could not report shoot progress")
	}
}

// CleanShootTaskError removes the error with taskID from the Shoot's status.LastErrors array.
// If the status.LastErrors array is empty then status.LastErrors is also removed.
func (o *Operation) CleanShootTaskError(ctx context.Context, taskID string) {
	if err := o.Shoot.UpdateInfoStatus(ctx, o.GardenClient, false, func(shoot *gardencorev1beta1.Shoot) error {
		shoot.Status.LastErrors = v1beta1helper.DeleteLastErrorByTaskID(shoot.Status.LastErrors, taskID)
		return nil
	}); err != nil {
		o.Logger.Error(err, "Could not update last errors of shoot", "shoot", client.ObjectKeyFromObject(o.Shoot.GetInfo()))
	}
}

// SeedVersion is a shorthand for the kubernetes version of the SeedClientSet.
func (o *Operation) SeedVersion() string {
	return o.SeedClientSet.Version()
}

// ShootVersion is a shorthand for the desired kubernetes version of the operation's shoot.
func (o *Operation) ShootVersion() string {
	return o.Shoot.GetInfo().Spec.Kubernetes.Version
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

// EnsureShootStateExists creates the ShootState resource for the corresponding shoot and updates the operations object
func (o *Operation) EnsureShootStateExists(ctx context.Context) error {
	var (
		err        error
		shootState = &gardencorev1beta1.ShootState{
			ObjectMeta: metav1.ObjectMeta{
				Name:      o.Shoot.GetInfo().Name,
				Namespace: o.Shoot.GetInfo().Namespace,
			},
		}
	)

	if err = o.GardenClient.Create(ctx, shootState); client.IgnoreAlreadyExists(err) != nil {
		return err
	}

	if err = o.GardenClient.Get(ctx, client.ObjectKeyFromObject(shootState), shootState); err != nil {
		return err
	}
	o.SetShootState(shootState)

	return nil
}

// DeleteShootState deletes the ShootState resource for the corresponding shoot.
func (o *Operation) DeleteShootState(ctx context.Context) error {
	shootState := &gardencorev1beta1.ShootState{
		ObjectMeta: metav1.ObjectMeta{
			Name:      o.Shoot.GetInfo().Name,
			Namespace: o.Shoot.GetInfo().Namespace,
		},
	}

	if err := gardenerutils.ConfirmDeletion(ctx, o.GardenClient, shootState); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	return client.IgnoreNotFound(o.GardenClient.Delete(ctx, shootState))
}

// GetShootState returns the shootstate resource of this Shoot in a concurrency safe way.
// This method should be used only for reading the data of the returned shootstate resource. The returned shootstate
// resource MUST NOT BE MODIFIED (except in test code) since this might interfere with other concurrent reads and writes.
// To properly update the shootstate resource of this Shoot use SaveGardenerResourceDataInShootState.
func (o *Operation) GetShootState() *gardencorev1beta1.ShootState {
	shootState, ok := o.shootState.Load().(*gardencorev1beta1.ShootState)
	if !ok {
		return nil
	}
	return shootState
}

// SetShootState sets the shootstate resource of this Shoot in a concurrency safe way.
// This method is not protected by a mutex and does not update the shootstate resource in the cluster and so
// should be used only in exceptional situations, or as a convenience in test code. The shootstate passed as a parameter
// MUST NOT BE MODIFIED after the call to SetShootState (except in test code) since this might interfere with other concurrent reads and writes.
// To properly update the shootstate resource of this Shoot use SaveGardenerResourceDataInShootState.
func (o *Operation) SetShootState(shootState *gardencorev1beta1.ShootState) {
	o.shootState.Store(shootState)
}

// SaveGardenerResourceDataInShootState updates the shootstate resource of this Shoot in a concurrency safe way,
// using the given context and mutate function.
// The mutate function should modify the passed GardenerResourceData so that changes are persisted.
// This method is protected by a mutex, so only a single SaveGardenerResourceDataInShootState operation can be
// executed at any point in time.
func (o *Operation) SaveGardenerResourceDataInShootState(ctx context.Context, f func(*[]gardencorev1beta1.GardenerResourceData) error) error {
	o.shootStateMutex.Lock()
	defer o.shootStateMutex.Unlock()

	shootState := o.GetShootState().DeepCopy()
	original := shootState.DeepCopy()
	patch := client.StrategicMergeFrom(original)

	if err := f(&shootState.Spec.Gardener); err != nil {
		return err
	}
	if equality.Semantic.DeepEqual(original.Spec.Gardener, shootState.Spec.Gardener) {
		return nil
	}
	if err := o.GardenClient.Patch(ctx, shootState, patch); err != nil {
		return err
	}
	o.SetShootState(shootState)
	return nil
}

// DeleteClusterResourceFromSeed deletes the `Cluster` extension resource for the shoot in the seed cluster.
func (o *Operation) DeleteClusterResourceFromSeed(ctx context.Context) error {
	return client.IgnoreNotFound(o.SeedClientSet.Client().Delete(ctx, &extensionsv1alpha1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: o.Shoot.SeedNamespace}}))
}

// ComputeGrafanaHosts computes the host for plutono.
func (o *Operation) ComputeGrafanaHosts() []string {
	return []string{
		o.ComputeGrafanaHost(),
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

// ComputeLokiHosts computes the host for vali.
func (o *Operation) ComputeLokiHosts() []string {
	return []string{
		o.ComputeLokiHost(),
	}
}

// IsShootMonitoringEnabled returns true if shoot monitoring is enabled and shoot is not of purpose testing.
func (o *Operation) IsShootMonitoringEnabled() bool {
	return helper.IsMonitoringEnabled(o.Config) && o.Shoot.Purpose != gardencorev1beta1.ShootPurposeTesting
}

// WantsGrafana returns true if shoot is not of purpose testing and either shoot monitoring or vali is enabled.
func (o *Operation) WantsGrafana() bool {
	return o.Shoot.Purpose != gardencorev1beta1.ShootPurposeTesting && (helper.IsMonitoringEnabled(o.Config) || helper.IsLokiEnabled(o.Config))
}

// ComputeKubeAPIServerHost computes the host with a TLS certificate from a trusted origin for KubeAPIServer.
func (o *Operation) ComputeKubeAPIServerHost() string {
	return o.ComputeIngressHost(common.KubeAPIServerPrefix)
}

// ComputeGrafanaHost computes the host for Grafana.
func (o *Operation) ComputeGrafanaHost() string {
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

// ComputeLokiHost computes the host for vali.
func (o *Operation) ComputeLokiHost() string {
	return o.ComputeIngressHost(common.LokiPrefix)
}

// technicalIDPattern addresses the ambiguity that one or two dashes could follow the prefix "shoot" in the technical ID of the shoot.
var technicalIDPattern = regexp.MustCompile(fmt.Sprintf("^%s-?", v1beta1constants.TechnicalIDPrefix))

// ComputeIngressHost computes the host for a given prefix.
func (o *Operation) ComputeIngressHost(prefix string) string {
	shortID := technicalIDPattern.ReplaceAllString(o.Shoot.GetInfo().Status.TechnicalID, "")
	return fmt.Sprintf("%s-%s.%s", prefix, shortID, o.Seed.IngressDomain())
}

// ToAdvertisedAddresses returns list of advertised addresses on a Shoot cluster.
func (o *Operation) ToAdvertisedAddresses() []gardencorev1beta1.ShootAdvertisedAddress {
	var addresses []gardencorev1beta1.ShootAdvertisedAddress

	if o.Shoot == nil {
		return addresses
	}

	if o.Shoot.ExternalClusterDomain != nil && len(*o.Shoot.ExternalClusterDomain) > 0 {
		addresses = append(addresses, gardencorev1beta1.ShootAdvertisedAddress{
			Name: "external",
			URL:  "https://" + gardenerutils.GetAPIServerDomain(*o.Shoot.ExternalClusterDomain),
		})
	}

	if len(o.Shoot.InternalClusterDomain) > 0 {
		addresses = append(addresses, gardencorev1beta1.ShootAdvertisedAddress{
			Name: "internal",
			URL:  "https://" + gardenerutils.GetAPIServerDomain(o.Shoot.InternalClusterDomain),
		})
	}

	if len(o.APIServerAddress) > 0 && len(addresses) == 0 {
		addresses = append(addresses, gardencorev1beta1.ShootAdvertisedAddress{
			Name: "unmanaged",
			URL:  "https://" + o.APIServerAddress,
		})
	}

	return addresses
}

// UpdateAdvertisedAddresses updates the shoot.status.advertisedAddresses with the list of
// addresses on which the API server of the shoot is accessible.
func (o *Operation) UpdateAdvertisedAddresses(ctx context.Context) error {
	return o.Shoot.UpdateInfoStatus(ctx, o.GardenClient, false, func(shoot *gardencorev1beta1.Shoot) error {
		shoot.Status.AdvertisedAddresses = o.ToAdvertisedAddresses()
		return nil
	})
}

// StoreSecret stores the passed secret under the given key from the operation. Calling this function is thread-safe.
func (o *Operation) StoreSecret(key string, secret *corev1.Secret) {
	o.secretsMutex.Lock()
	defer o.secretsMutex.Unlock()

	if o.secrets == nil {
		o.secrets = make(map[string]*corev1.Secret)
	}

	o.secrets[key] = secret
}

// AllSecretKeys returns all stored secret keys from the operation. Calling this function is thread-safe.
func (o *Operation) AllSecretKeys() []string {
	o.secretsMutex.RLock()
	defer o.secretsMutex.RUnlock()

	var keys []string
	for key := range o.secrets {
		keys = append(keys, key)
	}
	return keys
}

// LoadSecret loads the secret under the given key from the operation. Calling this function is thread-safe.
// Be aware that the returned pointer and the underlying secret map refer to the same secret object.
// If you need to modify the returned secret, copy it first and store the changes via `StoreSecret`.
func (o *Operation) LoadSecret(key string) *corev1.Secret {
	o.secretsMutex.RLock()
	defer o.secretsMutex.RUnlock()

	val := o.secrets[key]
	return val
}

// DeleteSecret deleted the secret under the given key from the operation. Calling this function is thread-safe.
func (o *Operation) DeleteSecret(key string) {
	o.secretsMutex.Lock()
	defer o.secretsMutex.Unlock()

	delete(o.secrets, key)
}
