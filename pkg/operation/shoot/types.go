// Copyright 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package shoot

import (
	"context"
	"net"
	"sync"
	"sync/atomic"

	"github.com/Masterminds/semver"
	corev1 "k8s.io/api/core/v1"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/apiserverproxy"
	"github.com/gardener/gardener/pkg/component/backupentry"
	"github.com/gardener/gardener/pkg/component/blackboxexporter"
	"github.com/gardener/gardener/pkg/component/clusterautoscaler"
	"github.com/gardener/gardener/pkg/component/clusteridentity"
	"github.com/gardener/gardener/pkg/component/coredns"
	"github.com/gardener/gardener/pkg/component/etcd"
	"github.com/gardener/gardener/pkg/component/etcdcopybackupstask"
	"github.com/gardener/gardener/pkg/component/extensions/containerruntime"
	"github.com/gardener/gardener/pkg/component/extensions/controlplane"
	"github.com/gardener/gardener/pkg/component/extensions/dnsrecord"
	"github.com/gardener/gardener/pkg/component/extensions/extension"
	"github.com/gardener/gardener/pkg/component/extensions/infrastructure"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig"
	"github.com/gardener/gardener/pkg/component/extensions/worker"
	"github.com/gardener/gardener/pkg/component/kubeapiserver"
	"github.com/gardener/gardener/pkg/component/kubecontrollermanager"
	"github.com/gardener/gardener/pkg/component/kubeproxy"
	"github.com/gardener/gardener/pkg/component/kubernetesdashboard"
	"github.com/gardener/gardener/pkg/component/kubescheduler"
	"github.com/gardener/gardener/pkg/component/kubestatemetrics"
	"github.com/gardener/gardener/pkg/component/machinecontrollermanager"
	"github.com/gardener/gardener/pkg/component/nodelocaldns"
	"github.com/gardener/gardener/pkg/component/resourcemanager"
	"github.com/gardener/gardener/pkg/component/vpa"
	"github.com/gardener/gardener/pkg/component/vpnseedserver"
	"github.com/gardener/gardener/pkg/component/vpnshoot"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

// Builder is an object that builds Shoot objects.
type Builder struct {
	shootObjectFunc  func(context.Context) (*gardencorev1beta1.Shoot, error)
	cloudProfileFunc func(context.Context, string) (*gardencorev1beta1.CloudProfile, error)
	shootSecretFunc  func(context.Context, string, string) (*corev1.Secret, error)
	seed             *gardencorev1beta1.Seed
	projectName      string
	internalDomain   *gardenerutils.Domain
	defaultDomains   []*gardenerutils.Domain
}

// Shoot is an object containing information about a Shoot cluster.
type Shoot struct {
	info      atomic.Value
	infoMutex sync.Mutex

	Secret       *corev1.Secret
	CloudProfile *gardencorev1beta1.CloudProfile

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
	PSPDisabled                             bool
	TopologyAwareRoutingEnabled             bool
	Networks                                *Networks
	BackupEntryName                         string
	CloudConfigExecutionMaxDelaySeconds     int

	Components *Components
}

// Components contains different components deployed in the Shoot cluster.
type Components struct {
	BackupEntry              backupentry.Interface
	SourceBackupEntry        backupentry.Interface
	ControlPlane             *ControlPlane
	Extensions               *Extensions
	SystemComponents         *SystemComponents
	Logging                  *Logging
	GardenerAccess           component.Deployer
	DependencyWatchdogAccess component.Deployer
	Addons                   *Addons
}

// ControlPlane contains references to K8S control plane components.
type ControlPlane struct {
	ClusterAutoscaler        clusterautoscaler.Interface
	EtcdMain                 etcd.Interface
	EtcdEvents               etcd.Interface
	EtcdCopyBackupsTask      etcdcopybackupstask.Interface
	KubeAPIServerIngress     component.Deployer
	KubeAPIServerService     component.DeployWaiter
	KubeAPIServerSNI         component.DeployWaiter
	KubeAPIServerSNIPhase    component.Phase
	KubeAPIServer            kubeapiserver.Interface
	KubeScheduler            kubescheduler.Interface
	KubeControllerManager    kubecontrollermanager.Interface
	KubeStateMetrics         kubestatemetrics.Interface
	MachineControllerManager machinecontrollermanager.Interface
	ResourceManager          resourcemanager.Interface
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
	OwnerDNSRecord        dnsrecord.Interface
	Extension             extension.Interface
	Infrastructure        infrastructure.Interface
	Network               component.DeployMigrateWaiter
	OperatingSystemConfig operatingsystemconfig.Interface
	Worker                worker.Interface
}

// SystemComponents contains references to system components.
type SystemComponents struct {
	APIServerProxy      apiserverproxy.Interface
	BlackboxExporter    blackboxexporter.Interface
	ClusterIdentity     clusteridentity.Interface
	CoreDNS             coredns.Interface
	KubeProxy           kubeproxy.Interface
	MetricsServer       component.DeployWaiter
	Namespaces          component.DeployWaiter
	NodeLocalDNS        nodelocaldns.Interface
	NodeProblemDetector component.DeployWaiter
	NodeExporter        component.DeployWaiter
	Resources           component.DeployWaiter
	VPNShoot            vpnshoot.Interface
}

// Logging contains references to logging deployers
type Logging struct {
	ShootRBACProxy   component.Deployer
	ShootEventLogger component.Deployer
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
