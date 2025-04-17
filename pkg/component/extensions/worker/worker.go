// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package worker

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/extensions"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

const (
	// DefaultInterval is the default interval for retry operations.
	DefaultInterval = 5 * time.Second
	// DefaultSevereThreshold is the default threshold until an error reported by another component is treated as
	// 'severe'.
	DefaultSevereThreshold = 30 * time.Second
	// DefaultTimeout is the default timeout and defines how long Gardener should wait for a successful reconciliation
	// of a Worker resource.
	DefaultTimeout = 10 * time.Minute
)

// TimeNow returns the current time. Exposed for testing.
var TimeNow = time.Now

// Interface is an interface for managing Workers.
type Interface interface {
	component.DeployMigrateWaiter
	Get(context.Context) (*extensionsv1alpha1.Worker, error)
	SetSSHPublicKey([]byte)
	SetInfrastructureProviderStatus(*runtime.RawExtension)
	SetWorkerPoolNameToOperatingSystemConfigsMap(map[string]*operatingsystemconfig.OperatingSystemConfigs)
	MachineDeployments() []extensionsv1alpha1.MachineDeployment
	WaitUntilWorkerStatusMachineDeploymentsUpdated(ctx context.Context) error
}

// Values contains the values used to create a Worker resources.
type Values struct {
	// Namespace is the Shoot namespace in the seed.
	Namespace string
	// Name is the name of the Worker resource.
	Name string
	// Type is the type of the Worker provider.
	Type string
	// Region is the region of the shoot.
	Region string
	// Workers is the list of worker pools.
	Workers []gardencorev1beta1.Worker
	// KubernetesVersion is the Kubernetes version of the cluster for which the worker nodes shall be created.
	KubernetesVersion *semver.Version
	// KubeletConfig is the configuration of the Kubelet
	KubeletConfig *gardencorev1beta1.KubeletConfig
	// MachineTypes is the list of machine types present in the CloudProfile referenced by the shoot
	MachineTypes []gardencorev1beta1.MachineType
	// SSHPublicKey is the public SSH key that shall be installed on the worker nodes.
	SSHPublicKey []byte
	// InfrastructureProviderStatus is the provider status of the Infrastructure resource which might be relevant for
	// the Worker reconciliation.
	InfrastructureProviderStatus *runtime.RawExtension
	// WorkerPoolNameToOperatingSystemConfigsMap contains the operating system configurations for the worker pools.
	WorkerPoolNameToOperatingSystemConfigsMap map[string]*operatingsystemconfig.OperatingSystemConfigs
	// NodeLocalDNSEnabled indicates whether node local dns is enabled or not.
	NodeLocalDNSEnabled bool
}

// New creates a new instance of Interface.
func New(
	log logr.Logger,
	client client.Client,
	values *Values,
	waitInterval time.Duration,
	waitSevereThreshold time.Duration,
	waitTimeout time.Duration,
) Interface {
	return &worker{
		log:                 log,
		client:              client,
		values:              values,
		waitInterval:        waitInterval,
		waitSevereThreshold: waitSevereThreshold,
		waitTimeout:         waitTimeout,

		worker: &extensionsv1alpha1.Worker{
			ObjectMeta: metav1.ObjectMeta{
				Name:      values.Name,
				Namespace: values.Namespace,
			},
		},
	}
}

type worker struct {
	values              *Values
	log                 logr.Logger
	client              client.Client
	waitInterval        time.Duration
	waitSevereThreshold time.Duration
	waitTimeout         time.Duration

	worker                           *extensionsv1alpha1.Worker
	machineDeployments               []extensionsv1alpha1.MachineDeployment
	machineDeploymentsLastUpdateTime *metav1.Time
}

// Deploy uses the seed client to create or update the Worker resource.
func (w *worker) Deploy(ctx context.Context) error {
	_, err := w.deploy(ctx, v1beta1constants.GardenerOperationReconcile)
	return err
}

func (w *worker) deploy(ctx context.Context, operation string) (extensionsv1alpha1.Object, error) {
	var pools []extensionsv1alpha1.WorkerPool

	obj := &extensionsv1alpha1.Worker{}
	if err := w.client.Get(ctx, client.ObjectKey{Name: w.worker.Name, Namespace: w.worker.Namespace}, obj); err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, err
		}
	}

	for _, workerPool := range w.values.Workers {
		var volume *extensionsv1alpha1.Volume
		if workerPool.Volume != nil {
			volume = &extensionsv1alpha1.Volume{
				Name:      workerPool.Volume.Name,
				Type:      workerPool.Volume.Type,
				Size:      workerPool.Volume.VolumeSize,
				Encrypted: workerPool.Volume.Encrypted,
			}
		}

		var dataVolumes []extensionsv1alpha1.DataVolume
		if len(workerPool.DataVolumes) > 0 {
			for _, dataVolume := range workerPool.DataVolumes {
				dataVolumes = append(dataVolumes, extensionsv1alpha1.DataVolume{
					Name:      dataVolume.Name,
					Type:      dataVolume.Type,
					Size:      dataVolume.VolumeSize,
					Encrypted: dataVolume.Encrypted,
				})
			}
		}

		var pConfig *runtime.RawExtension
		if workerPool.ProviderConfig != nil {
			pConfig = &runtime.RawExtension{
				Raw: workerPool.ProviderConfig.Raw,
			}
		}

		oscConfig, ok := w.values.WorkerPoolNameToOperatingSystemConfigsMap[workerPool.Name]
		if !ok {
			return nil, fmt.Errorf("missing operating system config for worker pool %v", workerPool.Name)
		}
		if oscConfig.Init.SecretName == nil {
			return nil, fmt.Errorf("missing secret name for worker pool %v", workerPool.Name)
		}

		var (
			gardenerNodeAgentSecretName = oscConfig.Init.GardenerNodeAgentSecretName
			nodeAgentSecretName         *string
		)
		if oscConfig.Init.IncludeSecretNameInWorkerPool {
			nodeAgentSecretName = &gardenerNodeAgentSecretName
		}

		workerPoolKubernetesVersion := w.values.KubernetesVersion.String()
		kubeletConfig := w.values.KubeletConfig
		if workerPool.Kubernetes != nil {
			if workerPool.Kubernetes.Version != nil {
				workerPoolKubernetesVersion = *workerPool.Kubernetes.Version
			}

			if workerPool.Kubernetes.Kubelet != nil {
				kubeletConfig = workerPool.Kubernetes.Kubelet
			}
		}

		nodeTemplate, machineType := w.findNodeTemplateAndMachineTypeByPoolName(obj, workerPool.Name)

		if nodeTemplate == nil || machineType != workerPool.Machine.Type {
			// initializing nodeTemplate by fetching details from cloudprofile, if present there
			if machineDetails := v1beta1helper.FindMachineTypeByName(w.values.MachineTypes, workerPool.Machine.Type); machineDetails != nil {
				nodeTemplate = &extensionsv1alpha1.NodeTemplate{
					Capacity: corev1.ResourceList{
						corev1.ResourceCPU:    machineDetails.CPU,
						"gpu":                 machineDetails.GPU,
						corev1.ResourceMemory: machineDetails.Memory,
					},
				}
			} else {
				nodeTemplate = nil
			}
		}

		var autoscalerOptions *extensionsv1alpha1.ClusterAutoscalerOptions
		if workerPool.ClusterAutoscaler != nil {
			autoscalerOptions = &extensionsv1alpha1.ClusterAutoscalerOptions{}
			if workerPool.ClusterAutoscaler.ScaleDownUtilizationThreshold != nil {
				autoscalerOptions.ScaleDownUtilizationThreshold = ptr.To(fmt.Sprint(*workerPool.ClusterAutoscaler.ScaleDownUtilizationThreshold))
			}
			if workerPool.ClusterAutoscaler.ScaleDownGpuUtilizationThreshold != nil {
				autoscalerOptions.ScaleDownGpuUtilizationThreshold = ptr.To(fmt.Sprint(*workerPool.ClusterAutoscaler.ScaleDownGpuUtilizationThreshold))
			}
			if workerPool.ClusterAutoscaler.ScaleDownUnneededTime != nil {
				autoscalerOptions.ScaleDownUnneededTime = workerPool.ClusterAutoscaler.ScaleDownUnneededTime
			}
			if workerPool.ClusterAutoscaler.ScaleDownUnreadyTime != nil {
				autoscalerOptions.ScaleDownUnreadyTime = workerPool.ClusterAutoscaler.ScaleDownUnreadyTime
			}
			if workerPool.ClusterAutoscaler.MaxNodeProvisionTime != nil {
				autoscalerOptions.MaxNodeProvisionTime = workerPool.ClusterAutoscaler.MaxNodeProvisionTime
			}
		}

		pools = append(pools, extensionsv1alpha1.WorkerPool{
			Name:           workerPool.Name,
			Minimum:        workerPool.Minimum,
			Maximum:        workerPool.Maximum,
			MaxSurge:       *workerPool.MaxSurge,
			MaxUnavailable: *workerPool.MaxUnavailable,
			Annotations:    workerPool.Annotations,
			Labels:         gardenerutils.NodeLabelsForWorkerPool(workerPool, w.values.NodeLocalDNSEnabled, gardenerNodeAgentSecretName),
			Taints:         workerPool.Taints,
			MachineType:    workerPool.Machine.Type,
			MachineImage: extensionsv1alpha1.MachineImage{
				Name:    workerPool.Machine.Image.Name,
				Version: *workerPool.Machine.Image.Version,
			},
			NodeTemplate:        nodeTemplate,
			NodeAgentSecretName: nodeAgentSecretName,
			ProviderConfig:      pConfig,
			UserDataSecretRef: corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: *oscConfig.Init.SecretName},
				Key:                  extensionsv1alpha1.OperatingSystemConfigSecretDataKey,
			},
			Volume:                           volume,
			DataVolumes:                      dataVolumes,
			KubeletDataVolumeName:            workerPool.KubeletDataVolumeName,
			KubernetesVersion:                &workerPoolKubernetesVersion,
			KubeletConfig:                    kubeletConfig,
			Zones:                            workerPool.Zones,
			MachineControllerManagerSettings: workerPool.MachineControllerManagerSettings,
			Architecture:                     workerPool.Machine.Architecture,
			ClusterAutoscaler:                autoscalerOptions,
			Priority:                         workerPool.Priority,
			UpdateStrategy:                   workerPool.UpdateStrategy,
		})
	}

	// We operate on arrays (pools) with merge patch without optimistic locking here, meaning this will replace
	// the arrays as a whole.
	// However, this is not a problem, as no other client should write to these arrays as the Worker spec is supposed
	// to be owned by gardenlet exclusively.
	_, err := controllerutils.GetAndCreateOrMergePatch(ctx, w.client, w.worker, func() error {
		metav1.SetMetaDataAnnotation(&w.worker.ObjectMeta, v1beta1constants.GardenerOperation, operation)
		metav1.SetMetaDataAnnotation(&w.worker.ObjectMeta, v1beta1constants.GardenerTimestamp, TimeNow().UTC().Format(time.RFC3339Nano))

		w.worker.Spec = extensionsv1alpha1.WorkerSpec{
			DefaultSpec: extensionsv1alpha1.DefaultSpec{
				Type: w.values.Type,
			},
			Region: w.values.Region,
			SecretRef: corev1.SecretReference{
				Name:      v1beta1constants.SecretNameCloudProvider,
				Namespace: w.worker.Namespace,
			},
			SSHPublicKey:                 w.values.SSHPublicKey,
			InfrastructureProviderStatus: w.values.InfrastructureProviderStatus,
			Pools:                        pools,
		}

		return nil
	})

	// populate the MachineDeploymentsLastUpdate time as it will be used later to confirm if the machineDeployments slice in the worker
	// status got updated with the latest ones.
	w.machineDeploymentsLastUpdateTime = obj.Status.MachineDeploymentsLastUpdateTime

	return w.worker, err
}

// Restore uses the seed client and the ShootState to create the Worker resources and restore their state.
func (w *worker) Restore(ctx context.Context, shootState *gardencorev1beta1.ShootState) error {
	return extensions.RestoreExtensionWithDeployFunction(
		ctx,
		w.client,
		shootState,
		extensionsv1alpha1.WorkerResource,
		w.deploy,
	)
}

// Migrate migrates the Worker resource.
func (w *worker) Migrate(ctx context.Context) error {
	return extensions.MigrateExtensionObject(
		ctx,
		w.client,
		w.worker,
	)
}

// Destroy deletes the Worker resource.
func (w *worker) Destroy(ctx context.Context) error {
	return extensions.DeleteExtensionObject(
		ctx,
		w.client,
		w.worker,
	)
}

// Wait waits until the Worker resource is ready.
func (w *worker) Wait(ctx context.Context) error {
	return extensions.WaitUntilExtensionObjectReady(
		ctx,
		w.client,
		w.log,
		w.worker,
		extensionsv1alpha1.WorkerResource,
		w.waitInterval,
		w.waitSevereThreshold,
		w.waitTimeout,
		nil,
	)
}

// WaitUntilWorkerStatusMachineDeploymentsUpdated waits until the worker status is updated with the latest machineDeployment slice.
func (w *worker) WaitUntilWorkerStatusMachineDeploymentsUpdated(ctx context.Context) error {
	return extensions.WaitUntilObjectReadyWithHealthFunction(
		ctx,
		w.client,
		w.log,
		w.checkWorkerStatusMachineDeploymentsUpdated,
		w.worker,
		extensionsv1alpha1.WorkerResource,
		w.waitInterval,
		w.waitSevereThreshold,
		w.waitTimeout,
		func() error {
			w.machineDeployments = w.worker.Status.MachineDeployments
			return nil
		},
	)
}

// WaitMigrate waits until the Worker resources are migrated successfully.
func (w *worker) WaitMigrate(ctx context.Context) error {
	return extensions.WaitUntilExtensionObjectMigrated(
		ctx,
		w.client,
		w.worker,
		extensionsv1alpha1.WorkerResource,
		w.waitInterval,
		w.waitTimeout,
	)
}

// WaitCleanup waits until the Worker resource is deleted.
func (w *worker) WaitCleanup(ctx context.Context) error {
	return extensions.WaitUntilExtensionObjectDeleted(
		ctx,
		w.client,
		w.log,
		w.worker,
		extensionsv1alpha1.WorkerResource,
		w.waitInterval,
		w.waitTimeout,
	)
}

// Get retrieves and returns the Worker resource.
func (w *worker) Get(ctx context.Context) (*extensionsv1alpha1.Worker, error) {
	if err := w.client.Get(ctx, client.ObjectKeyFromObject(w.worker), w.worker); err != nil {
		return nil, err
	}

	return w.worker, nil
}

// SetSSHPublicKey sets the public SSH key in the values.
func (w *worker) SetSSHPublicKey(key []byte) {
	w.values.SSHPublicKey = key
}

// SetInfrastructureProviderStatus sets the infrastructure provider status in the values.
func (w *worker) SetInfrastructureProviderStatus(status *runtime.RawExtension) {
	w.values.InfrastructureProviderStatus = status
}

// SetWorkerPoolNameToOperatingSystemConfigsMap sets the operating system config maps in the values.
func (w *worker) SetWorkerPoolNameToOperatingSystemConfigsMap(maps map[string]*operatingsystemconfig.OperatingSystemConfigs) {
	w.values.WorkerPoolNameToOperatingSystemConfigsMap = maps
}

// MachineDeployments returns the generated machine deployments of the Worker.
func (w *worker) MachineDeployments() []extensionsv1alpha1.MachineDeployment {
	return w.machineDeployments
}

func (w *worker) findNodeTemplateAndMachineTypeByPoolName(obj *extensionsv1alpha1.Worker, poolName string) (*extensionsv1alpha1.NodeTemplate, string) {
	for _, pool := range obj.Spec.Pools {
		if pool.Name == poolName {
			return pool.NodeTemplate, pool.MachineType
		}
	}
	return nil, ""
}

// checkWorkerStatusMachineDeploymentsUpdated checks if the status of the worker is updated or not during its reconciliation.
// It is updated if
// * The status.MachineDeploymentsLastUpdateTime > the value of the time stamp stored in worker struct before the reconciliation begins.
func (w *worker) checkWorkerStatusMachineDeploymentsUpdated(o client.Object) error {
	obj, ok := o.(*extensionsv1alpha1.Worker)
	if !ok {
		return fmt.Errorf("expected *extensionsv1alpha1.Worker but got %T", o)
	}

	if obj.Status.MachineDeploymentsLastUpdateTime != nil && (w.machineDeploymentsLastUpdateTime == nil || obj.Status.MachineDeploymentsLastUpdateTime.After(w.machineDeploymentsLastUpdateTime.Time)) {
		return nil
	}

	return errors.New("worker status machineDeployments has not been updated")
}
