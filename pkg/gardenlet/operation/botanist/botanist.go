// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"
	"fmt"
	"slices"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/clock"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/component/etcd/etcd"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

// DefaultInterval is the default interval for retry operations.
const DefaultInterval = 5 * time.Second

// New takes an operation object <o> and creates a new Botanist object.
func New(ctx context.Context, o *operation.Operation) (*Botanist, error) {
	var (
		b   = &Botanist{Operation: o}
		err error
	)

	o.SecretsManager, err = secretsmanager.New(
		ctx,
		b.Logger.WithName("secretsmanager"),
		clock.RealClock{},
		b.SeedClientSet.Client(),
		b.Shoot.ControlPlaneNamespace,
		v1beta1constants.SecretManagerIdentityGardenlet,
		secretsmanager.Config{
			CASecretAutoRotation: false,
			SecretNamesToTimes:   b.lastSecretRotationStartTimes(),
		},
	)
	if err != nil {
		return nil, err
	}

	// extension components
	o.Shoot.Components.Extensions.ExternalDNSRecord = b.DefaultExternalDNSRecord()
	o.Shoot.Components.Extensions.InternalDNSRecord = b.DefaultInternalDNSRecord()
	o.Shoot.Components.Extensions.IngressDNSRecord = b.DefaultIngressDNSRecord()

	o.Shoot.Components.Extensions.Extension, err = b.DefaultExtension(ctx)
	if err != nil {
		return nil, err
	}
	if !o.Shoot.IsWorkerless {
		o.Shoot.Components.Extensions.ContainerRuntime = b.DefaultContainerRuntime()
		o.Shoot.Components.Extensions.ControlPlane = b.DefaultControlPlane(extensionsv1alpha1.Normal)
		o.Shoot.Components.Extensions.ControlPlaneExposure = b.DefaultControlPlane(extensionsv1alpha1.Exposure)
		o.Shoot.Components.Extensions.Infrastructure = b.DefaultInfrastructure()
		o.Shoot.Components.Extensions.Network = b.DefaultNetwork()
		o.Shoot.Components.Extensions.Worker = b.DefaultWorker()

		if o.Shoot.IsAutonomous() && !o.Shoot.RunsControlPlane() {
			// For `gardenadm bootstrap`, we don't initialize the control plane machines with a "full OSC".
			// Instead, we provide a small alternative OSC, that only fetches the `gardenadm` binary from the registry.
			o.Shoot.Components.Extensions.OperatingSystemConfig, err = b.ControlPlaneBootstrapOperatingSystemConfig()
			if err != nil {
				return nil, err
			}
		} else {
			o.Shoot.Components.Extensions.OperatingSystemConfig, err = b.DefaultOperatingSystemConfig()
			if err != nil {
				return nil, err
			}
		}
	}

	// control plane components
	o.Shoot.Components.ControlPlane.Alertmanager, err = b.DefaultAlertmanager()
	if err != nil {
		return nil, err
	}
	o.Shoot.Components.ControlPlane.BlackboxExporter, err = b.DefaultBlackboxExporterControlPlane()
	if err != nil {
		return nil, err
	}
	o.Shoot.Components.ControlPlane.EtcdCopyBackupsTask = b.DefaultEtcdCopyBackupsTask()
	o.Shoot.Components.ControlPlane.EtcdMain, err = b.DefaultEtcd(v1beta1constants.ETCDRoleMain, etcd.ClassImportant)
	if err != nil {
		return nil, err
	}
	o.Shoot.Components.ControlPlane.EtcdEvents, err = b.DefaultEtcd(v1beta1constants.ETCDRoleEvents, etcd.ClassNormal)
	if err != nil {
		return nil, err
	}
	o.Shoot.Components.ControlPlane.EventLogger, err = b.DefaultEventLogger()
	if err != nil {
		return nil, err
	}
	o.Shoot.Components.ControlPlane.KubeAPIServerIngress = b.DefaultKubeAPIServerIngress()
	o.Shoot.Components.ControlPlane.KubeAPIServerService = b.DefaultKubeAPIServerService()
	o.Shoot.Components.ControlPlane.KubeAPIServerSNI = b.DefaultKubeAPIServerSNI()
	o.Shoot.Components.ControlPlane.KubeAPIServer, err = b.DefaultKubeAPIServer(ctx)
	if err != nil {
		return nil, err
	}
	o.Shoot.Components.ControlPlane.KubeControllerManager, err = b.DefaultKubeControllerManager()
	if err != nil {
		return nil, err
	}
	o.Shoot.Components.ControlPlane.KubeStateMetrics, err = b.DefaultKubeStateMetrics()
	if err != nil {
		return nil, err
	}
	o.Shoot.Components.ControlPlane.Plutono, err = b.DefaultPlutono()
	if err != nil {
		return nil, err
	}
	o.Shoot.Components.ControlPlane.Prometheus, err = b.DefaultPrometheus()
	if err != nil {
		return nil, err
	}
	o.Shoot.Components.ControlPlane.ResourceManager, err = b.DefaultResourceManager()
	if err != nil {
		return nil, err
	}
	if !o.Shoot.IsWorkerless {
		o.Shoot.Components.ControlPlane.ClusterAutoscaler, err = b.DefaultClusterAutoscaler()
		if err != nil {
			return nil, err
		}
		o.Shoot.Components.ControlPlane.KubeScheduler, err = b.DefaultKubeScheduler()
		if err != nil {
			return nil, err
		}
		o.Shoot.Components.ControlPlane.VerticalPodAutoscaler, err = b.DefaultVerticalPodAutoscaler()
		if err != nil {
			return nil, err
		}
		o.Shoot.Components.ControlPlane.VPNSeedServer, err = b.DefaultVPNSeedServer()
		if err != nil {
			return nil, err
		}
		o.Shoot.Components.ControlPlane.MachineControllerManager, err = b.DefaultMachineControllerManager()
		if err != nil {
			return nil, err
		}
	}
	o.Shoot.Components.ControlPlane.Vali, err = b.DefaultVali()
	if err != nil {
		return nil, err
	}

	// system components
	o.Shoot.Components.SystemComponents.Resources = b.DefaultShootSystem()
	o.Shoot.Components.SystemComponents.Namespaces = b.DefaultShootNamespaces()
	o.Shoot.Components.SystemComponents.ClusterIdentity = b.DefaultClusterIdentity()

	if !o.Shoot.IsWorkerless {
		o.Shoot.Components.SystemComponents.APIServerProxy, err = b.DefaultAPIServerProxy()
		if err != nil {
			return nil, err
		}

		o.Shoot.Components.SystemComponents.BlackboxExporter, err = b.DefaultBlackboxExporterCluster()
		if err != nil {
			return nil, err
		}

		o.Shoot.Components.SystemComponents.CoreDNS, err = b.DefaultCoreDNS()
		if err != nil {
			return nil, err
		}
		o.Shoot.Components.SystemComponents.NodeLocalDNS, err = b.DefaultNodeLocalDNS()
		if err != nil {
			return nil, err
		}
		o.Shoot.Components.SystemComponents.MetricsServer, err = b.DefaultMetricsServer()
		if err != nil {
			return nil, err
		}
		o.Shoot.Components.SystemComponents.VPNShoot, err = b.DefaultVPNShoot()
		if err != nil {
			return nil, err
		}
		o.Shoot.Components.SystemComponents.NodeProblemDetector, err = b.DefaultNodeProblemDetector()
		if err != nil {
			return nil, err
		}
		o.Shoot.Components.SystemComponents.NodeExporter, err = b.DefaultNodeExporter()
		if err != nil {
			return nil, err
		}
		o.Shoot.Components.SystemComponents.KubeProxy, err = b.DefaultKubeProxy()
		if err != nil {
			return nil, err
		}
	}

	// other components
	o.Shoot.Components.SourceBackupEntry = b.SourceBackupEntry()
	o.Shoot.Components.BackupEntry = b.DefaultCoreBackupEntry()
	o.Shoot.Components.GardenerAccess = b.DefaultGardenerAccess()
	if !o.Shoot.IsWorkerless {
		o.Shoot.Components.DependencyWatchdogAccess = b.DefaultDependencyWatchdogAccess()
	}

	// Addons
	if !o.Shoot.IsWorkerless {
		o.Shoot.Components.Addons.KubernetesDashboard, err = b.DefaultKubernetesDashboard()
		if err != nil {
			return nil, err
		}
		o.Shoot.Components.Addons.NginxIngress, err = b.DefaultNginxIngress()
		if err != nil {
			return nil, err
		}
	}

	return b, nil
}

// IsGardenerResourceManagerReady checks if gardener-resource-manager is running, so that node-agent-authorizer webhook is accessible.
func (b *Botanist) IsGardenerResourceManagerReady(ctx context.Context) (bool, error) {
	// The reconcile flow deploys the kube-apiserver of the shoot before the gardener-resource-manager (it has to be
	// this way, otherwise the Gardener components cannot start). However, GRM serves an authorization webhook for the
	// NodeAgentAuthorizer feature. We can only configure kube-apiserver to consult this webhook when GRM runs, obviously.
	// This is not possible in the initial kube-apiserver deployment (due to above order).
	// Hence, we have to deploy kube-apiserver a second time - this time with the NodeAgentAuthorizer feature getting enabled.
	// From then on, all subsequent reconciliations can always enable it and only one deployment is needed.
	resourceManagerDeployment := &appsv1.Deployment{}
	if err := b.SeedClientSet.Client().Get(ctx, client.ObjectKey{Name: v1beta1constants.DeploymentNameGardenerResourceManager, Namespace: b.Shoot.ControlPlaneNamespace}, resourceManagerDeployment); err != nil {
		if !apierrors.IsNotFound(err) {
			return false, err
		}
		return false, nil
	}

	return resourceManagerDeployment.Status.ReadyReplicas > 0, nil
}

// RequiredExtensionsReady checks whether all required extensions needed for a shoot operation exist and are ready.
func (b *Botanist) RequiredExtensionsReady(ctx context.Context) error {
	controllerRegistrationList := &gardencorev1beta1.ControllerRegistrationList{}
	if err := b.GardenClient.List(ctx, controllerRegistrationList); err != nil {
		return err
	}
	requiredExtensions := gardenerutils.ComputeRequiredExtensionsForShoot(b.Shoot.GetInfo(), b.Seed.GetInfo(), controllerRegistrationList, b.Garden.InternalDomain, b.Shoot.ExternalDomain)

	return gardenerutils.RequiredExtensionsReady(ctx, b.GardenClient, b.Seed.GetInfo().Name, requiredExtensions)
}

// outOfClusterAPIServerFQDN returns the Fully Qualified Domain Name of the apiserver
// with dot "." suffix. It'll prevent extra requests to the DNS in case the record is not
// available.
func (b *Botanist) outOfClusterAPIServerFQDN() string {
	return fmt.Sprintf("%s.", b.Shoot.ComputeOutOfClusterAPIServerAddress(true))
}

// SetInPlaceUpdatePendingWorkers sets the Shoot status with the name of worker pools which are undergoing an in-place update.
func (b *Botanist) SetInPlaceUpdatePendingWorkers(ctx context.Context, worker *extensionsv1alpha1.Worker) error {
	var (
		autoInPlaceUpdatePendingWorkers   []string
		manualInPlaceUpdatePendingWorkers []string
	)

	for _, pool := range b.Shoot.GetInfo().Spec.Provider.Workers {
		if !v1beta1helper.IsUpdateStrategyInPlace(pool.UpdateStrategy) {
			continue
		}

		if worker != nil {
			var oldPool extensionsv1alpha1.WorkerPool
			oldPoolIndex := slices.IndexFunc(worker.Spec.Pools, func(ow extensionsv1alpha1.WorkerPool) bool {
				oldPool = ow
				return ow.Name == pool.Name
			})

			if oldPoolIndex != -1 && worker.Status.InPlaceUpdates != nil && worker.Status.InPlaceUpdates.WorkerPoolToHashMap != nil {
				if oldPoolHash, ok := worker.Status.InPlaceUpdates.WorkerPoolToHashMap[oldPool.Name]; ok {
					var (
						kubernetesVersion    = b.Shoot.GetInfo().Spec.Kubernetes.Version
						kubeletConfiguration = b.Shoot.GetInfo().Spec.Kubernetes.Kubelet
					)

					if pool.Kubernetes != nil {
						if pool.Kubernetes.Version != nil {
							kubernetesVersion = *pool.Kubernetes.Version
						}

						if pool.Kubernetes.Kubelet != nil {
							kubeletConfiguration = pool.Kubernetes.Kubelet
						}
					}

					newPoolHash, err := gardenerutils.CalculateWorkerPoolHashForInPlaceUpdate(
						pool.Name,
						&kubernetesVersion,
						kubeletConfiguration,
						ptr.Deref(pool.Machine.Image.Version, ""),
						b.Shoot.GetInfo().Status.Credentials,
					)
					if err != nil {
						return fmt.Errorf("failed to calculate worker pool %q hash: %w", pool.Name, err)
					}

					if oldPoolHash == newPoolHash {
						continue
					}
				}
			}
		}

		switch ptr.Deref(pool.UpdateStrategy, "") {
		case gardencorev1beta1.AutoInPlaceUpdate:
			autoInPlaceUpdatePendingWorkers = append(autoInPlaceUpdatePendingWorkers, pool.Name)
		case gardencorev1beta1.ManualInPlaceUpdate:
			manualInPlaceUpdatePendingWorkers = append(manualInPlaceUpdatePendingWorkers, pool.Name)
		}
	}

	if len(autoInPlaceUpdatePendingWorkers) == 0 && len(manualInPlaceUpdatePendingWorkers) == 0 {
		return nil
	}

	if b.Shoot.GetInfo().Status.InPlaceUpdates != nil &&
		b.Shoot.GetInfo().Status.InPlaceUpdates.PendingWorkerUpdates != nil &&
		sets.New(autoInPlaceUpdatePendingWorkers...).Equal(sets.New(b.Shoot.GetInfo().Status.InPlaceUpdates.PendingWorkerUpdates.AutoInPlaceUpdate...)) &&
		sets.New(manualInPlaceUpdatePendingWorkers...).Equal(sets.New(b.Shoot.GetInfo().Status.InPlaceUpdates.PendingWorkerUpdates.ManualInPlaceUpdate...)) {
		return nil
	}

	return b.Shoot.UpdateInfoStatus(ctx, b.GardenClient, false, true, func(shoot *gardencorev1beta1.Shoot) error {
		if shoot.Status.InPlaceUpdates == nil {
			shoot.Status.InPlaceUpdates = &gardencorev1beta1.InPlaceUpdatesStatus{}
		}

		if shoot.Status.InPlaceUpdates.PendingWorkerUpdates == nil {
			shoot.Status.InPlaceUpdates.PendingWorkerUpdates = &gardencorev1beta1.PendingWorkerUpdates{}
		}

		for _, poolName := range autoInPlaceUpdatePendingWorkers {
			if slices.Contains(shoot.Status.InPlaceUpdates.PendingWorkerUpdates.AutoInPlaceUpdate, poolName) {
				continue
			}
			shoot.Status.InPlaceUpdates.PendingWorkerUpdates.AutoInPlaceUpdate = append(shoot.Status.InPlaceUpdates.PendingWorkerUpdates.AutoInPlaceUpdate, poolName)
		}

		for _, poolName := range manualInPlaceUpdatePendingWorkers {
			if slices.Contains(shoot.Status.InPlaceUpdates.PendingWorkerUpdates.ManualInPlaceUpdate, poolName) {
				continue
			}
			shoot.Status.InPlaceUpdates.PendingWorkerUpdates.ManualInPlaceUpdate = append(shoot.Status.InPlaceUpdates.PendingWorkerUpdates.ManualInPlaceUpdate, poolName)
		}

		return nil
	})
}
