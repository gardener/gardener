// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/operation/botanist/component/backupentry"
	"github.com/gardener/gardener/pkg/operation/botanist/component/clusterautoscaler"
	"github.com/gardener/gardener/pkg/operation/botanist/component/clusteridentity"
	"github.com/gardener/gardener/pkg/operation/botanist/component/coredns"
	"github.com/gardener/gardener/pkg/operation/botanist/component/etcd"
	"github.com/gardener/gardener/pkg/operation/botanist/component/etcdcopybackupstask"
	"github.com/gardener/gardener/pkg/operation/botanist/component/extensions/containerruntime"
	"github.com/gardener/gardener/pkg/operation/botanist/component/extensions/controlplane"
	"github.com/gardener/gardener/pkg/operation/botanist/component/extensions/dnsrecord"
	"github.com/gardener/gardener/pkg/operation/botanist/component/extensions/extension"
	"github.com/gardener/gardener/pkg/operation/botanist/component/extensions/infrastructure"
	"github.com/gardener/gardener/pkg/operation/botanist/component/extensions/operatingsystemconfig"
	"github.com/gardener/gardener/pkg/operation/botanist/component/extensions/worker"
	"github.com/gardener/gardener/pkg/operation/botanist/component/kubeapiserver"
	"github.com/gardener/gardener/pkg/operation/botanist/component/kubecontrollermanager"
	"github.com/gardener/gardener/pkg/operation/botanist/component/kubeproxy"
	"github.com/gardener/gardener/pkg/operation/botanist/component/kubescheduler"
	"github.com/gardener/gardener/pkg/operation/botanist/component/kubestatemetrics"
	"github.com/gardener/gardener/pkg/operation/botanist/component/nodelocaldns"
	"github.com/gardener/gardener/pkg/operation/botanist/component/resourcemanager"
	"github.com/gardener/gardener/pkg/operation/botanist/component/vpa"
	"github.com/gardener/gardener/pkg/operation/botanist/component/vpnseedserver"
	"github.com/gardener/gardener/pkg/operation/botanist/component/vpnshoot"
	"github.com/gardener/gardener/pkg/operation/garden"

	"github.com/Masterminds/semver"
	corev1 "k8s.io/api/core/v1"
)

// Builder is an object that builds Shoot objects.
type Builder struct {
	shootObjectFunc   func(context.Context) (*gardencorev1beta1.Shoot, error)
	cloudProfileFunc  func(context.Context, string) (*gardencorev1beta1.CloudProfile, error)
	exposureClassFunc func(context.Context, string) (*gardencorev1alpha1.ExposureClass, error)
	shootSecretFunc   func(context.Context, string, string) (*corev1.Secret, error)
	projectName       string
	internalDomain    *garden.Domain
	defaultDomains    []*garden.Domain
	disableDNS        bool
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

	DisableDNS            bool
	InternalClusterDomain string
	ExternalClusterDomain *string
	ExternalDomain        *garden.Domain

	Purpose                             gardencorev1beta1.ShootPurpose
	WantsClusterAutoscaler              bool
	WantsVerticalPodAutoscaler          bool
	WantsAlertmanager                   bool
	IgnoreAlerts                        bool
	HibernationEnabled                  bool
	ReversedVPNEnabled                  bool
	NodeLocalDNSEnabled                 bool
	PSPDisabled                         bool
	Networks                            *Networks
	ExposureClass                       *gardencorev1alpha1.ExposureClass
	BackupEntryName                     string
	CloudConfigExecutionMaxDelaySeconds int

	Components *Components
}

// Components contains different components deployed in the Shoot cluster.
type Components struct {
	BackupEntry              backupentry.Interface
	SourceBackupEntry        backupentry.Interface
	ControlPlane             *ControlPlane
	Extensions               *Extensions
	NetworkPolicies          component.Deployer
	SystemComponents         *SystemComponents
	Logging                  *Logging
	GardenerAccess           component.Deployer
	DependencyWatchdogAccess component.Deployer
	HVPA                     component.MonitoringComponent
}

// ControlPlane contains references to K8S control plane components.
type ControlPlane struct {
	ClusterAutoscaler     clusterautoscaler.Interface
	EtcdMain              etcd.Interface
	EtcdEvents            etcd.Interface
	EtcdCopyBackupsTask   etcdcopybackupstask.Interface
	KubeAPIServerService  component.DeployWaiter
	KubeAPIServerSNI      component.DeployWaiter
	KubeAPIServerSNIPhase component.Phase
	KubeAPIServer         kubeapiserver.Interface
	KubeScheduler         kubescheduler.Interface
	KubeControllerManager kubecontrollermanager.Interface
	KubeStateMetrics      kubestatemetrics.Interface
	ResourceManager       resourcemanager.Interface
	VerticalPodAutoscaler vpa.Interface
	VPNSeedServer         vpnseedserver.Interface
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
	ClusterIdentity     clusteridentity.Interface
	CoreDNS             coredns.Interface
	KubeProxy           kubeproxy.Interface
	MetricsServer       component.DeployWaiter
	Namespaces          component.DeployWaiter
	NodeLocalDNS        nodelocaldns.Interface
	NodeProblemDetector component.DeployWaiter
	Resources           component.DeployWaiter
	VPNShoot            vpnshoot.Interface
}

// Logging contains references to logging deployers
type Logging struct {
	ShootRBACProxy   component.Deployer
	ShootEventLogger component.Deployer
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

// IncompleteDNSConfigError is a custom error type.
type IncompleteDNSConfigError struct{}

// Error prints the error message of the IncompleteDNSConfigError error.
func (e *IncompleteDNSConfigError) Error() string {
	return "unable to figure out which secret should be used for dns"
}

// IsIncompleteDNSConfigError returns true if the error indicates that not the DNS config is incomplete.
func IsIncompleteDNSConfigError(err error) bool {
	_, ok := err.(*IncompleteDNSConfigError)
	return ok
}
