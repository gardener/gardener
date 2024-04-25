// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/imagevector"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/observability/monitoring"
	"github.com/gardener/gardener/pkg/component/observability/monitoring/alertmanager"
	sharedcomponent "github.com/gardener/gardener/pkg/component/shared"
	gardenlethelper "github.com/gardener/gardener/pkg/gardenlet/apis/config/helper"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// DefaultAlertmanager creates a new alertmanager deployer.
func (b *Botanist) DefaultAlertmanager() (alertmanager.Interface, error) {
	var emailReceivers []string
	if monitoring := b.Shoot.GetInfo().Spec.Monitoring; monitoring != nil && monitoring.Alerting != nil {
		emailReceivers = monitoring.Alerting.EmailReceivers
	}

	return sharedcomponent.NewAlertmanager(b.Logger, b.SeedClientSet.Client(), b.Shoot.SeedNamespace, alertmanager.Values{
		Name:               "shoot",
		ClusterType:        component.ClusterTypeShoot,
		PriorityClassName:  v1beta1constants.PriorityClassNameShootControlPlane100,
		StorageCapacity:    resource.MustParse(b.Seed.GetValidVolumeSize("1Gi")),
		Replicas:           b.Shoot.GetReplicas(1),
		AlertingSMTPSecret: b.LoadSecret(v1beta1constants.GardenRoleAlerting),
		EmailReceivers:     emailReceivers,
		Ingress: &alertmanager.IngressValues{
			Host:           b.ComputeAlertManagerHost(),
			SecretsManager: b.SecretsManager,
			SigningCA:      v1beta1constants.SecretNameCACluster,
		},
	})
}

// DeployAlertManager reconciles the shoot alert manager.
func (b *Botanist) DeployAlertManager(ctx context.Context) error {
	if !b.Shoot.WantsAlertmanager || !b.IsShootMonitoringEnabled() {
		return b.Shoot.Components.Monitoring.Alertmanager.Destroy(ctx)
	}

	ingressAuthSecret, found := b.SecretsManager.Get(v1beta1constants.SecretNameObservabilityIngressUsers)
	if !found {
		return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameObservabilityIngressUsers)
	}

	b.Operation.Shoot.Components.Monitoring.Alertmanager.SetIngressAuthSecret(ingressAuthSecret)
	b.Operation.Shoot.Components.Monitoring.Alertmanager.SetIngressWildcardCertSecret(b.ControlPlaneWildcardCert)

	if err := b.Shoot.Components.Monitoring.Alertmanager.Deploy(ctx); err != nil {
		return err
	}

	// TODO(rfranzke): Remove this after v1.93 has been released.
	return kubernetesutils.DeleteObjects(ctx, b.SeedClientSet.Client(),
		&appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: "alertmanager", Namespace: b.Shoot.SeedNamespace}},
		&networkingv1.Ingress{ObjectMeta: metav1.ObjectMeta{Name: "alertmanager", Namespace: b.Shoot.SeedNamespace}},
		&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "alertmanager-client", Namespace: b.Shoot.SeedNamespace}},
		&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "alertmanager", Namespace: b.Shoot.SeedNamespace}},
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "alertmanager-basic-auth", Namespace: b.Shoot.SeedNamespace}},
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "alertmanager-config", Namespace: b.Shoot.SeedNamespace}},
		&corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "alertmanager-db-alertmanager-0", Namespace: b.Shoot.SeedNamespace}},
	)
}

// MigrateAlertManager migrate the shoot alert manager to prometheus-operator.
// TODO(rfranzke): Remove this function after v1.93 has been released.
func (b *Botanist) MigrateAlertManager(ctx context.Context) error {
	oldStatefulSet := &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: "alertmanager", Namespace: b.Shoot.SeedNamespace}}
	if err := b.SeedClientSet.Client().Get(ctx, client.ObjectKeyFromObject(oldStatefulSet), oldStatefulSet); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("failed reading old Alertmanager StatefulSet %s: %w", client.ObjectKeyFromObject(oldStatefulSet), err)
	}

	return b.DeployAlertManager(ctx)
}

// DefaultMonitoring creates a new monitoring component.
func (b *Botanist) DefaultMonitoring() (monitoring.Interface, error) {
	imageBlackboxExporter, err := imagevector.ImageVector().FindImage(imagevector.ImageNameBlackboxExporter)
	if err != nil {
		return nil, err
	}
	imageConfigmapReloader, err := imagevector.ImageVector().FindImage(imagevector.ImageNameConfigmapReloader)
	if err != nil {
		return nil, err
	}
	imagePrometheus, err := imagevector.ImageVector().FindImage(imagevector.ImageNamePrometheus)
	if err != nil {
		return nil, err
	}

	var alertingSecrets []*corev1.Secret
	for _, key := range b.GetSecretKeysOfRole(v1beta1constants.GardenRoleAlerting) {
		alertingSecrets = append(alertingSecrets, b.LoadSecret(key))
	}

	values := monitoring.Values{
		AlertingSecrets:              alertingSecrets,
		AlertmanagerEnabled:          b.Shoot.WantsAlertmanager,
		APIServerDomain:              gardenerutils.GetAPIServerDomain(b.Shoot.InternalClusterDomain),
		APIServerHost:                b.SeedClientSet.RESTConfig().Host,
		Config:                       b.Config.Monitoring,
		GlobalShootRemoteWriteSecret: b.LoadSecret(v1beta1constants.GardenRoleGlobalShootRemoteWriteMonitoring),
		IgnoreAlerts:                 b.Shoot.IgnoreAlerts,
		ImageBlackboxExporter:        imageBlackboxExporter.String(),
		ImageConfigmapReloader:       imageConfigmapReloader.String(),
		ImagePrometheus:              imagePrometheus.String(),
		IngressHostPrometheus:        b.ComputePrometheusHost(),
		IsWorkerless:                 b.Shoot.IsWorkerless,
		KubernetesVersion:            b.Shoot.GetInfo().Spec.Kubernetes.Version,
		MonitoringConfig:             b.Shoot.GetInfo().Spec.Monitoring,
		NodeLocalDNSEnabled:          b.Shoot.NodeLocalDNSEnabled,
		ProjectName:                  b.Garden.Project.Name,
		Replicas:                     b.Shoot.GetReplicas(1),
		RuntimeProviderType:          b.Seed.GetInfo().Spec.Provider.Type,
		RuntimeRegion:                b.Seed.GetInfo().Spec.Provider.Region,
		TargetName:                   b.Shoot.GetInfo().Name,
		TargetProviderType:           b.Shoot.GetInfo().Spec.Provider.Type,
		WildcardCertName:             nil,
	}

	if b.Shoot.Networks != nil {
		if services := b.Shoot.Networks.Services; services != nil {
			values.ServiceNetworkCIDR = ptr.To(services.String())
		}
		if pods := b.Shoot.Networks.Pods; pods != nil {
			values.PodNetworkCIDR = ptr.To(pods.String())
		}
		if apiServer := b.Shoot.Networks.APIServer; apiServer != nil {
			values.APIServerServiceIP = ptr.To(apiServer.String())
		}
	}

	if b.Shoot.GetInfo().Spec.Networking != nil {
		values.NodeNetworkCIDR = b.Shoot.GetInfo().Spec.Networking.Nodes
	}

	return monitoring.New(
		b.SeedClientSet.Client(),
		b.SeedClientSet.ChartApplier(),
		b.SecretsManager,
		b.Shoot.SeedNamespace,
		values,
	), nil
}

// DeployMonitoring installs the Helm release "seed-monitoring" in the Seed clusters. It comprises components
// to monitor the Shoot cluster whose control plane runs in the Seed cluster.
func (b *Botanist) DeployMonitoring(ctx context.Context) error {
	if !b.IsShootMonitoringEnabled() {
		return b.Shoot.Components.Monitoring.Monitoring.Destroy(ctx)
	}

	if b.ControlPlaneWildcardCert != nil {
		b.Operation.Shoot.Components.Monitoring.Monitoring.SetWildcardCertName(ptr.To(b.ControlPlaneWildcardCert.GetName()))
	}
	b.Shoot.Components.Monitoring.Monitoring.SetNamespaceUID(b.SeedNamespaceObject.UID)
	b.Shoot.Components.Monitoring.Monitoring.SetComponents(b.getMonitoringComponents())
	return b.Shoot.Components.Monitoring.Monitoring.Deploy(ctx)
}

func (b *Botanist) getMonitoringComponents() []component.MonitoringComponent {
	// Fetch component-specific monitoring configuration
	monitoringComponents := []component.MonitoringComponent{
		b.Shoot.Components.ControlPlane.EtcdMain,
		b.Shoot.Components.ControlPlane.EtcdEvents,
		b.Shoot.Components.ControlPlane.KubeAPIServer,
		b.Shoot.Components.ControlPlane.KubeControllerManager,
		b.Shoot.Components.ControlPlane.KubeStateMetrics,
		b.Shoot.Components.ControlPlane.ResourceManager,
	}

	if b.Shoot.IsShootControlPlaneLoggingEnabled(b.Config) && gardenlethelper.IsValiEnabled(b.Config) {
		monitoringComponents = append(monitoringComponents, b.Shoot.Components.Logging.Vali)
	}

	if !b.Shoot.IsWorkerless {
		monitoringComponents = append(monitoringComponents,
			b.Shoot.Components.ControlPlane.KubeScheduler,
			b.Shoot.Components.ControlPlane.MachineControllerManager,
			b.Shoot.Components.ControlPlane.VPNSeedServer,
			b.Shoot.Components.SystemComponents.BlackboxExporter,
			b.Shoot.Components.SystemComponents.CoreDNS,
			b.Shoot.Components.SystemComponents.KubeProxy,
			b.Shoot.Components.SystemComponents.NodeExporter,
			b.Shoot.Components.SystemComponents.VPNShoot,
		)

		if b.ShootUsesDNS() {
			monitoringComponents = append(monitoringComponents, b.Shoot.Components.SystemComponents.APIServerProxy)
		}

		if b.Shoot.NodeLocalDNSEnabled {
			monitoringComponents = append(monitoringComponents, b.Shoot.Components.SystemComponents.NodeLocalDNS)
		}

		if b.Shoot.WantsClusterAutoscaler {
			monitoringComponents = append(monitoringComponents, b.Shoot.Components.ControlPlane.ClusterAutoscaler)
		}
	}

	return monitoringComponents
}
