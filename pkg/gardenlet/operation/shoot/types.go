// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shoot

import (
	"context"
	"net"
	"sync"
	"sync/atomic"

	"github.com/Masterminds/semver/v3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/autoscaling/clusterautoscaler"
	"github.com/gardener/gardener/pkg/component/autoscaling/vpa"
	"github.com/gardener/gardener/pkg/component/clusteridentity"
	etcdcopybackupstask "github.com/gardener/gardener/pkg/component/etcd/copybackupstask"
	"github.com/gardener/gardener/pkg/component/etcd/etcd"
	"github.com/gardener/gardener/pkg/component/extensions/containerruntime"
	"github.com/gardener/gardener/pkg/component/extensions/controlplane"
	"github.com/gardener/gardener/pkg/component/extensions/dnsrecord"
	"github.com/gardener/gardener/pkg/component/extensions/extension"
	"github.com/gardener/gardener/pkg/component/extensions/infrastructure"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig"
	"github.com/gardener/gardener/pkg/component/extensions/worker"
	"github.com/gardener/gardener/pkg/component/garden/backupentry"
	"github.com/gardener/gardener/pkg/component/gardener/resourcemanager"
	kubeapiserver "github.com/gardener/gardener/pkg/component/kubernetes/apiserver"
	kubecontrollermanager "github.com/gardener/gardener/pkg/component/kubernetes/controllermanager"
	kubernetesdashboard "github.com/gardener/gardener/pkg/component/kubernetes/dashboard"
	kubeproxy "github.com/gardener/gardener/pkg/component/kubernetes/proxy"
	kubescheduler "github.com/gardener/gardener/pkg/component/kubernetes/scheduler"
	"github.com/gardener/gardener/pkg/component/networking/apiserverproxy"
	"github.com/gardener/gardener/pkg/component/networking/coredns"
	vpnseedserver "github.com/gardener/gardener/pkg/component/networking/vpn/seedserver"
	"github.com/gardener/gardener/pkg/component/nodemanagement/machinecontrollermanager"
	"github.com/gardener/gardener/pkg/component/observability/logging/vali"
	"github.com/gardener/gardener/pkg/component/observability/monitoring/alertmanager"
	"github.com/gardener/gardener/pkg/component/observability/monitoring/kubestatemetrics"
	"github.com/gardener/gardener/pkg/component/observability/monitoring/prometheus"
	"github.com/gardener/gardener/pkg/component/observability/plutono"
	shootsystem "github.com/gardener/gardener/pkg/component/shoot/system"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

// Builder is an object that builds Shoot objects.
type Builder struct {
	shootObjectFunc              func(context.Context) (*gardencorev1beta1.Shoot, error)
	cloudProfileFunc             func(context.Context, string) (*gardencorev1beta1.CloudProfile, error)
	shootSecretFunc              func(context.Context, string, string) (*corev1.Secret, error)
	serviceAccountIssuerHostname func() (*string, error)
	seed                         *gardencorev1beta1.Seed
	exposureClass                *gardencorev1beta1.ExposureClass
	projectName                  string
	internalDomain               *gardenerutils.Domain
	defaultDomains               []*gardenerutils.Domain
}

// Shoot is an object containing information about a Shoot cluster.
type Shoot struct {
	info      atomic.Value
	infoMutex sync.Mutex

	shootState atomic.Value

	Secret        *corev1.Secret
	CloudProfile  *gardencorev1beta1.CloudProfile
	ExposureClass *gardencorev1beta1.ExposureClass

	SeedNamespace     string
	KubernetesVersion *semver.Version
	GardenerVersion   *semver.Version

	InternalClusterDomain string
	ExternalClusterDomain *string
	ExternalDomain        *gardenerutils.Domain

	Purpose                                 gardencorev1beta1.ShootPurpose
	IsWorkerless                            bool
	WantsClusterAutoscaler                  bool
	WantsVerticalPodAutoscaler              bool
	WantsAlertmanager                       bool
	IgnoreAlerts                            bool
	HibernationEnabled                      bool
	VPNHighAvailabilityEnabled              bool
	VPNHighAvailabilityNumberOfSeedServers  int
	VPNHighAvailabilityNumberOfShootClients int
	NodeLocalDNSEnabled                     bool
	TopologyAwareRoutingEnabled             bool
	Networks                                *Networks
	BackupEntryName                         string
	OSCSyncJitterPeriod                     *metav1.Duration
	ResourcesToEncrypt                      []string
	EncryptedResources                      []string
	ServiceAccountIssuerHostname            *string

	Components *Components
}

// Components contains different components deployed in the Shoot cluster.
type Components struct {
	BackupEntry              backupentry.Interface
	SourceBackupEntry        backupentry.Interface
	ControlPlane             *ControlPlane
	Extensions               *Extensions
	SystemComponents         *SystemComponents
	Addons                   *Addons
	GardenerAccess           component.Deployer
	DependencyWatchdogAccess component.Deployer
}

// ControlPlane contains references to K8S control plane components.
type ControlPlane struct {
	Alertmanager             alertmanager.Interface
	BlackboxExporter         component.DeployWaiter
	ClusterAutoscaler        clusterautoscaler.Interface
	EtcdMain                 etcd.Interface
	EtcdEvents               etcd.Interface
	EtcdCopyBackupsTask      etcdcopybackupstask.Interface
	EventLogger              component.Deployer
	KubeAPIServerIngress     component.Deployer
	KubeAPIServerService     component.DeployWaiter
	KubeAPIServerSNI         component.DeployWaiter
	KubeAPIServer            kubeapiserver.Interface
	KubeScheduler            kubescheduler.Interface
	KubeControllerManager    kubecontrollermanager.Interface
	KubeStateMetrics         kubestatemetrics.Interface
	MachineControllerManager machinecontrollermanager.Interface
	Plutono                  plutono.Interface
	Prometheus               prometheus.Interface
	ResourceManager          resourcemanager.Interface
	Vali                     vali.Interface
	VerticalPodAutoscaler    vpa.Interface
	VPNSeedServer            vpnseedserver.Interface
}

// Extensions contains references to extension resources.
type Extensions struct {
	ContainerRuntime      containerruntime.Interface
	ControlPlane          controlplane.Interface
	ControlPlaneExposure  controlplane.Interface
	ExternalDNSRecord     dnsrecord.Interface
	InternalDNSRecord     dnsrecord.Interface
	IngressDNSRecord      dnsrecord.Interface
	Extension             extension.Interface
	Infrastructure        infrastructure.Interface
	Network               component.DeployMigrateWaiter
	OperatingSystemConfig operatingsystemconfig.Interface
	Worker                worker.Interface
}

// SystemComponents contains references to system components.
type SystemComponents struct {
	APIServerProxy      apiserverproxy.Interface
	BlackboxExporter    component.DeployWaiter
	ClusterIdentity     clusteridentity.Interface
	CoreDNS             coredns.Interface
	KubeProxy           kubeproxy.Interface
	MetricsServer       component.DeployWaiter
	Namespaces          component.DeployWaiter
	NodeLocalDNS        component.DeployWaiter
	NodeProblemDetector component.DeployWaiter
	NodeExporter        component.DeployWaiter
	Resources           shootsystem.Interface
	VPNShoot            component.DeployWaiter
}

// Addons contains references for the addons.
type Addons struct {
	KubernetesDashboard kubernetesdashboard.Interface
	NginxIngress        component.Deployer
}

// Networks contains pre-calculated subnets and IP address for various components.
type Networks struct {
	// Pods subnet
	Pods *net.IPNet
	// Services subnet
	Services *net.IPNet
	// APIServer is the ClusterIP of default/kubernetes Service
	APIServer net.IP
	// CoreDNS is the ClusterIP of kube-system/coredns Service
	CoreDNS net.IP
}
