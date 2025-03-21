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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

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
	"github.com/gardener/gardener/pkg/component/extensions/network"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig"
	"github.com/gardener/gardener/pkg/component/extensions/worker"
	"github.com/gardener/gardener/pkg/component/garden/backupentry"
	"github.com/gardener/gardener/pkg/component/gardener/resourcemanager"
	kubeapiserver "github.com/gardener/gardener/pkg/component/kubernetes/apiserver"
	kubecontrollermanager "github.com/gardener/gardener/pkg/component/kubernetes/controllermanager"
	kubernetesdashboard "github.com/gardener/gardener/pkg/component/kubernetes/dashboard"
	kubeproxy "github.com/gardener/gardener/pkg/component/kubernetes/proxy"
	"github.com/gardener/gardener/pkg/component/networking/apiserverproxy"
	"github.com/gardener/gardener/pkg/component/networking/coredns"
	"github.com/gardener/gardener/pkg/component/networking/nodelocaldns"
	vpnseedserver "github.com/gardener/gardener/pkg/component/networking/vpn/seedserver"
	vpnshoot "github.com/gardener/gardener/pkg/component/networking/vpn/shoot"
	"github.com/gardener/gardener/pkg/component/nodemanagement/machinecontrollermanager"
	"github.com/gardener/gardener/pkg/component/observability/logging/vali"
	"github.com/gardener/gardener/pkg/component/observability/monitoring/alertmanager"
	"github.com/gardener/gardener/pkg/component/observability/monitoring/prometheus"
	"github.com/gardener/gardener/pkg/component/observability/plutono"
	shootsystem "github.com/gardener/gardener/pkg/component/shoot/system"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

// Builder is an object that builds Shoot objects.
type Builder struct {
	shootObjectFunc              func(context.Context) (*gardencorev1beta1.Shoot, error)
	cloudProfileFunc             func(context.Context, *gardencorev1beta1.Shoot) (*gardencorev1beta1.CloudProfile, error)
	shootCredentialsFunc         func(context.Context, string, string, bool) (client.Object, error)
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

	// Credentials is either [*corev1.Secret] or [*securityv1alpha1.WorkloadIdentity]
	Credentials   client.Object
	CloudProfile  *gardencorev1beta1.CloudProfile
	ExposureClass *gardencorev1beta1.ExposureClass

	// ControlPlaneNamespace is the namespace in which the control plane components run.
	ControlPlaneNamespace string
	KubernetesVersion     *semver.Version
	GardenerVersion       *semver.Version

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
	VPNVPAUpdateDisabled                    bool
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
	Alertmanager        alertmanager.Interface
	BlackboxExporter    component.DeployWaiter
	ClusterAutoscaler   clusterautoscaler.Interface
	EtcdMain            etcd.Interface
	EtcdEvents          etcd.Interface
	EtcdCopyBackupsTask etcdcopybackupstask.Interface
	EventLogger         component.Deployer
	// TODO(oliver-goetz): Remove this deployer when Gardener v1.115.0 is released.
	KubeAPIServerIngress     component.Deployer
	KubeAPIServerService     component.DeployWaiter
	KubeAPIServerSNI         component.DeployWaiter
	KubeAPIServer            kubeapiserver.Interface
	KubeScheduler            component.DeployWaiter
	KubeControllerManager    kubecontrollermanager.Interface
	KubeStateMetrics         component.DeployWaiter
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
	Network               network.Interface
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
	NodeLocalDNS        nodelocaldns.Interface
	NodeProblemDetector component.DeployWaiter
	NodeExporter        component.DeployWaiter
	Resources           shootsystem.Interface
	VPNShoot            vpnshoot.Interface
}

// Addons contains references for the addons.
type Addons struct {
	KubernetesDashboard kubernetesdashboard.Interface
	NginxIngress        component.Deployer
}

// Networks contains pre-calculated subnets and IP address for various components.
type Networks struct {
	// Pods subnets
	Pods []net.IPNet
	// Services subnets
	Services []net.IPNet
	// Nodes subnets
	Nodes []net.IPNet
	// APIServer are the ClusterIPs of default/kubernetes Service
	APIServer []net.IP
	// CoreDNS are the ClusterIPs of kube-system/coredns Service
	CoreDNS []net.IP
}
