// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package operatingsystemconfig

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
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
	nodeagentv1alpha1 "github.com/gardener/gardener/pkg/nodeagent/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/flow"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	imagevectorutils "github.com/gardener/gardener/pkg/utils/imagevector"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
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
)

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
	// SetSSHPublicKeys sets the SSHPublicKeys value.
	SetSSHPublicKeys([]string)
	// WorkerNameToOperatingSystemConfigsMap returns a map whose key is a worker name and whose value is a structure
	// containing both the init and the original operating system config data.
	WorkerNameToOperatingSystemConfigsMap() map[string]*OperatingSystemConfigs
}

// Values contains the values used to create an OperatingSystemConfig resource.
type Values struct {
	// Namespace is the namespace for the OperatingSystemConfig resource.
	Namespace string
	// KubernetesVersion is the version for the kubelets of all worker pools.
	KubernetesVersion *semver.Version
	// Workers is the list of worker pools.
	Workers []gardencorev1beta1.Worker

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
	// ClusterDNSAddress is the address for in-cluster DNS.
	ClusterDNSAddress string
	// ClusterDomain is the Kubernetes cluster domain.
	ClusterDomain string
	// Images is a map containing the necessary container images for the systemd units (hyperkube and pause-container).
	Images map[string]*imagevectorutils.Image
	// KubeletConfig is the default kubelet configuration for all worker pools. Individual worker pools might overwrite
	// this configuration.
	KubeletConfig *gardencorev1beta1.KubeletConfig
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

	osc.workerNameToOSCs = make(map[string]*OperatingSystemConfigs, len(values.Workers))
	for _, worker := range values.Workers {
		osc.workerNameToOSCs[worker.Name] = &OperatingSystemConfigs{}
	}
	osc.oscs = make(map[string]*extensionsv1alpha1.OperatingSystemConfig, len(osc.workerNameToOSCs)*2)

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

	lock             sync.Mutex
	workerNameToOSCs map[string]*OperatingSystemConfigs
	oscs             map[string]*extensionsv1alpha1.OperatingSystemConfig
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
	// Content is the actual cloud-config user data.
	// TODO(rfranzke): Remove this Content field after v1.100 is released.
	Content string
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
	if err := gardenerutils.
		NewShootAccessSecret(nodeagentv1alpha1.AccessSecretName, o.values.Namespace).
		WithTargetSecret(nodeagentv1alpha1.AccessSecretName, metav1.NamespaceSystem).
		WithTokenExpirationDuration("720h").
		Reconcile(ctx, o.client); err != nil {
		return err
	}

	fns := o.forEachWorkerPoolAndPurposeTaskFn(func(_ context.Context, osc *extensionsv1alpha1.OperatingSystemConfig, worker gardencorev1beta1.Worker, purpose extensionsv1alpha1.OperatingSystemConfigPurpose) error {
		d, err := o.newDeployer(osc, worker, purpose)
		if err != nil {
			return err
		}

		return reconcileFn(d)
	})

	return flow.Parallel(fns...)(ctx)
}

// Wait waits until the OperatingSystemConfig CRD is ready (deployed or restored). It also reads the produced secret
// containing the cloud-config and stores its data which can later be retrieved with the WorkerNameToOperatingSystemConfigsMap
// method.
func (o *operatingSystemConfig) Wait(ctx context.Context) error {
	fns := o.forEachWorkerPoolAndPurposeTaskFn(func(ctx context.Context, osc *extensionsv1alpha1.OperatingSystemConfig, worker gardencorev1beta1.Worker, purpose extensionsv1alpha1.OperatingSystemConfigPurpose) error {
		return extensions.WaitUntilExtensionObjectReady(ctx,
			o.client,
			o.log,
			osc,
			extensionsv1alpha1.OperatingSystemConfigResource,
			o.waitInterval,
			o.waitSevereThreshold,
			o.waitTimeout,
			func() error {
				if osc.Status.CloudConfig == nil {
					return fmt.Errorf("no cloud config information provided in status")
				}

				secret := &corev1.Secret{}
				if err := o.client.Get(ctx, client.ObjectKey{Namespace: osc.Status.CloudConfig.SecretRef.Namespace, Name: osc.Status.CloudConfig.SecretRef.Name}, secret); err != nil {
					return err
				}

				kubernetesVersion, err := v1beta1helper.CalculateEffectiveKubernetesVersion(o.values.KubernetesVersion, worker.Kubernetes)
				if err != nil {
					return err
				}
				oscKey := Key(worker.Name, kubernetesVersion, worker.CRI)

				data := Data{
					Object:                      osc,
					Content:                     string(secret.Data[extensionsv1alpha1.OperatingSystemConfigSecretDataKey]),
					GardenerNodeAgentSecretName: oscKey,
					SecretName:                  &secret.Name,
				}

				o.lock.Lock()
				defer o.lock.Unlock()

				switch purpose {
				case extensionsv1alpha1.OperatingSystemConfigPurposeProvision:
					o.workerNameToOSCs[worker.Name].Init = data
				case extensionsv1alpha1.OperatingSystemConfigPurposeReconcile:
					o.workerNameToOSCs[worker.Name].Original = data
				default:
					return fmt.Errorf("unknown purpose %q", purpose)
				}

				return nil
			},
		)
	})

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
	return o.deleteOperatingSystemConfigResources(ctx, sets.New[string]())
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
	wantedOSCs, err := o.getWantedOSCNames()
	if err != nil {
		return err
	}
	return o.deleteOperatingSystemConfigResources(ctx, wantedOSCs)
}

// WaitCleanupStaleResources waits until all unused OperatingSystemConfig resources are cleaned up.
func (o *operatingSystemConfig) WaitCleanupStaleResources(ctx context.Context) error {
	wantedOSCs, err := o.getWantedOSCNames()
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
func (o *operatingSystemConfig) getWantedOSCNames() (sets.Set[string], error) {
	wantedOSCNames := sets.New[string]()

	for _, worker := range o.values.Workers {
		if worker.Machine.Image == nil {
			continue
		}

		for _, purpose := range []extensionsv1alpha1.OperatingSystemConfigPurpose{
			extensionsv1alpha1.OperatingSystemConfigPurposeProvision,
			extensionsv1alpha1.OperatingSystemConfigPurposeReconcile,
		} {
			kubernetesVersion, err := v1beta1helper.CalculateEffectiveKubernetesVersion(o.values.KubernetesVersion, worker.Kubernetes)
			if err != nil {
				return nil, err
			}
			wantedOSCNames.Insert(Key(worker.Name, kubernetesVersion, worker.CRI) + keySuffix(worker.Machine.Image.Name, purpose))
		}
	}

	return wantedOSCNames, nil
}

func (o *operatingSystemConfig) forEachWorkerPoolAndPurpose(fn func(*extensionsv1alpha1.OperatingSystemConfig, gardencorev1beta1.Worker, extensionsv1alpha1.OperatingSystemConfigPurpose) error) error {
	for _, worker := range o.values.Workers {
		if worker.Machine.Image == nil {
			continue
		}

		for _, purpose := range []extensionsv1alpha1.OperatingSystemConfigPurpose{
			extensionsv1alpha1.OperatingSystemConfigPurposeProvision,
			extensionsv1alpha1.OperatingSystemConfigPurposeReconcile,
		} {
			kubernetesVersion, err := v1beta1helper.CalculateEffectiveKubernetesVersion(o.values.KubernetesVersion, worker.Kubernetes)
			if err != nil {
				return err
			}
			oscName := Key(worker.Name, kubernetesVersion, worker.CRI) + keySuffix(worker.Machine.Image.Name, purpose)

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

			if err := fn(osc, worker, purpose); err != nil {
				return err
			}
		}
	}

	return nil
}

func (o *operatingSystemConfig) forEachWorkerPoolAndPurposeTaskFn(fn func(context.Context, *extensionsv1alpha1.OperatingSystemConfig, gardencorev1beta1.Worker, extensionsv1alpha1.OperatingSystemConfigPurpose) error) []flow.TaskFn {
	var fns []flow.TaskFn

	_ = o.forEachWorkerPoolAndPurpose(func(osc *extensionsv1alpha1.OperatingSystemConfig, worker gardencorev1beta1.Worker, purpose extensionsv1alpha1.OperatingSystemConfigPurpose) error {
		fns = append(fns, func(ctx context.Context) error {
			return fn(ctx, osc, worker, purpose)
		})
		return nil
	})

	return fns
}

// SetAPIServerURL sets the APIServerURL value.
func (o *operatingSystemConfig) SetAPIServerURL(apiServerURL string) {
	o.values.APIServerURL = apiServerURL
}

// SetCABundle sets the CABundle value.
func (o *operatingSystemConfig) SetCABundle(val *string) {
	o.values.CABundle = val
}

// SetSSHPublicKeys sets the SSHPublicKeys value.
func (o *operatingSystemConfig) SetSSHPublicKeys(keys []string) {
	o.values.SSHPublicKeys = keys
}

// WorkerNameToOperatingSystemConfigsMap returns a map whose key is a worker name and whose value is a structure
// containing both the init script and the original config.
func (o *operatingSystemConfig) WorkerNameToOperatingSystemConfigsMap() map[string]*OperatingSystemConfigs {
	return o.workerNameToOSCs
}

func (o *operatingSystemConfig) newDeployer(osc *extensionsv1alpha1.OperatingSystemConfig, worker gardencorev1beta1.Worker, purpose extensionsv1alpha1.OperatingSystemConfigPurpose) (deployer, error) {
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
	setDefaultEvictionMemoryAvailable(kubeletConfigParameters.EvictionHard, kubeletConfigParameters.EvictionSoft, o.values.MachineTypes, worker.Machine.Type)

	kubernetesVersion, err := v1beta1helper.CalculateEffectiveKubernetesVersion(o.values.KubernetesVersion, worker.Kubernetes)
	if err != nil {
		return deployer{}, err
	}

	images := make(map[string]*imagevectorutils.Image, len(o.values.Images))
	for imageName, image := range o.values.Images {
		images[imageName] = image
	}

	images[imagevector.ImageNameHyperkube], err = imagevector.ImageVector().FindImage(imagevector.ImageNameHyperkube, imagevectorutils.RuntimeVersion(kubernetesVersion.String()), imagevectorutils.TargetVersion(kubernetesVersion.String()))
	if err != nil {
		return deployer{}, fmt.Errorf("failed finding hyperkube image for version %s: %w", kubernetesVersion.String(), err)
	}

	return deployer{
		client:                  o.client,
		osc:                     osc,
		worker:                  worker,
		purpose:                 purpose,
		key:                     Key(worker.Name, kubernetesVersion, worker.CRI),
		apiServerURL:            o.values.APIServerURL,
		caBundle:                caBundle,
		clusterCASecretName:     clusterCASecret.Name,
		clusterCABundle:         clusterCASecret.Data[secretsutils.DataKeyCertificateBundle],
		clusterDNSAddress:       o.values.ClusterDNSAddress,
		clusterDomain:           o.values.ClusterDomain,
		criName:                 criName,
		images:                  images,
		kubeletCABundle:         kubeletCASecret.Data[secretsutils.DataKeyCertificateBundle],
		kubeletConfigParameters: kubeletConfigParameters,
		kubeletCLIFlags:         kubeletCLIFlags,
		kubeletDataVolumeName:   worker.KubeletDataVolumeName,
		kubernetesVersion:       kubernetesVersion,
		sshPublicKeys:           o.values.SSHPublicKeys,
		sshAccessEnabled:        o.values.SSHAccessEnabled,
		valiIngressHostName:     o.values.ValiIngressHostName,
		valitailEnabled:         o.values.ValitailEnabled,
		nodeLocalDNSEnabled:     o.values.NodeLocalDNSEnabled,
		primaryIPFamily:         o.values.PrimaryIPFamily,
		taints:                  worker.Taints,
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
	caBundle                *string
	clusterCASecretName     string
	clusterCABundle         []byte
	clusterDNSAddress       string
	clusterDomain           string
	criName                 extensionsv1alpha1.CRIName
	images                  map[string]*imagevectorutils.Image
	kubeletCABundle         []byte
	kubeletConfigParameters components.ConfigurableKubeletConfigParameters
	kubeletCLIFlags         components.ConfigurableKubeletCLIFlags
	kubeletDataVolumeName   *string
	kubernetesVersion       *semver.Version
	sshPublicKeys           []string
	sshAccessEnabled        bool
	valiIngressHostName     string
	valitailEnabled         bool
	nodeLocalDNSEnabled     bool
	primaryIPFamily         gardencorev1beta1.IPFamily
	taints                  []corev1.Taint
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
		ClusterDNSAddress:       d.clusterDNSAddress,
		ClusterDomain:           d.clusterDomain,
		CRIName:                 d.criName,
		Images:                  d.images,
		NodeLabels:              gardenerutils.NodeLabelsForWorkerPool(d.worker, d.nodeLocalDNSEnabled, d.key),
		KubeletCABundle:         d.kubeletCABundle,
		KubeletConfigParameters: d.kubeletConfigParameters,
		KubeletCLIFlags:         d.kubeletCLIFlags,
		KubeletDataVolumeName:   d.kubeletDataVolumeName,
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
			d.images[imagevector.ImageNameGardenerNodeAgent].String(),
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

		d.osc.Spec.Type = d.worker.Machine.Image.Name
		d.osc.Spec.ProviderConfig = d.worker.Machine.Image.ProviderConfig
		d.osc.Spec.Purpose = d.purpose
		d.osc.Spec.Units = units
		d.osc.Spec.Files = files

		if d.worker.CRI != nil {
			d.osc.Spec.CRIConfig = &extensionsv1alpha1.CRIConfig{
				Name: extensionsv1alpha1.CRIName(d.worker.CRI.Name),
			}
		}

		return nil
	})
	return d.osc, err
}

// Key returns the key that can be used as secret name based on the provided worker name, Kubernetes version and CRI
// configuration.
func Key(workerName string, kubernetesVersion *semver.Version, criConfig *gardencorev1beta1.CRI) string {
	if kubernetesVersion == nil {
		return ""
	}

	var (
		kubernetesMajorMinorVersion = fmt.Sprintf("%d.%d", kubernetesVersion.Major(), kubernetesVersion.Minor())
		criName                     gardencorev1beta1.CRIName
	)

	if criConfig != nil {
		criName = criConfig.Name
	}

	return fmt.Sprintf("gardener-node-agent-%s-%s", workerName, utils.ComputeSHA256Hex([]byte(kubernetesMajorMinorVersion + string(criName)))[:5])
}

func keySuffix(machineImageName string, purpose extensionsv1alpha1.OperatingSystemConfigPurpose) string {
	switch purpose {
	case extensionsv1alpha1.OperatingSystemConfigPurposeProvision:
		return "-" + machineImageName + "-init"
	case extensionsv1alpha1.OperatingSystemConfigPurposeReconcile:
		return "-" + machineImageName + "-original"
	}
	return ""
}
