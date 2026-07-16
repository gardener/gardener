// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"
	"fmt"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/component/etcd/etcd"
)

func (b *Botanist) instantiateComponents(ctx context.Context) (err error) {
	if err := b.instantiateComponentsControlPlane(ctx); err != nil {
		return fmt.Errorf("failed to instantiate control plane components: %w", err)
	}

	if err := b.instantiateComponentsObservability(); err != nil {
		return fmt.Errorf("failed to instantiate observability components: %w", err)
	}

	if err := b.instantiateComponentsExtensions(ctx); err != nil {
		return fmt.Errorf("failed to instantiate extension components: %w", err)
	}

	if err := b.instantiateComponentsSystem(); err != nil {
		return fmt.Errorf("failed to instantiate system components: %w", err)
	}

	b.instantiateComponentsMisc()

	if err := b.instantiateComponentsAddons(); err != nil {
		return fmt.Errorf("failed to instantiate addon components: %w", err)
	}

	return nil
}

func (b *Botanist) instantiateComponentsExtensions(ctx context.Context) (err error) {
	b.Shoot.Components.Extensions.ContainerRuntime = b.DefaultContainerRuntime()
	b.Shoot.Components.Extensions.ControlPlane = b.DefaultControlPlane()
	b.Shoot.Components.Extensions.Extension, err = b.DefaultExtension(ctx)
	if err != nil {
		return err
	}
	b.Shoot.Components.Extensions.ExternalDNSRecord = b.DefaultExternalDNSRecord()
	b.Shoot.Components.Extensions.InternalDNSRecord = b.DefaultInternalDNSRecord()
	b.Shoot.Components.Extensions.IngressDNSRecord = b.DefaultIngressDNSRecord()
	b.Shoot.Components.Extensions.Infrastructure = b.DefaultInfrastructure()
	b.Shoot.Components.Extensions.Network = b.DefaultNetwork()
	b.Shoot.Components.Extensions.OperatingSystemConfig, err = b.DefaultOperatingSystemConfig()
	if err != nil {
		return err
	}
	b.Shoot.Components.Extensions.SelfHostedShootExposure = b.DefaultSelfHostedShootExposure()
	b.Shoot.Components.Extensions.Worker = b.DefaultWorker()

	return nil
}

func (b *Botanist) instantiateComponentsControlPlane(ctx context.Context) (err error) {
	b.Shoot.Components.ControlPlane.ClusterAutoscaler, err = b.DefaultClusterAutoscaler()
	if err != nil {
		return err
	}
	b.Shoot.Components.ControlPlane.EtcdCopyBackupsTask = b.DefaultEtcdCopyBackupsTask()
	b.Shoot.Components.ControlPlane.EtcdDruid, err = b.DefaultEtcdDruid()
	if err != nil {
		return err
	}
	b.Shoot.Components.ControlPlane.EtcdMain, err = b.DefaultEtcd(v1beta1constants.ETCDRoleMain, etcd.ClassImportant)
	if err != nil {
		return err
	}
	b.Shoot.Components.ControlPlane.EtcdEvents, err = b.DefaultEtcd(v1beta1constants.ETCDRoleEvents, etcd.ClassNormal)
	if err != nil {
		return err
	}
	b.Shoot.Components.ControlPlane.IstioBasicAuthServer, err = b.DefaultIstioBasicAuthServer()
	if err != nil {
		return err
	}
	b.Shoot.Components.ControlPlane.KubeAPIServerService = b.DefaultKubeAPIServerService()
	b.Shoot.Components.ControlPlane.KubeAPIServerSNI = b.DefaultKubeAPIServerSNI()
	b.Shoot.Components.ControlPlane.KubeAPIServer, err = b.DefaultKubeAPIServer(ctx)
	if err != nil {
		return err
	}
	b.Shoot.Components.ControlPlane.KubeControllerManager, err = b.DefaultKubeControllerManager()
	if err != nil {
		return err
	}
	b.Shoot.Components.ControlPlane.KubeScheduler, err = b.DefaultKubeScheduler()
	if err != nil {
		return err
	}
	b.Shoot.Components.ControlPlane.MachineControllerManager, err = b.DefaultMachineControllerManager()
	if err != nil {
		return err
	}
	b.Shoot.Components.ControlPlane.ResourceManager, err = b.DefaultResourceManager()
	if err != nil {
		return err
	}
	b.Shoot.Components.ControlPlane.RuntimeResourceManager, err = b.DefaultRuntimeGardenerResourceManager()
	if err != nil {
		return err
	}
	b.Shoot.Components.ControlPlane.VerticalPodAutoscaler, err = b.DefaultVerticalPodAutoscaler()
	if err != nil {
		return err
	}
	b.Shoot.Components.ControlPlane.VPNSeedServer, err = b.DefaultVPNSeedServer()
	if err != nil {
		return err
	}

	return nil
}

func (b *Botanist) instantiateComponentsObservability() (err error) {
	// TODO(rfranzke,rickardsjp): Enable these components once the observability components are ready for self-hosted shoots.
	if b.Shoot.IsSelfHosted() {
		return nil
	}

	b.Shoot.Components.ControlPlane.Alertmanager, err = b.DefaultAlertmanager()
	if err != nil {
		return err
	}
	b.Shoot.Components.ControlPlane.BlackboxExporter, err = b.DefaultBlackboxExporterControlPlane()
	if err != nil {
		return err
	}
	b.Shoot.Components.ControlPlane.EventLogger, err = b.DefaultEventLogger()
	if err != nil {
		return err
	}
	b.Shoot.Components.ControlPlane.KubeStateMetrics, err = b.DefaultKubeStateMetrics()
	if err != nil {
		return err
	}
	b.Shoot.Components.ControlPlane.OtelCollector, err = b.DefaultOtelCollector()
	if err != nil {
		return err
	}
	b.Shoot.Components.ControlPlane.Plutono, err = b.DefaultPlutono()
	if err != nil {
		return err
	}
	b.Shoot.Components.ControlPlane.Prometheus, err = b.DefaultPrometheus()
	if err != nil {
		return err
	}
	b.Shoot.Components.ControlPlane.Vali, err = b.DefaultVali()
	if err != nil {
		return err
	}
	b.Shoot.Components.ControlPlane.VictoriaLogs, err = b.DefaultVictoriaLogs()
	if err != nil {
		return err
	}

	return nil
}

func (b *Botanist) instantiateComponentsSystem() (err error) {
	b.Shoot.Components.SystemComponents.APIServerProxy, err = b.DefaultAPIServerProxy()
	if err != nil {
		return err
	}
	b.Shoot.Components.SystemComponents.BlackboxExporter, err = b.DefaultBlackboxExporterCluster()
	if err != nil {
		return err
	}
	b.Shoot.Components.SystemComponents.ClusterIdentity = b.DefaultClusterIdentity()
	b.Shoot.Components.SystemComponents.CoreDNS, err = b.DefaultCoreDNS()
	if err != nil {
		return err
	}
	b.Shoot.Components.SystemComponents.KubeProxy, err = b.DefaultKubeProxy()
	if err != nil {
		return err
	}
	b.Shoot.Components.SystemComponents.MetricsServer, err = b.DefaultMetricsServer()
	if err != nil {
		return err
	}
	b.Shoot.Components.SystemComponents.Namespaces = b.DefaultShootNamespaces()
	b.Shoot.Components.SystemComponents.NodeExporter, err = b.DefaultNodeExporter()
	if err != nil {
		return err
	}
	b.Shoot.Components.SystemComponents.NodeLocalDNS, err = b.DefaultNodeLocalDNS()
	if err != nil {
		return err
	}
	b.Shoot.Components.SystemComponents.NodeProblemDetector, err = b.DefaultNodeProblemDetector()
	if err != nil {
		return err
	}
	b.Shoot.Components.SystemComponents.Resources = b.DefaultShootSystem()
	b.Shoot.Components.SystemComponents.VPNShoot, err = b.DefaultVPNShoot()
	if err != nil {
		return err
	}

	return nil
}

func (b *Botanist) instantiateComponentsMisc() {
	b.Shoot.Components.BackupBucket = b.DefaultCoreBackupBucket()
	b.Shoot.Components.BackupEntry = b.DefaultCoreBackupEntry()
	b.Shoot.Components.Bastion = b.DefaultBastion()
	b.Shoot.Components.DependencyWatchdogAccess = b.DefaultDependencyWatchdogAccess()
	b.Shoot.Components.GardenerAccess = b.DefaultGardenerAccess()
	b.Shoot.Components.SourceBackupEntry = b.SourceBackupEntry()
}

func (b *Botanist) instantiateComponentsAddons() (err error) {
	b.Shoot.Components.Addons.KubernetesDashboard, err = b.DefaultKubernetesDashboard()
	if err != nil {
		return err
	}
	b.Shoot.Components.Addons.NginxIngress, err = b.DefaultNginxIngress()
	if err != nil {
		return err
	}

	return nil
}
