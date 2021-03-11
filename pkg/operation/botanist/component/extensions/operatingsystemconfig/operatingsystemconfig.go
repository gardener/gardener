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

package operatingsystemconfig

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/operation/botanist/component/extensions/operatingsystemconfig/downloader"
	"github.com/gardener/gardener/pkg/operation/botanist/component/extensions/operatingsystemconfig/original"
	"github.com/gardener/gardener/pkg/operation/botanist/component/extensions/operatingsystemconfig/original/components"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/flow"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	"github.com/Masterminds/semver"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
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
	// DeleteStaleResources deletes stale OperatingSystemConfig resources.
	DeleteStaleResources(context.Context) error
	// SetCABundle sets the CABundle value.
	SetCABundle(*string)
	// SetKubeletCACertificate sets the KubeletCACertificate value.
	SetKubeletCACertificate(string)
	// SetSSHPublicKey sets the SSHPublicKey value.
	SetSSHPublicKey(string)
	// WorkerNameToOperatingSystemConfigsMap returns a map whose key is a worker name and whose value is a structure
	// containing both the downloader as well as the original operating system config data.
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

	// DownloaderValues are configuration values required for the 'provision' OperatingSystemConfigPurpose.
	DownloaderValues
	// OriginalValues are configuration values required for the 'reconcile' OperatingSystemConfigPurpose.
	OriginalValues
}

// DownloaderValues are configuration values required for the 'provision' OperatingSystemConfigPurpose.
type DownloaderValues struct {
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
	Images map[string]*imagevector.Image
	// KubeletCACertificate is the certificate authority for the kubelet.
	KubeletCACertificate string
	// KubeletCLIFlags is the set of configurable kubelet CLI flags. If the respective worker pool provides kubelet
	// configuration as well then this might get overwritten.
	KubeletCLIFlags components.ConfigurableKubeletCLIFlags
	// KubeletConfigParameters is the set of configurable kubelet config parameters. If the respective worker pool
	// provides kubelet configuration as well then this might get overwritten.
	KubeletConfigParameters components.ConfigurableKubeletConfigParameters
	// MachineTypes is a list of machine types.
	MachineTypes []gardencorev1beta1.MachineType
	// SSHPublicKey is a public SSH key.
	SSHPublicKey string
}

// New creates a new instance of Interface.
func New(
	logger logrus.FieldLogger,
	client client.Client,
	values *Values,
	waitInterval time.Duration,
	waitSevereThreshold time.Duration,
	waitTimeout time.Duration,
) Interface {
	osc := &operatingSystemConfig{
		client:              client,
		logger:              logger,
		values:              values,
		waitInterval:        waitInterval,
		waitSevereThreshold: waitSevereThreshold,
		waitTimeout:         waitTimeout,
	}

	osc.workerNameToOSCs = make(map[string]*OperatingSystemConfigs, len(values.Workers))
	for _, worker := range values.Workers {
		osc.workerNameToOSCs[worker.Name] = &OperatingSystemConfigs{}
	}

	return osc
}

type operatingSystemConfig struct {
	values              *Values
	logger              logrus.FieldLogger
	client              client.Client
	waitInterval        time.Duration
	waitSevereThreshold time.Duration
	waitTimeout         time.Duration

	lock             sync.Mutex
	workerNameToOSCs map[string]*OperatingSystemConfigs
}

// OperatingSystemConfigs contains operating system configs for the downloader script as well as for the original cloud
// config.
type OperatingSystemConfigs struct {
	// Downloader is the data for the downloader script.
	Downloader Data
	// Original is the data for the to-be-downloaded cloud-config user data.
	Original Data
}

// Data contains the actual content, a command to load it and all units that shall be considered for restart on change.
type Data struct {
	// Content is the actual cloud-config user data.
	Content string
	// Command is the command for reloading the cloud-config (in case a new version was downloaded).
	Command *string
	// Units is the list of systemd unit names.
	Units []string
}

// Deploy uses the client to create or update the OperatingSystemConfig custom resources.
func (o *operatingSystemConfig) Deploy(ctx context.Context) error {
	fns := o.forEachWorkerPoolAndPurposeTaskFn(func(ctx context.Context, worker gardencorev1beta1.Worker, purpose extensionsv1alpha1.OperatingSystemConfigPurpose) error {
		d := o.newDeployer(worker, purpose)
		_, err := d.deploy(ctx, v1beta1constants.GardenerOperationReconcile)
		return err
	})

	return flow.Parallel(fns...)(ctx)
}

// Restore uses the seed client and the ShootState to create the OperatingSystemConfig custom resources in the Shoot
// namespace in the Seed and restore its state.
func (o *operatingSystemConfig) Restore(ctx context.Context, shootState *v1alpha1.ShootState) error {
	fns := o.forEachWorkerPoolAndPurposeTaskFn(func(ctx context.Context, worker gardencorev1beta1.Worker, purpose extensionsv1alpha1.OperatingSystemConfigPurpose) error {
		d := o.newDeployer(worker, purpose)
		return common.RestoreExtensionWithDeployFunction(ctx, o.client, shootState, extensionsv1alpha1.OperatingSystemConfigResource, o.values.Namespace, d.deploy)
	})

	return flow.Parallel(fns...)(ctx)
}

// Wait waits until the OperatingSystemConfig CRD is ready (deployed or restored). It also reads the produced secret
// containing the cloud-config and stores its data which can later be retrieved with the WorkerNameToOperatingSystemConfigsMap
// method.
func (o *operatingSystemConfig) Wait(ctx context.Context) error {
	fns := o.forEachWorkerPoolAndPurposeTaskFn(func(ctx context.Context, worker gardencorev1beta1.Worker, purpose extensionsv1alpha1.OperatingSystemConfigPurpose) error {
		return common.WaitUntilExtensionCRReady(
			ctx,
			o.client,
			o.logger,
			func() client.Object { return &extensionsv1alpha1.OperatingSystemConfig{} },
			extensionsv1alpha1.OperatingSystemConfigResource,
			o.values.Namespace,
			Key(worker.Name, o.values.KubernetesVersion)+purposeToKeySuffix(purpose),
			o.waitInterval,
			o.waitSevereThreshold,
			o.waitTimeout,
			func(obj client.Object) error {
				osc, ok := obj.(*extensionsv1alpha1.OperatingSystemConfig)
				if !ok {
					return fmt.Errorf("expected extensionsv1alpha1.OperatingSystemConfig but got %T", obj)
				}
				if osc.Status.CloudConfig == nil {
					return fmt.Errorf("no cloud config information provided in status")
				}

				secret := &corev1.Secret{}
				if err := o.client.Get(ctx, kutil.Key(osc.Status.CloudConfig.SecretRef.Namespace, osc.Status.CloudConfig.SecretRef.Name), secret); err != nil {
					return err
				}

				data := Data{
					Content: string(secret.Data[extensionsv1alpha1.OperatingSystemConfigSecretDataKey]),
					Command: osc.Status.Command,
					Units:   osc.Status.Units,
				}

				o.lock.Lock()
				defer o.lock.Unlock()

				switch purpose {
				case extensionsv1alpha1.OperatingSystemConfigPurposeProvision:
					o.workerNameToOSCs[worker.Name].Downloader = data
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
	return common.MigrateExtensionCRs(
		ctx,
		o.client,
		&extensionsv1alpha1.OperatingSystemConfigList{},
		func() extensionsv1alpha1.Object { return &extensionsv1alpha1.OperatingSystemConfig{} },
		o.values.Namespace,
	)
}

// WaitMigrate waits until the OperatingSystemConfig custom resource have been successfully migrated.
func (o *operatingSystemConfig) WaitMigrate(ctx context.Context) error {
	return common.WaitUntilExtensionCRsMigrated(
		ctx,
		o.client,
		&extensionsv1alpha1.OperatingSystemConfigList{},
		func() extensionsv1alpha1.Object { return &extensionsv1alpha1.OperatingSystemConfig{} },
		o.values.Namespace,
		o.waitInterval,
		o.waitTimeout,
	)
}

// Destroy deletes all the OperatingSystemConfig resources.
func (o *operatingSystemConfig) Destroy(ctx context.Context) error {
	return o.deleteOperatingSystemConfigResources(ctx, sets.NewString())
}

func (o *operatingSystemConfig) deleteOperatingSystemConfigResources(ctx context.Context, wantedOSCNames sets.String) error {
	return common.DeleteExtensionCRs(
		ctx,
		o.client,
		&extensionsv1alpha1.OperatingSystemConfigList{},
		func() extensionsv1alpha1.Object { return &extensionsv1alpha1.OperatingSystemConfig{} },
		o.values.Namespace,
		func(obj extensionsv1alpha1.Object) bool {
			return !wantedOSCNames.Has(obj.GetName())
		},
	)
}

// WaitCleanup waits until the OperatingSystemConfig CRD is deleted
func (o *operatingSystemConfig) WaitCleanup(ctx context.Context) error {
	return common.WaitUntilExtensionCRsDeleted(
		ctx,
		o.client,
		o.logger,
		&extensionsv1alpha1.OperatingSystemConfigList{},
		func() extensionsv1alpha1.Object { return &extensionsv1alpha1.OperatingSystemConfig{} },
		extensionsv1alpha1.OperatingSystemConfigResource,
		o.values.Namespace,
		o.waitInterval,
		o.waitTimeout,
		nil,
	)
}

// DeleteStaleResources deletes unused OperatingSystemConfig resources from the shoot namespace in the seed.
func (o *operatingSystemConfig) DeleteStaleResources(ctx context.Context) error {
	wantedOSCNames := sets.NewString()

	_ = o.forEachWorkerPoolAndPurpose(ctx, func(_ context.Context, worker gardencorev1beta1.Worker, purpose extensionsv1alpha1.OperatingSystemConfigPurpose) error {
		wantedOSCNames.Insert(Key(worker.Name, o.values.KubernetesVersion) + purposeToKeySuffix(purpose))
		return nil
	})

	return o.deleteOperatingSystemConfigResources(ctx, wantedOSCNames)
}

func (o *operatingSystemConfig) forEachWorkerPoolAndPurpose(ctx context.Context, fn func(context.Context, gardencorev1beta1.Worker, extensionsv1alpha1.OperatingSystemConfigPurpose) error) error {
	for _, worker := range o.values.Workers {
		if worker.Machine.Image == nil {
			continue
		}

		for _, purpose := range []extensionsv1alpha1.OperatingSystemConfigPurpose{
			extensionsv1alpha1.OperatingSystemConfigPurposeProvision,
			extensionsv1alpha1.OperatingSystemConfigPurposeReconcile,
		} {
			if err := fn(ctx, worker, purpose); err != nil {
				return err
			}
		}
	}

	return nil
}

func (o *operatingSystemConfig) forEachWorkerPoolAndPurposeTaskFn(fn func(context.Context, gardencorev1beta1.Worker, extensionsv1alpha1.OperatingSystemConfigPurpose) error) []flow.TaskFn {
	var fns []flow.TaskFn

	_ = o.forEachWorkerPoolAndPurpose(context.Background(), func(_ context.Context, worker gardencorev1beta1.Worker, purpose extensionsv1alpha1.OperatingSystemConfigPurpose) error {
		fns = append(fns, func(ctx context.Context) error {
			return fn(ctx, worker, purpose)
		})
		return nil
	})

	return fns
}

// AppendCABundle sets the CABundle value.
func (o *operatingSystemConfig) SetCABundle(val *string) {
	o.values.CABundle = val
}

// SetKubeletCACertificate sets the KubeletCACertificate value.
func (o *operatingSystemConfig) SetKubeletCACertificate(cert string) {
	o.values.KubeletCACertificate = cert
}

// SetSSHPublicKey sets the SSHPublicKey value.
func (o *operatingSystemConfig) SetSSHPublicKey(key string) {
	o.values.SSHPublicKey = key
}

// WorkerNameToOperatingSystemConfigsMap returns a map whose key is a worker name and whose value is a structure
// containing both the downloader as well as the original operating system config data.
func (o *operatingSystemConfig) WorkerNameToOperatingSystemConfigsMap() map[string]*OperatingSystemConfigs {
	return o.workerNameToOSCs
}

func (o *operatingSystemConfig) newDeployer(worker gardencorev1beta1.Worker, purpose extensionsv1alpha1.OperatingSystemConfigPurpose) deployer {
	criName := extensionsv1alpha1.CRINameDocker
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

	kubeletConfigParameters := o.values.KubeletConfigParameters
	kubeletCLIFlags := o.values.KubeletCLIFlags
	if worker.Kubernetes != nil && worker.Kubernetes.Kubelet != nil {
		kubeletConfigParameters = components.KubeletConfigParametersFromCoreV1beta1KubeletConfig(worker.Kubernetes.Kubelet)
		kubeletCLIFlags = components.KubeletCLIFlagsFromCoreV1beta1KubeletConfig(worker.Kubernetes.Kubelet)
	}
	setDefaultEvictionMemoryAvailable(kubeletConfigParameters.EvictionHard, kubeletConfigParameters.EvictionSoft, o.values.MachineTypes, worker.Machine.Type)

	return deployer{
		client:                  o.client,
		worker:                  worker,
		purpose:                 purpose,
		key:                     Key(worker.Name, o.values.KubernetesVersion),
		namespace:               o.values.Namespace,
		apiServerURL:            o.values.APIServerURL,
		caBundle:                caBundle,
		clusterDNSAddress:       o.values.ClusterDNSAddress,
		clusterDomain:           o.values.ClusterDomain,
		criName:                 criName,
		images:                  o.values.Images,
		kubeletCACertificate:    o.values.KubeletCACertificate,
		kubeletConfigParameters: kubeletConfigParameters,
		kubeletCLIFlags:         kubeletCLIFlags,
		kubeletDataVolumeName:   worker.KubeletDataVolumeName,
		kubernetesVersion:       o.values.KubernetesVersion,
		sshPublicKey:            o.values.SSHPublicKey,
	}
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
	client    client.Client
	worker    gardencorev1beta1.Worker
	purpose   extensionsv1alpha1.OperatingSystemConfigPurpose
	key       string
	namespace string

	// downloader values
	apiServerURL string

	// original values
	caBundle                *string
	clusterDNSAddress       string
	clusterDomain           string
	criName                 extensionsv1alpha1.CRIName
	images                  map[string]*imagevector.Image
	kubeletCACertificate    string
	kubeletConfigParameters components.ConfigurableKubeletConfigParameters
	kubeletCLIFlags         components.ConfigurableKubeletCLIFlags
	kubeletDataVolumeName   *string
	kubernetesVersion       *semver.Version
	sshPublicKey            string
}

// exposed for testing
var (
	// DownloaderConfigFn is a function for computing the cloud config downloader units and files.
	DownloaderConfigFn = downloader.Config
	// OriginalConfigFn is a function for computing the downloaded cloud config user data units and files.
	OriginalConfigFn = original.Config
)

func (d *deployer) deploy(ctx context.Context, operation string) (extensionsv1alpha1.Object, error) {
	var (
		units []extensionsv1alpha1.Unit
		files []extensionsv1alpha1.File
		err   error

		name = d.key + purposeToKeySuffix(d.purpose)
	)

	// The cloud-config-downloader unit is added regardless of the purpose of the OperatingSystemConfig:
	// If the purpose is 'provision' then it is anyways the only unit that is being installed in this provisioning phase
	// because it will download the original cloud config user data.
	// If the purpose is 'reconcile' then its unit content as well as its configuration (certificates, etc.) is added
	// as well so that it can be updated regularly (otherwise, these resources would only be created once during the
	// initial VM bootstrapping phase and never touched again).
	downloaderUnits, downloaderFiles, err := DownloaderConfigFn(d.key, d.apiServerURL)
	if err != nil {
		return nil, err
	}

	switch d.purpose {
	case extensionsv1alpha1.OperatingSystemConfigPurposeProvision:
		units, files = downloaderUnits, downloaderFiles

	case extensionsv1alpha1.OperatingSystemConfigPurposeReconcile:
		units, files, err = OriginalConfigFn(components.Context{
			CABundle:                d.caBundle,
			ClusterDNSAddress:       d.clusterDNSAddress,
			ClusterDomain:           d.clusterDomain,
			CRIName:                 d.criName,
			Images:                  d.images,
			KubeletCACertificate:    d.kubeletCACertificate,
			KubeletConfigParameters: d.kubeletConfigParameters,
			KubeletCLIFlags:         d.kubeletCLIFlags,
			KubeletDataVolumeName:   d.kubeletDataVolumeName,
			KubernetesVersion:       d.kubernetesVersion,
			SSHPublicKey:            d.sshPublicKey,
		})
		if err != nil {
			return nil, err
		}

		// For backwards-compatibility with the OS extensions, we do not directly add the cloud-config-downloader unit
		// but rather the systemd configuration file.
		// See for more information:
		// - https://github.com/gardener/gardener/pull/3449/
		// - https://github.com/gardener/gardener-extension-os-gardenlinux/pull/24
		var ccdUnitContent *string
		for _, unit := range downloaderUnits {
			if unit.Name == downloader.UnitName {
				ccdUnitContent = unit.Content
				break
			}
		}

		if ccdUnitContent != nil {
			files = append(files, downloaderFiles...)
			files = append(files, extensionsv1alpha1.File{
				Path:        "/etc/systemd/system/" + downloader.UnitName,
				Permissions: pointer.Int32Ptr(0644),
				Content: extensionsv1alpha1.FileContent{
					Inline: &extensionsv1alpha1.FileContentInline{
						Encoding: "b64",
						Data:     utils.EncodeBase64([]byte(*ccdUnitContent)),
					},
				},
			})
		}

	default:
		return nil, fmt.Errorf("unknown purpose: %q", d.purpose)
	}

	osc := &extensionsv1alpha1.OperatingSystemConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: d.namespace,
		},
	}

	_, err = controllerutil.CreateOrUpdate(ctx, d.client, osc, func() error {
		metav1.SetMetaDataAnnotation(&osc.ObjectMeta, v1beta1constants.GardenerOperation, operation)
		metav1.SetMetaDataAnnotation(&osc.ObjectMeta, v1beta1constants.GardenerTimestamp, TimeNow().UTC().String())

		osc.Spec.Type = d.worker.Machine.Image.Name
		osc.Spec.ProviderConfig = d.worker.Machine.Image.ProviderConfig
		osc.Spec.Purpose = d.purpose
		osc.Spec.Units = units
		osc.Spec.Files = files

		if d.worker.CRI != nil {
			osc.Spec.CRIConfig = &extensionsv1alpha1.CRIConfig{
				Name: extensionsv1alpha1.CRIName(d.worker.CRI.Name),
			}
		}

		if d.purpose == extensionsv1alpha1.OperatingSystemConfigPurposeReconcile {
			osc.Spec.ReloadConfigFilePath = pointer.StringPtr(downloader.PathDownloadedCloudConfig)
		}

		return nil
	})
	return osc, err
}

// Key returns the key that can be used as secret name based on the provided worker name and Kubernetes version.
func Key(workerName string, kubernetesVersion *semver.Version) string {
	if kubernetesVersion == nil {
		return ""
	}

	kubernetesMajorMinorVersion := fmt.Sprintf("%d.%d", kubernetesVersion.Major(), kubernetesVersion.Minor())
	return fmt.Sprintf("cloud-config-%s-%s", workerName, utils.ComputeSHA256Hex([]byte(kubernetesMajorMinorVersion))[:5])
}

func purposeToKeySuffix(purpose extensionsv1alpha1.OperatingSystemConfigPurpose) string {
	switch purpose {
	case extensionsv1alpha1.OperatingSystemConfigPurposeProvision:
		return "-downloader"
	case extensionsv1alpha1.OperatingSystemConfigPurposeReconcile:
		return "-original"
	}
	return ""
}
