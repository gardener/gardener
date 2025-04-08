// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package operation

import (
	"context"
	"fmt"
	"regexp"

	"github.com/Masterminds/semver/v3"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1/helper"
	"github.com/gardener/gardener/pkg/gardenlet/operation/garden"
	"github.com/gardener/gardener/pkg/gardenlet/operation/seed"
	shootpkg "github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
	"github.com/gardener/gardener/pkg/utils/flow"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	versionutils "github.com/gardener/gardener/pkg/utils/version"
)

// NewBuilder returns a new Builder.
func NewBuilder() *Builder {
	return &Builder{
		clockFunc: func() clock.Clock {
			return clock.RealClock{}
		},
		configFunc: func() (*gardenletconfigv1alpha1.GardenletConfiguration, error) {
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
		loggerFunc: func() (logr.Logger, error) {
			return logr.Discard(), fmt.Errorf("logger is required but not set")
		},
		secretsFunc: func() (map[string]*corev1.Secret, error) {
			return nil, fmt.Errorf("secrets map is required but not set")
		},
		seedFunc: func(context.Context) (*seed.Seed, error) {
			return nil, fmt.Errorf("seed object is required but not set")
		},
		shootFunc: func(context.Context, client.Reader, *garden.Garden, *seed.Seed, *corev1.Secret) (*shootpkg.Shoot, error) {
			return nil, fmt.Errorf("shoot object is required but not set")
		},
	}
}

// WithConfig sets the configFunc attribute at the Builder.
func (b *Builder) WithConfig(cfg *gardenletconfigv1alpha1.GardenletConfiguration) *Builder {
	b.configFunc = func() (*gardenletconfigv1alpha1.GardenletConfiguration, error) { return cfg, nil }
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
	b.shootFunc = func(_ context.Context, _ client.Reader, _ *garden.Garden, _ *seed.Seed, _ *corev1.Secret) (*shootpkg.Shoot, error) {
		return s, nil
	}
	return b
}

// WithShootFromCluster sets the shootFunc attribute at the Builder which will build a new Shoot object constructed from the cluster resource.
// The shoot status is still taken from the passed `shoot`, though.
// The credentials in the Shoot object are always set to `nil`.
func (b *Builder) WithShootFromCluster(seedClientSet kubernetes.Interface, s *gardencorev1beta1.Shoot) *Builder {
	b.shootFunc = func(ctx context.Context, c client.Reader, gardenObj *garden.Garden, seedObj *seed.Seed, serviceAccountIssuerConfig *corev1.Secret) (*shootpkg.Shoot, error) {
		controlPlaneNamespace := v1beta1helper.ControlPlaneNamespaceForShoot(s)

		shoot, err := shootpkg.
			NewBuilder().
			WithShootObjectFromCluster(seedClientSet, controlPlaneNamespace).
			WithCloudProfileObjectFromCluster(seedClientSet, controlPlaneNamespace).
			WithoutShootCredentials().
			WithSeedObject(seedObj.GetInfo()).
			WithProjectName(gardenObj.Project.Name).
			WithInternalDomain(gardenObj.InternalDomain).
			WithDefaultDomains(gardenObj.DefaultDomains).
			WithServiceAccountIssuerHostname(serviceAccountIssuerConfig).
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

// WithClock sets the clockFunc attribute at the Builder.
func (b *Builder) WithClock(c clock.Clock) *Builder {
	b.clockFunc = func() clock.Clock { return c }
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
		Clock:          b.clockFunc(),
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
	secrets := make(map[string]*corev1.Secret, len(secretsMap))
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

	shoot, err := b.shootFunc(ctx, gardenClient, garden, seed, secrets[v1beta1constants.GardenRoleShootServiceAccountIssuer])
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
	if err := o.SeedClientSet.APIReader().Get(ctx, client.ObjectKey{Namespace: o.Shoot.ControlPlaneNamespace, Name: v1beta1constants.DeploymentNameKubeAPIServer}, deployment); err != nil {
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

// ReportShootProgress will update the last operation object in the Shoot manifest `status` section
// by the current progress of the Flow execution.
func (o *Operation) ReportShootProgress(ctx context.Context, stats *flow.Stats) {
	var (
		description    = flow.MakeDescription(stats)
		progress       = stats.ProgressPercent()
		lastUpdateTime = metav1.Now()
	)

	if err := o.Shoot.UpdateInfoStatus(ctx, o.GardenClient, true, false, func(shoot *gardencorev1beta1.Shoot) error {
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
	if err := o.Shoot.UpdateInfoStatus(ctx, o.GardenClient, false, false, func(shoot *gardencorev1beta1.Shoot) error {
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

// DeleteClusterResourceFromSeed deletes the `Cluster` extension resource for the shoot in the seed cluster.
func (o *Operation) DeleteClusterResourceFromSeed(ctx context.Context) error {
	return client.IgnoreNotFound(o.SeedClientSet.Client().Delete(ctx, &extensionsv1alpha1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: o.Shoot.ControlPlaneNamespace}}))
}

// IsShootMonitoringEnabled returns true if shoot monitoring is enabled and shoot is not of purpose testing.
func (o *Operation) IsShootMonitoringEnabled() bool {
	return helper.IsMonitoringEnabled(o.Config) && o.Shoot.Purpose != gardencorev1beta1.ShootPurposeTesting
}

// WantsObservabilityComponents returns true if shoot is not of purpose testing and either shoot monitoring or vali is enabled.
func (o *Operation) WantsObservabilityComponents() bool {
	return o.Shoot.Purpose != gardencorev1beta1.ShootPurposeTesting && (helper.IsMonitoringEnabled(o.Config) || helper.IsValiEnabled(o.Config))
}

// ComputeKubeAPIServerHost computes the host with a TLS certificate from a trusted origin for KubeAPIServer.
func (o *Operation) ComputeKubeAPIServerHost() string {
	return o.ComputeIngressHost("api")
}

// ComputePlutonoHost computes the host for Plutono.
func (o *Operation) ComputePlutonoHost() string {
	return o.ComputeIngressHost("gu")
}

// ComputeAlertManagerHost computes the host for alert manager.
func (o *Operation) ComputeAlertManagerHost() string {
	return o.ComputeIngressHost("au")
}

// ComputePrometheusHost computes the host for prometheus.
func (o *Operation) ComputePrometheusHost() string {
	return o.ComputeIngressHost("p")
}

// ComputeValiHost computes the host for vali.
func (o *Operation) ComputeValiHost() string {
	return o.ComputeIngressHost("v")
}

// technicalIDPattern addresses the ambiguity that one or two dashes could follow the prefix "shoot" in the technical ID of the shoot.
var technicalIDPattern = regexp.MustCompile(fmt.Sprintf("^%s-?", v1beta1constants.TechnicalIDPrefix))

// ComputeIngressHost computes the host for a given prefix.
func (o *Operation) ComputeIngressHost(prefix string) string {
	shortID := technicalIDPattern.ReplaceAllString(o.Shoot.GetInfo().Status.TechnicalID, "")
	return fmt.Sprintf("%s-%s.%s", prefix, shortID, o.Seed.IngressDomain())
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
