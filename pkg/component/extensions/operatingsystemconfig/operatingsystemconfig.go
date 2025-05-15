// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package operatingsystemconfig

import (
	"bytes"
	"context"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/go-logr/logr"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/imagevector"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/nodeinit"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/gardeneruser"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/nodeagent"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/sshdensurer"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/extensions"
	"github.com/gardener/gardener/pkg/features"
	nodeagentconfigv1alpha1 "github.com/gardener/gardener/pkg/nodeagent/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/flow"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	imagevectorutils "github.com/gardener/gardener/pkg/utils/imagevector"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	"github.com/gardener/gardener/pkg/utils/version"
)

const (
	// DefaultInterval is the default interval for retry operations.
	DefaultInterval = 5 * time.Second
	// DefaultSevereThreshold is the default threshold until an error reported by another component is treated as
	// 'severe'.
	DefaultSevereThreshold = 30 * time.Second
	// DefaultTimeout is the default timeout and defines how long Gardener should wait for a successful reconciliation
	// of an OperatingSystemConfig resource.
	DefaultTimeout = 3 * time.Minute
	// WorkerPoolHashesSecretName is the name of the secret that tracks the OSC key calculation version used for each worker pool.
	WorkerPoolHashesSecretName = "worker-pools-operatingsystemconfig-hashes" // #nosec G101 -- No credential.
	// poolHashesDataKey is the key in the data of the WorkerPoolHashesSecretName used to store the calculated hashes.
	poolHashesDataKey = "pools"
)

// LatestHashVersion is the latest version support for calculateKeyVersion. Exposed for testing.
var LatestHashVersion = func() int {
	// WorkerPoolHash is behind feature gate as extensions must be updated first
	if features.DefaultFeatureGate.Enabled(features.NewWorkerPoolHash) {
		return 2
	}
	return 1
}

// TimeNow returns the current time. Exposed for testing.
var TimeNow = time.Now

// Interface is an interface for managing OperatingSystemConfigs.
type Interface interface {
	component.DeployMigrateWaiter
	// DeleteStaleResources deletes unused OperatingSystemConfig resources from the shoot namespace in the seed.
	DeleteStaleResources(context.Context) error
	// WaitCleanupStaleResources waits until all unused OperatingSystemConfig resources are cleaned up.
	WaitCleanupStaleResources(context.Context) error
	// SetAPIServerURL sets the APIServerURL value.
	SetAPIServerURL(string)
	// SetCABundle sets the CABundle value.
	SetCABundle(*string)
	// SetCredentialsRotationStatus sets the credentials rotation status
	SetCredentialsRotationStatus(*gardencorev1beta1.ShootCredentialsRotation)
	// SetSSHPublicKeys sets the SSHPublicKeys value.
	SetSSHPublicKeys([]string)
	// WorkerPoolNameToOperatingSystemConfigsMap returns a map whose key is a worker pool name and whose value is a structure
	// containing both the init and the original operating system config data.
	WorkerPoolNameToOperatingSystemConfigsMap() map[string]*OperatingSystemConfigs
	// SetClusterDNSAddresses sets the cluster DNS addresses.
	SetClusterDNSAddresses([]string)
}

// Values contains the values used to create an OperatingSystemConfig resource.
type Values struct {
	// Namespace is the namespace for the OperatingSystemConfig resource.
	Namespace string
	// KubernetesVersion is the version for the kubelets of all worker pools.
	KubernetesVersion *semver.Version
	// Workers is the list of worker pools.
	Workers []gardencorev1beta1.Worker
	// CredentialsRotationStatus
	CredentialsRotationStatus *gardencorev1beta1.ShootCredentialsRotation

	// InitValues are configuration values required for the 'provision' OperatingSystemConfigPurpose.
	InitValues
	// OriginalValues are configuration values required for the 'reconcile' OperatingSystemConfigPurpose.
	OriginalValues
}

// InitValues are configuration values required for the 'provision' OperatingSystemConfigPurpose.
type InitValues struct {
	// APIServerURL is the address (including https:// protocol prefix) to the kube-apiserver (from which the original
	// cloud-config user data will be downloaded).
	APIServerURL string
}

// OriginalValues are configuration values required for the 'reconcile' OperatingSystemConfigPurpose.
type OriginalValues struct {
	// CABundle is the bundle of certificate authorities that will be added as root certificates.
	CABundle *string
	// ClusterDNSAddresses are the addresses for in-cluster DNS.
	ClusterDNSAddresses []string
	// ClusterDomain is the Kubernetes cluster domain.
	ClusterDomain string
	// Images is a map containing the necessary container images for the systemd units (hyperkube and pause-container).
	Images map[string]*imagevectorutils.Image
	// KubeletConfig is the default kubelet configuration for all worker pools. Individual worker pools might overwrite
	// this configuration.
	KubeletConfig *gardencorev1beta1.KubeletConfig
	// KubeProxyEnabled indicates whether kube-proxy is enabled or not.
	KubeProxyEnabled bool
	// MachineTypes is a list of machine types.
	MachineTypes []gardencorev1beta1.MachineType
	// SSHPublicKeys is a list of public SSH keys.
	SSHPublicKeys []string
	// SSHAccessEnabled states whether sshd.service service in systemd should be enabled and running for the worker nodes.
	SSHAccessEnabled bool
	// ValitailEnabled states whether Valitail shall be enabled.
	ValitailEnabled bool
	// ValiIngressHostName is the ingress host name of the shoot's Vali.
	ValiIngressHostName string
	// NodeMonitorGracePeriod defines the grace period before an unresponsive node is marked unhealthy.
	NodeMonitorGracePeriod metav1.Duration
	// NodeLocalDNSEnabled indicates whether node local dns is enabled or not.
	NodeLocalDNSEnabled bool
	// PrimaryIPFamily represents the preferred IP family (IPv4 or IPv6) to be used.
	PrimaryIPFamily gardencorev1beta1.IPFamily
}

// New creates a new instance of Interface.
func New(
	log logr.Logger,
	client client.Client,
	secretsManager secretsmanager.Interface,
	values *Values,
	waitInterval time.Duration,
	waitSevereThreshold time.Duration,
	waitTimeout time.Duration,
) Interface {
	osc := &operatingSystemConfig{
		log:                 log,
		client:              client,
		secretsManager:      secretsManager,
		values:              values,
		waitInterval:        waitInterval,
		waitSevereThreshold: waitSevereThreshold,
		waitTimeout:         waitTimeout,
	}

	osc.workerPoolNameToOSCs = make(map[string]*OperatingSystemConfigs, len(values.Workers))
	for _, worker := range values.Workers {
		osc.workerPoolNameToOSCs[worker.Name] = &OperatingSystemConfigs{}
	}
	osc.oscs = make(map[string]*extensionsv1alpha1.OperatingSystemConfig, len(osc.workerPoolNameToOSCs)*2)

	return osc
}

type operatingSystemConfig struct {
	log            logr.Logger
	client         client.Client
	secretsManager secretsmanager.Interface
	values         *Values

	waitInterval        time.Duration
	waitSevereThreshold time.Duration
	waitTimeout         time.Duration

	lock                        sync.Mutex
	workerPoolNameToOSCs        map[string]*OperatingSystemConfigs
	oscs                        map[string]*extensionsv1alpha1.OperatingSystemConfig
	workerPoolNameToHashVersion map[string]int
}

// OperatingSystemConfigs contains operating system configs for the init script as well as for the original config.
type OperatingSystemConfigs struct {
	// Init is the data for the init script.
	Init Data
	// Original is the data for the to-be-downloaded original config.
	Original Data
}

// Data contains the actual content, a command to load it and all units that shall be considered for restart on change.
type Data struct {
	// Object is the plain OperatingSystemConfig object.
	Object *extensionsv1alpha1.OperatingSystemConfig
	// IncludeSecretNameInWorkerPool states whether a extensionsv1alpha1.WorkerPool must include the GardenerNodeAgentSecretName
	IncludeSecretNameInWorkerPool bool
	// GardenerNodeAgentSecretName is the name of the secret storing the gardener node agent configuration in the shoot cluster.
	GardenerNodeAgentSecretName string
	// SecretName is the name of a secret storing the actual cloud-config user data.
	SecretName *string
}

// Deploy uses the client to create or update the OperatingSystemConfig custom resources.
func (o *operatingSystemConfig) Deploy(ctx context.Context) error {
	return o.reconcile(ctx, func(d deployer) error {
		_, err := d.deploy(ctx, v1beta1constants.GardenerOperationReconcile)
		return err
	})
}

// Restore uses the seed client and the ShootState to create the OperatingSystemConfig custom resources in the Shoot
// namespace in the Seed and restore its state.
func (o *operatingSystemConfig) Restore(ctx context.Context, shootState *gardencorev1beta1.ShootState) error {
	return o.reconcile(ctx, func(d deployer) error {
		return extensions.RestoreExtensionWithDeployFunction(ctx, o.client, shootState, extensionsv1alpha1.OperatingSystemConfigResource, d.deploy)
	})
}

func (o *operatingSystemConfig) reconcile(ctx context.Context, reconcileFn func(deployer) error) error {
	if !features.DefaultFeatureGate.Enabled(features.NodeAgentAuthorizer) {
		if err := gardenerutils.
			NewShootAccessSecret(nodeagentconfigv1alpha1.AccessSecretName, o.values.Namespace).
			WithTargetSecret(nodeagentconfigv1alpha1.AccessSecretName, metav1.NamespaceSystem).
			WithTokenExpirationDuration("720h").
			Reconcile(ctx, o.client); err != nil {
			return err
		}
	}

	if err := o.updateHashVersioningSecret(ctx); err != nil {
		return err
	}

	fns, err := o.forEachWorkerPoolAndPurposeTaskFn(ctx, func(ctx context.Context, hashVersion int, osc *extensionsv1alpha1.OperatingSystemConfig, worker gardencorev1beta1.Worker, purpose extensionsv1alpha1.OperatingSystemConfigPurpose) error {
		d, err := o.newDeployer(ctx, hashVersion, osc, worker, purpose)
		if err != nil {
			return err
		}

		if err := reconcileFn(d); err != nil {
			return fmt.Errorf("failed reconciling OperatingSystemConfig %s for worker %s: %w", client.ObjectKeyFromObject(osc), worker.Name, err)
		}

		oscKey, err := o.calculateKeyForVersion(ctx, hashVersion, &worker)
		if err != nil {
			return err
		}

		data := Data{
			Object:                        osc,
			IncludeSecretNameInWorkerPool: hashVersion > 1,
			GardenerNodeAgentSecretName:   oscKey,
		}

		o.lock.Lock()
		defer o.lock.Unlock()

		switch purpose {
		case extensionsv1alpha1.OperatingSystemConfigPurposeProvision:
			o.workerPoolNameToOSCs[worker.Name].Init = data
		case extensionsv1alpha1.OperatingSystemConfigPurposeReconcile:
			o.workerPoolNameToOSCs[worker.Name].Original = data
		default:
			return fmt.Errorf("unknown purpose %q", purpose)
		}

		return nil
	})
	if err != nil {
		return err
	}

	return flow.Parallel(fns...)(ctx)
}

type poolHash struct {
	Pools []poolHashEntry `yaml:"pools"`
}

type poolHashEntry struct {
	Name                string         `yaml:"name"`
	CurrentVersion      int            `yaml:"currentVersion"`
	HashVersionToOSCKey map[int]string `yaml:"hashVersionToOSCKey"`
}

func decodePoolHashes(secret *corev1.Secret) (map[string]poolHashEntry, error) {
	var pools poolHash

	versions := secret.Data[poolHashesDataKey]
	if len(versions) > 0 {
		if err := yaml.NewDecoder(bytes.NewReader(versions)).Decode(&pools); err != nil {
			return nil, err
		}
	}

	workerPoolNameToHashEntry := make(map[string]poolHashEntry)
	for _, entry := range pools.Pools {
		workerPoolNameToHashEntry[entry.Name] = entry
	}

	return workerPoolNameToHashEntry, nil
}

func encodePoolHashes(pools *poolHash, secret *corev1.Secret) error {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	if err := enc.Encode(&pools); err != nil {
		return err
	}

	if secret.Data == nil {
		secret.Data = map[string][]byte{}
	}
	secret.Data[poolHashesDataKey] = buf.Bytes()
	return nil
}

func (o *operatingSystemConfig) updateHashVersioningSecret(ctx context.Context) error {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: o.values.Namespace, Name: WorkerPoolHashesSecretName},
	}

	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, o.client, secret, func() error {
		workerPoolNameToHashEntry, err := decodePoolHashes(secret)
		if err != nil {
			return err
		}

		var pools poolHash
		for _, worker := range o.values.Workers {
			workerHash, ok := workerPoolNameToHashEntry[worker.Name]
			if !ok {
				workerHash.Name = worker.Name
				workerHash.CurrentVersion = LatestHashVersion()
			}

			if !v1beta1helper.IsUpdateStrategyInPlace(worker.UpdateStrategy) || len(workerHash.HashVersionToOSCKey) == 0 {
				// check if hashes still match
				hashHasChanged := false
				for version, hash := range workerHash.HashVersionToOSCKey {
					expectedHash, err := o.calculateKeyForVersion(ctx, version, &worker)
					if err != nil {
						return err
					}
					if hash != expectedHash {
						hashHasChanged = true
						break
					}
				}

				if hashHasChanged {
					workerHash.CurrentVersion = LatestHashVersion()
				}

				// calculate expected hashes
				currentHash, err := o.calculateKeyForVersion(ctx, workerHash.CurrentVersion, &worker)
				if err != nil {
					return err
				}
				latestHash, err := o.calculateKeyForVersion(ctx, LatestHashVersion(), &worker)
				if err != nil {
					return err
				}

				// rebuild hashes
				clear(workerHash.HashVersionToOSCKey)
				if workerHash.HashVersionToOSCKey == nil {
					workerHash.HashVersionToOSCKey = map[int]string{}
				}
				workerHash.HashVersionToOSCKey[workerHash.CurrentVersion] = currentHash
				workerHash.HashVersionToOSCKey[LatestHashVersion()] = latestHash
			}

			// update secret
			workerPoolNameToHashEntry[worker.Name] = workerHash

			pools.Pools = append(pools.Pools, workerHash)
		}

		if err := encodePoolHashes(&pools, secret); err != nil {
			return err
		}
		metav1.SetMetaDataLabel(&secret.ObjectMeta, secretsmanager.LabelKeyPersist, "true")
		secret.Type = corev1.SecretTypeOpaque
		return nil
	}); err != nil {
		return err
	}

	workerPoolNameToHashEntry, err := decodePoolHashes(secret)
	if err != nil {
		return err
	}

	o.workerPoolNameToHashVersion = make(map[string]int, len(workerPoolNameToHashEntry))
	for name, entry := range workerPoolNameToHashEntry {
		o.workerPoolNameToHashVersion[name] = entry.CurrentVersion
	}

	return nil
}

func (o *operatingSystemConfig) hashVersion(workerPoolName string) (int, error) {
	// updateHashVersioningSecret() is currently always called before this method
	// thus just implement a sanity check instead of querying the hash version secret
	if o.workerPoolNameToHashVersion == nil {
		return 0, fmt.Errorf("hash version not yet synced")
	}

	if version, ok := o.workerPoolNameToHashVersion[workerPoolName]; ok {
		return version, nil
	}
	return 0, fmt.Errorf("no version available for %v", workerPoolName)
}

// Wait waits until the OperatingSystemConfig CRD is ready (deployed or restored). It also reads the produced secret
// containing the cloud-config and stores its data which can later be retrieved with the WorkerPoolNameToOperatingSystemConfigsMap
// method.
func (o *operatingSystemConfig) Wait(ctx context.Context) error {
	fns, err := o.forEachWorkerPoolAndPurposeTaskFn(ctx, func(ctx context.Context, _ int, osc *extensionsv1alpha1.OperatingSystemConfig, worker gardencorev1beta1.Worker, purpose extensionsv1alpha1.OperatingSystemConfigPurpose) error {
		return extensions.WaitUntilExtensionObjectReady(ctx,
			o.client,
			o.log,
			osc,
			extensionsv1alpha1.OperatingSystemConfigResource,
			o.waitInterval,
			o.waitSevereThreshold,
			o.waitTimeout,
			func() error {
				if purpose != extensionsv1alpha1.OperatingSystemConfigPurposeProvision {
					return nil
				}

				if osc.Status.CloudConfig == nil {
					return fmt.Errorf("no cloud config information provided in status")
				}

				secret := &corev1.Secret{}
				if err := o.client.Get(ctx, client.ObjectKey{Namespace: osc.Status.CloudConfig.SecretRef.Namespace, Name: osc.Status.CloudConfig.SecretRef.Name}, secret); err != nil {
					return err
				}

				o.lock.Lock()
				defer o.lock.Unlock()

				o.workerPoolNameToOSCs[worker.Name].Init.SecretName = &secret.Name
				return nil
			},
		)
	})
	if err != nil {
		return err
	}

	return flow.ParallelExitOnError(fns...)(ctx)
}

// Migrate migrates the OperatingSystemConfig custom resources.
func (o *operatingSystemConfig) Migrate(ctx context.Context) error {
	return extensions.MigrateExtensionObjects(
		ctx,
		o.client,
		&extensionsv1alpha1.OperatingSystemConfigList{},
		o.values.Namespace,
		nil,
	)
}

// WaitMigrate waits until the OperatingSystemConfig custom resource have been successfully migrated.
func (o *operatingSystemConfig) WaitMigrate(ctx context.Context) error {
	return extensions.WaitUntilExtensionObjectsMigrated(
		ctx,
		o.client,
		&extensionsv1alpha1.OperatingSystemConfigList{},
		extensionsv1alpha1.OperatingSystemConfigResource,
		o.values.Namespace,
		o.waitInterval,
		o.waitTimeout,
		nil,
	)
}

// Destroy deletes all the OperatingSystemConfig resources.
func (o *operatingSystemConfig) Destroy(ctx context.Context) error {
	if err := o.deleteOperatingSystemConfigResources(ctx, sets.New[string]()); err != nil {
		return err
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: o.values.Namespace, Name: WorkerPoolHashesSecretName},
	}
	return client.IgnoreNotFound(o.client.Delete(ctx, secret))
}

func (o *operatingSystemConfig) deleteOperatingSystemConfigResources(ctx context.Context, wantedOSCNames sets.Set[string]) error {
	return extensions.DeleteExtensionObjects(
		ctx,
		o.client,
		&extensionsv1alpha1.OperatingSystemConfigList{},
		o.values.Namespace,
		func(obj extensionsv1alpha1.Object) bool {
			return !wantedOSCNames.Has(obj.GetName())
		},
	)
}

// WaitCleanup waits until all OperatingSystemConfig resources are cleaned up.
func (o *operatingSystemConfig) WaitCleanup(ctx context.Context) error {
	return o.waitCleanup(ctx, sets.New[string]())
}

// DeleteStaleResources deletes unused OperatingSystemConfig resources from the shoot namespace in the seed.
func (o *operatingSystemConfig) DeleteStaleResources(ctx context.Context) error {
	wantedOSCs, err := o.getWantedOSCNames(ctx)
	if err != nil {
		return err
	}
	return o.deleteOperatingSystemConfigResources(ctx, wantedOSCs)
}

// WaitCleanupStaleResources waits until all unused OperatingSystemConfig resources are cleaned up.
func (o *operatingSystemConfig) WaitCleanupStaleResources(ctx context.Context) error {
	wantedOSCs, err := o.getWantedOSCNames(ctx)
	if err != nil {
		return err
	}
	return o.waitCleanup(ctx, wantedOSCs)
}

func (o *operatingSystemConfig) waitCleanup(ctx context.Context, wantedOSCNames sets.Set[string]) error {
	return extensions.WaitUntilExtensionObjectsDeleted(
		ctx,
		o.client,
		o.log,
		&extensionsv1alpha1.OperatingSystemConfigList{},
		extensionsv1alpha1.OperatingSystemConfigResource,
		o.values.Namespace,
		o.waitInterval,
		o.waitTimeout,
		func(obj extensionsv1alpha1.Object) bool {
			return !wantedOSCNames.Has(obj.GetName())
		},
	)
}

// getWantedOSCNames returns the names of all OSC resources, that are currently needed based
// on the configured worker pools.
func (o *operatingSystemConfig) getWantedOSCNames(ctx context.Context) (sets.Set[string], error) {
	wantedOSCNames := sets.New[string]()

	for _, worker := range o.values.Workers {
		version, err := o.hashVersion(worker.Name)
		if err != nil {
			return nil, err
		}

		for _, purpose := range []extensionsv1alpha1.OperatingSystemConfigPurpose{
			extensionsv1alpha1.OperatingSystemConfigPurposeProvision,
			extensionsv1alpha1.OperatingSystemConfigPurposeReconcile,
		} {
			oscKey, err := o.calculateKeyForVersion(ctx, version, &worker)
			if err != nil {
				return nil, err
			}
			wantedOSCNames.Insert(oscKey + keySuffix(version, worker.Machine.Image, purpose))
		}
	}

	return wantedOSCNames, nil
}

func (o *operatingSystemConfig) forEachWorkerPoolAndPurpose(ctx context.Context, fn func(int, *extensionsv1alpha1.OperatingSystemConfig, gardencorev1beta1.Worker, extensionsv1alpha1.OperatingSystemConfigPurpose) error) error {
	for _, worker := range o.values.Workers {
		version, err := o.hashVersion(worker.Name)
		if err != nil {
			return err
		}

		for _, purpose := range []extensionsv1alpha1.OperatingSystemConfigPurpose{
			extensionsv1alpha1.OperatingSystemConfigPurposeProvision,
			extensionsv1alpha1.OperatingSystemConfigPurposeReconcile,
		} {
			oscKey, err := o.calculateKeyForVersion(ctx, version, &worker)
			if err != nil {
				return err
			}
			oscName := oscKey + keySuffix(version, worker.Machine.Image, purpose)

			osc, ok := o.oscs[oscName]
			if !ok {
				osc = &extensionsv1alpha1.OperatingSystemConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      oscName,
						Namespace: o.values.Namespace,
					},
				}
				// store object for later usage (we want to pass a filled object to WaitUntil*)
				o.oscs[oscName] = osc
			}

			if err := fn(version, osc, worker, purpose); err != nil {
				return err
			}
		}
	}

	return nil
}

func (o *operatingSystemConfig) forEachWorkerPoolAndPurposeTaskFn(ctx context.Context, fn func(context.Context, int, *extensionsv1alpha1.OperatingSystemConfig, gardencorev1beta1.Worker, extensionsv1alpha1.OperatingSystemConfigPurpose) error) ([]flow.TaskFn, error) {
	var fns []flow.TaskFn

	err := o.forEachWorkerPoolAndPurpose(ctx, func(version int, osc *extensionsv1alpha1.OperatingSystemConfig, worker gardencorev1beta1.Worker, purpose extensionsv1alpha1.OperatingSystemConfigPurpose) error {
		fns = append(fns, func(ctx context.Context) error {
			return fn(ctx, version, osc, worker, purpose)
		})
		return nil
	})

	return fns, err
}

// SetAPIServerURL sets the APIServerURL value.
func (o *operatingSystemConfig) SetAPIServerURL(apiServerURL string) {
	o.values.APIServerURL = apiServerURL
}

// SetCABundle sets the CABundle value.
func (o *operatingSystemConfig) SetCABundle(val *string) {
	o.values.CABundle = val
}

func (o *operatingSystemConfig) SetCredentialsRotationStatus(status *gardencorev1beta1.ShootCredentialsRotation) {
	o.values.CredentialsRotationStatus = status
}

// SetSSHPublicKeys sets the SSHPublicKeys value.
func (o *operatingSystemConfig) SetSSHPublicKeys(keys []string) {
	o.values.SSHPublicKeys = keys
}

// WorkerPoolNameToOperatingSystemConfigsMap returns a map whose key is a worker pool name and whose value is a structure
// containing both the init script and the original config.
func (o *operatingSystemConfig) WorkerPoolNameToOperatingSystemConfigsMap() map[string]*OperatingSystemConfigs {
	return o.workerPoolNameToOSCs
}

func (o *operatingSystemConfig) SetClusterDNSAddresses(clusterDNSAddresses []string) {
	o.values.ClusterDNSAddresses = clusterDNSAddresses
}

func (o *operatingSystemConfig) newDeployer(ctx context.Context, version int, osc *extensionsv1alpha1.OperatingSystemConfig, worker gardencorev1beta1.Worker, purpose extensionsv1alpha1.OperatingSystemConfigPurpose) (deployer, error) {
	criName := extensionsv1alpha1.CRINameContainerD
	if worker.CRI != nil {
		criName = extensionsv1alpha1.CRIName(worker.CRI.Name)
	}

	caBundle := o.values.CABundle
	if worker.CABundle != nil {
		if caBundle == nil {
			caBundle = worker.CABundle
		} else {
			*caBundle = fmt.Sprintf("%s\n%s", *caBundle, *worker.CABundle)
		}
	}

	clusterCASecret, found := o.secretsManager.Get(v1beta1constants.SecretNameCACluster)
	if !found {
		return deployer{}, fmt.Errorf("secret %q not found", v1beta1constants.SecretNameCACluster)
	}

	kubeletCASecret, found := o.secretsManager.Get(v1beta1constants.SecretNameCAKubelet)
	if !found {
		return deployer{}, fmt.Errorf("secret %q not found", v1beta1constants.SecretNameCAKubelet)
	}

	kubeletConfigParameters := components.KubeletConfigParametersFromCoreV1beta1KubeletConfig(o.values.KubeletConfig)
	kubeletCLIFlags := components.KubeletCLIFlagsFromCoreV1beta1KubeletConfig(o.values.KubeletConfig)
	if worker.Kubernetes != nil && worker.Kubernetes.Kubelet != nil {
		kubeletConfigParameters = components.KubeletConfigParametersFromCoreV1beta1KubeletConfig(worker.Kubernetes.Kubelet)
		kubeletCLIFlags = components.KubeletCLIFlagsFromCoreV1beta1KubeletConfig(worker.Kubernetes.Kubelet)
	}
	if worker.ControlPlane != nil {
		kubeletConfigParameters.WithStaticPodPath = true
	}
	setDefaultEvictionMemoryAvailable(kubeletConfigParameters.EvictionHard, kubeletConfigParameters.EvictionSoft, o.values.MachineTypes, worker.Machine.Type)

	kubernetesVersion, err := v1beta1helper.CalculateEffectiveKubernetesVersion(o.values.KubernetesVersion, worker.Kubernetes)
	if err != nil {
		return deployer{}, err
	}
	kubeletConfig := v1beta1helper.CalculateEffectiveKubeletConfiguration(o.values.KubeletConfig, worker.Kubernetes)

	images := make(map[string]*imagevectorutils.Image, len(o.values.Images))
	for imageName, image := range o.values.Images {
		images[imageName] = image
	}

	images[imagevector.ContainerImageNameHyperkube], err = imagevector.Containers().FindImage(imagevector.ContainerImageNameHyperkube, imagevectorutils.RuntimeVersion(kubernetesVersion.String()), imagevectorutils.TargetVersion(kubernetesVersion.String()))
	if err != nil {
		return deployer{}, fmt.Errorf("failed finding hyperkube image for version %s: %w", kubernetesVersion.String(), err)
	}

	oscKey, err := o.calculateKeyForVersion(ctx, version, &worker)
	if err != nil {
		return deployer{}, err
	}

	var caRotationLastInitiationTime, serviceAccountKeyRotationLastInitiationTime *metav1.Time

	if o.values.CredentialsRotationStatus != nil {
		if o.values.CredentialsRotationStatus.CertificateAuthorities != nil {
			caRotationLastInitiationTime = o.values.CredentialsRotationStatus.CertificateAuthorities.LastInitiationTime
		}
		if o.values.CredentialsRotationStatus.ServiceAccountKey != nil {
			serviceAccountKeyRotationLastInitiationTime = o.values.CredentialsRotationStatus.ServiceAccountKey.LastInitiationTime
		}
	}

	taints := slices.Clone(worker.Taints)
	if key := "node-role.kubernetes.io/control-plane"; worker.ControlPlane != nil && !slices.ContainsFunc(taints, func(taint corev1.Taint) bool {
		return taint.Key == key
	}) {
		taints = append(taints, corev1.Taint{Key: key, Effect: corev1.TaintEffectNoSchedule})
	}

	return deployer{
		client:                       o.client,
		osc:                          osc,
		worker:                       worker,
		purpose:                      purpose,
		key:                          oscKey,
		apiServerURL:                 o.values.APIServerURL,
		caBundle:                     caBundle,
		clusterCASecretName:          clusterCASecret.Name,
		clusterCABundle:              clusterCASecret.Data[secretsutils.DataKeyCertificateBundle],
		clusterDNSAddresses:          o.values.ClusterDNSAddresses,
		clusterDomain:                o.values.ClusterDomain,
		criName:                      criName,
		images:                       images,
		kubeletCABundle:              kubeletCASecret.Data[secretsutils.DataKeyCertificateBundle],
		kubeletConfig:                kubeletConfig,
		kubeletConfigParameters:      kubeletConfigParameters,
		kubeletCLIFlags:              kubeletCLIFlags,
		kubeletDataVolumeName:        worker.KubeletDataVolumeName,
		kubeProxyEnabled:             o.values.KubeProxyEnabled,
		kubernetesVersion:            kubernetesVersion,
		sshPublicKeys:                o.values.SSHPublicKeys,
		sshAccessEnabled:             o.values.SSHAccessEnabled,
		valiIngressHostName:          o.values.ValiIngressHostName,
		valitailEnabled:              o.values.ValitailEnabled,
		nodeMonitorGracePeriod:       o.values.NodeMonitorGracePeriod,
		nodeLocalDNSEnabled:          o.values.NodeLocalDNSEnabled,
		primaryIPFamily:              o.values.PrimaryIPFamily,
		taints:                       taints,
		caRotationLastInitiationTime: caRotationLastInitiationTime,
		serviceAccountKeyRotationLastInitiationTime: serviceAccountKeyRotationLastInitiationTime,
	}, nil
}

func setDefaultEvictionMemoryAvailable(evictionHard, evictionSoft map[string]string, machineTypes []gardencorev1beta1.MachineType, machineType string) {
	evictionHardMemoryAvailable, evictionSoftMemoryAvailable := "100Mi", "200Mi"

	for _, machtype := range machineTypes {
		if machtype.Name == machineType {
			evictionHardMemoryAvailable, evictionSoftMemoryAvailable = "5%", "10%"

			if machtype.Memory.Cmp(resource.MustParse("8Gi")) > 0 {
				evictionHardMemoryAvailable, evictionSoftMemoryAvailable = "1Gi", "1.5Gi"
			}

			break
		}
	}

	if evictionHard == nil {
		evictionHard = make(map[string]string)
	}
	if evictionHard[components.MemoryAvailable] == "" {
		evictionHard[components.MemoryAvailable] = evictionHardMemoryAvailable
	}

	if evictionSoft == nil {
		evictionSoft = make(map[string]string)
	}
	if evictionSoft[components.MemoryAvailable] == "" {
		evictionSoft[components.MemoryAvailable] = evictionSoftMemoryAvailable
	}
}

type deployer struct {
	client client.Client
	osc    *extensionsv1alpha1.OperatingSystemConfig

	key     string
	worker  gardencorev1beta1.Worker
	purpose extensionsv1alpha1.OperatingSystemConfigPurpose

	// init values
	apiServerURL string

	// original values
	caBundle                                    *string
	clusterCASecretName                         string
	clusterCABundle                             []byte
	clusterDNSAddresses                         []string
	clusterDomain                               string
	criName                                     extensionsv1alpha1.CRIName
	images                                      map[string]*imagevectorutils.Image
	kubeletCABundle                             []byte
	kubeletConfig                               *gardencorev1beta1.KubeletConfig
	kubeletConfigParameters                     components.ConfigurableKubeletConfigParameters
	kubeletCLIFlags                             components.ConfigurableKubeletCLIFlags
	kubeletDataVolumeName                       *string
	kubeProxyEnabled                            bool
	kubernetesVersion                           *semver.Version
	sshPublicKeys                               []string
	sshAccessEnabled                            bool
	valiIngressHostName                         string
	valitailEnabled                             bool
	nodeLocalDNSEnabled                         bool
	nodeMonitorGracePeriod                      metav1.Duration
	primaryIPFamily                             gardencorev1beta1.IPFamily
	taints                                      []corev1.Taint
	caRotationLastInitiationTime                *metav1.Time
	serviceAccountKeyRotationLastInitiationTime *metav1.Time
}

// exposed for testing
var (
	// InitConfigFn is a function for computing the gardener-node-init units and files.
	InitConfigFn = nodeinit.Config
	// OriginalConfigFn is a function for computing the downloaded cloud config user data units and files.
	OriginalConfigFn = original.Config
)

func (d *deployer) deploy(ctx context.Context, operation string) (extensionsv1alpha1.Object, error) {
	var (
		units []extensionsv1alpha1.Unit
		files []extensionsv1alpha1.File
		err   error
	)

	componentsContext := components.Context{
		Key:                     d.key,
		CABundle:                d.caBundle,
		ClusterDNSAddresses:     d.clusterDNSAddresses,
		ClusterDomain:           d.clusterDomain,
		CRIName:                 d.criName,
		Images:                  d.images,
		NodeLabels:              gardenerutils.NodeLabelsForWorkerPool(d.worker, d.nodeLocalDNSEnabled, d.key),
		NodeMonitorGracePeriod:  d.nodeMonitorGracePeriod,
		KubeletCABundle:         d.kubeletCABundle,
		KubeletConfigParameters: d.kubeletConfigParameters,
		KubeletCLIFlags:         d.kubeletCLIFlags,
		KubeletDataVolumeName:   d.kubeletDataVolumeName,
		KubeProxyEnabled:        d.kubeProxyEnabled,
		KubernetesVersion:       d.kubernetesVersion,
		SSHPublicKeys:           d.sshPublicKeys,
		SSHAccessEnabled:        d.sshAccessEnabled,
		ValitailEnabled:         d.valitailEnabled,
		ValiIngress:             d.valiIngressHostName,
		APIServerURL:            d.apiServerURL,
		Sysctls:                 d.worker.Sysctls,
		PreferIPv6:              d.primaryIPFamily == gardencorev1beta1.IPFamilyIPv6,
		Taints:                  d.taints,
	}

	switch d.purpose {
	case extensionsv1alpha1.OperatingSystemConfigPurposeProvision:
		units, files, err = InitConfigFn(
			d.worker,
			d.images[imagevector.ContainerImageNameGardenerNodeAgent].String(),
			nodeagent.ComponentConfig(d.key, d.kubernetesVersion, d.apiServerURL, d.clusterCABundle, nil),
		)
		if err != nil {
			return nil, err
		}

		// Add gardener-user and sshd-ensurer when SSH access for the node is enabled
		if d.sshAccessEnabled {
			for _, c := range []components.Component{gardeneruser.New(), sshdensurer.New()} {
				cUnits, cFiles, err := c.Config(componentsContext)
				if err != nil {
					return nil, err
				}
				units = append(units, cUnits...)
				files = append(files, cFiles...)
			}
		}

	case extensionsv1alpha1.OperatingSystemConfigPurposeReconcile:
		units, files, err = OriginalConfigFn(componentsContext)
		if err != nil {
			return nil, err
		}

	default:
		return nil, fmt.Errorf("unknown purpose: %q", d.purpose)
	}

	// We operate on arrays (units, files) with merge patch without optimistic locking here, meaning this will replace
	// the arrays as a whole.
	// However, this is not a problem, as no other client should write to these arrays as the OSC spec is supposed
	// to be owned by gardenlet exclusively.
	_, err = controllerutils.GetAndCreateOrMergePatch(ctx, d.client, d.osc, func() error {
		metav1.SetMetaDataAnnotation(&d.osc.ObjectMeta, v1beta1constants.GardenerOperation, operation)
		metav1.SetMetaDataAnnotation(&d.osc.ObjectMeta, v1beta1constants.GardenerTimestamp, TimeNow().UTC().Format(time.RFC3339Nano))
		metav1.SetMetaDataLabel(&d.osc.ObjectMeta, v1beta1constants.LabelWorkerPool, d.worker.Name)
		metav1.SetMetaDataLabel(&d.osc.ObjectMeta, v1beta1constants.LabelExtensionProviderMutatedByControlplaneWebhook, "true")

		if d.worker.Machine.Image != nil {
			d.osc.Spec.Type = d.worker.Machine.Image.Name
			d.osc.Spec.ProviderConfig = d.worker.Machine.Image.ProviderConfig
		}
		d.osc.Spec.Purpose = d.purpose
		d.osc.Spec.Units = units
		d.osc.Spec.Files = files

		if v1beta1helper.IsUpdateStrategyInPlace(d.worker.UpdateStrategy) && d.purpose == extensionsv1alpha1.OperatingSystemConfigPurposeReconcile {
			d.osc.Spec.InPlaceUpdates = &extensionsv1alpha1.InPlaceUpdates{
				KubeletVersion: d.kubernetesVersion.String(),
			}

			if d.worker.Machine.Image != nil {
				d.osc.Spec.InPlaceUpdates.OperatingSystemVersion = ptr.Deref(d.worker.Machine.Image.Version, "")
			}

			if d.caRotationLastInitiationTime != nil || d.serviceAccountKeyRotationLastInitiationTime != nil {
				d.osc.Spec.InPlaceUpdates.CredentialsRotation = &extensionsv1alpha1.CredentialsRotation{
					CertificateAuthorities: &extensionsv1alpha1.CARotation{
						LastInitiationTime: d.caRotationLastInitiationTime,
					},
					ServiceAccountKey: &extensionsv1alpha1.ServiceAccountKeyRotation{
						LastInitiationTime: d.serviceAccountKeyRotationLastInitiationTime,
					},
				}
			}
		}

		if d.worker.CRI != nil {
			d.osc.Spec.CRIConfig = &extensionsv1alpha1.CRIConfig{
				Name: extensionsv1alpha1.CRIName(d.worker.CRI.Name),
			}
		}

		if d.osc.Spec.CRIConfig != nil &&
			d.osc.Spec.CRIConfig.Name == extensionsv1alpha1.CRINameContainerD &&
			d.purpose == extensionsv1alpha1.OperatingSystemConfigPurposeReconcile {
			d.osc.Spec.CRIConfig.Containerd = &extensionsv1alpha1.ContainerdConfig{}

			if pauseImage := d.images[imagevector.ContainerImageNamePauseContainer]; pauseImage != nil {
				d.osc.Spec.CRIConfig.Containerd.SandboxImage = pauseImage.String()
			}

			if version.ConstraintK8sGreaterEqual131.Check(d.kubernetesVersion) {
				d.osc.Spec.CRIConfig.CgroupDriver = ptr.To(extensionsv1alpha1.CgroupDriverSystemd)
			}
		}

		return nil
	})
	return d.osc, err
}

func (o *operatingSystemConfig) calculateKeyForVersion(ctx context.Context, version int, worker *gardencorev1beta1.Worker) (string, error) {
	if !v1beta1helper.IsUpdateStrategyInPlace(worker.UpdateStrategy) {
		return o.calculateDefaultKey(version, worker)
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: o.values.Namespace, Name: WorkerPoolHashesSecretName},
	}

	if err := o.client.Get(ctx, client.ObjectKeyFromObject(secret), secret); err != nil {
		if !apierrors.IsNotFound(err) {
			return "", fmt.Errorf("failed to get secret %q: %w", secret.Name, err)
		}

		return o.calculateDefaultKey(version, worker)
	}

	workerPoolNameToHashEntry, err := decodePoolHashes(secret)
	if err != nil {
		return "", fmt.Errorf("failed to decode pool hashes: %w", err)
	}

	if workerHash, ok := workerPoolNameToHashEntry[worker.Name]; ok {
		if hash, ok := workerHash.HashVersionToOSCKey[version]; ok {
			return hash, nil
		}

		return "", fmt.Errorf("no hash available for version %d for worker pool %q", version, worker.Name)
	}

	return o.calculateDefaultKey(version, worker)
}

func (o *operatingSystemConfig) calculateDefaultKey(version int, worker *gardencorev1beta1.Worker) (string, error) {
	kubernetesVersion, err := v1beta1helper.CalculateEffectiveKubernetesVersion(o.values.KubernetesVersion, worker.Kubernetes)
	if err != nil {
		return "", err
	}
	kubeletConfiguration := v1beta1helper.CalculateEffectiveKubeletConfiguration(o.values.KubeletConfig, worker.Kubernetes)

	return CalculateKeyForVersion(version, kubernetesVersion, o.values, worker, kubeletConfiguration)
}

// CalculateKeyForVersion is exposed for testing purposes only
var CalculateKeyForVersion = calculateKeyForVersion

func calculateKeyForVersion(
	version int,
	kubernetesVersion *semver.Version,
	values *Values,
	worker *gardencorev1beta1.Worker,
	kubeletConfiguration *gardencorev1beta1.KubeletConfig,
) (
	string,
	error,
) {
	switch version {
	case 1:
		// TODO(MichaelEischer): Remove KeyV1 after support for Kubernetes 1.30 is dropped
		return KeyV1(worker.Name, v1beta1helper.IsUpdateStrategyInPlace(worker.UpdateStrategy), kubernetesVersion, worker.CRI), nil
	case 2:
		return KeyV2(kubernetesVersion, values.CredentialsRotationStatus, worker, values.NodeLocalDNSEnabled, kubeletConfiguration), nil
	default:
		return "", fmt.Errorf("unsupported osc key hash version %v", version)
	}
}

// KeyV1 returns the key that can be used as secret name based on the provided worker name, Kubernetes version and CRI
// configuration.
func KeyV1(workerPoolName string, inPlaceUpdate bool, kubernetesVersion *semver.Version, criConfig *gardencorev1beta1.CRI) string {
	if kubernetesVersion == nil {
		return ""
	}

	var (
		kubernetesMajorMinorVersion string
		criName                     gardencorev1beta1.CRIName
	)

	if !inPlaceUpdate {
		kubernetesMajorMinorVersion = fmt.Sprintf("%d.%d", kubernetesVersion.Major(), kubernetesVersion.Minor())
	}

	if criConfig != nil {
		criName = criConfig.Name
	}

	return fmt.Sprintf("gardener-node-agent-%s-%s", workerPoolName, utils.ComputeSHA256Hex([]byte(kubernetesMajorMinorVersion + string(criName)))[:5])
}

// KeyV2 returns the key that can be used as secret name based on the provided worker name,
// Kubernetes version, machine type, image, worker volume, CRI, credentials rotation, node local dns
// and kubelet configuration.
func KeyV2(
	kubernetesVersion *semver.Version,
	credentialsRotation *gardencorev1beta1.ShootCredentialsRotation,
	worker *gardencorev1beta1.Worker,
	nodeLocalDNSEnabled bool,
	kubeletConfiguration *gardencorev1beta1.KubeletConfig,
) string {
	if kubernetesVersion == nil {
		return ""
	}

	var (
		inPlaceUpdate               = v1beta1helper.IsUpdateStrategyInPlace(worker.UpdateStrategy)
		kubernetesMajorMinorVersion = fmt.Sprintf("%d.%d", kubernetesVersion.Major(), kubernetesVersion.Minor())
		data                        = []string{kubernetesMajorMinorVersion, worker.Machine.Type}
	)

	if worker.Machine.Image != nil {
		data = append(data, worker.Machine.Image.Name+*worker.Machine.Image.Version)
	}

	if inPlaceUpdate {
		data = []string{worker.Machine.Type}

		if worker.Machine.Image != nil {
			data = append(data, worker.Machine.Image.Name)
		}
	}

	if worker.Volume != nil {
		data = append(data, worker.Volume.VolumeSize)
		if worker.Volume.Type != nil {
			data = append(data, *worker.Volume.Type)
		}
	}

	if worker.CRI != nil {
		data = append(data, string(worker.CRI.Name))
	}

	if !inPlaceUpdate && credentialsRotation != nil {
		if credentialsRotation.CertificateAuthorities != nil {
			if lastInitiationTime := v1beta1helper.LastInitiationTimeForWorkerPool(worker.Name, credentialsRotation.CertificateAuthorities.PendingWorkersRollouts, credentialsRotation.CertificateAuthorities.LastInitiationTime); lastInitiationTime != nil {
				data = append(data, lastInitiationTime.String())
			}
		}
		if credentialsRotation.ServiceAccountKey != nil {
			if lastInitiationTime := v1beta1helper.LastInitiationTimeForWorkerPool(worker.Name, credentialsRotation.ServiceAccountKey.PendingWorkersRollouts, credentialsRotation.ServiceAccountKey.LastInitiationTime); lastInitiationTime != nil {
				data = append(data, lastInitiationTime.String())
			}
		}
	}

	if nodeLocalDNSEnabled {
		data = append(data, "node-local-dns")
	}

	if !inPlaceUpdate {
		data = append(data, gardenerutils.CalculateDataStringForKubeletConfiguration(kubeletConfiguration)...)
	}

	var result string
	for _, v := range data {
		result += utils.ComputeSHA256Hex([]byte(v))
	}

	return fmt.Sprintf("gardener-node-agent-%s-%s", worker.Name, utils.ComputeSHA256Hex([]byte(result))[:16])
}

func keySuffix(version int, machineImage *gardencorev1beta1.ShootMachineImage, purpose extensionsv1alpha1.OperatingSystemConfigPurpose) string {
	var imagePrefix string
	if version == 1 && machineImage != nil {
		imagePrefix = "-" + machineImage.Name
	}

	switch purpose {
	case extensionsv1alpha1.OperatingSystemConfigPurposeProvision:
		return imagePrefix + "-init"
	case extensionsv1alpha1.OperatingSystemConfigPurposeReconcile:
		return imagePrefix + "-original"
	}
	return ""
}
