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
	"fmt"
	"net"
	"strconv"

	"github.com/Masterminds/semver"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/vpnseedserver"
	gardenerextensions "github.com/gardener/gardener/pkg/extensions"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	gardenlethelper "github.com/gardener/gardener/pkg/gardenlet/apis/config/helper"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// NewBuilder returns a new Builder.
func NewBuilder() *Builder {
	return &Builder{
		shootObjectFunc: func(context.Context) (*gardencorev1beta1.Shoot, error) {
			return nil, fmt.Errorf("shoot object is required but not set")
		},
		cloudProfileFunc: func(context.Context, string) (*gardencorev1beta1.CloudProfile, error) {
			return nil, fmt.Errorf("cloudprofile object is required but not set")
		},
		shootSecretFunc: func(context.Context, string, string) (*corev1.Secret, error) {
			return nil, fmt.Errorf("shoot secret object is required but not set")
		},
	}
}

// WithShootObject sets the shootObjectFunc attribute at the Builder.
func (b *Builder) WithShootObject(shootObject *gardencorev1beta1.Shoot) *Builder {
	b.shootObjectFunc = func(context.Context) (*gardencorev1beta1.Shoot, error) { return shootObject, nil }
	return b
}

// WithShootObjectFromCluster sets the shootObjectFunc attribute at the Builder.
func (b *Builder) WithShootObjectFromCluster(seedClient kubernetes.Interface, shootNamespace string) *Builder {
	b.shootObjectFunc = func(ctx context.Context) (*gardencorev1beta1.Shoot, error) {
		cluster, err := gardenerextensions.GetCluster(ctx, seedClient.Client(), shootNamespace)
		if err != nil {
			return nil, err
		}
		return cluster.Shoot, err
	}
	return b
}

// WithCloudProfileObject sets the cloudProfileFunc attribute at the Builder.
func (b *Builder) WithCloudProfileObject(cloudProfileObject *gardencorev1beta1.CloudProfile) *Builder {
	b.cloudProfileFunc = func(context.Context, string) (*gardencorev1beta1.CloudProfile, error) { return cloudProfileObject, nil }
	return b
}

// WithCloudProfileObjectFrom sets the cloudProfileFunc attribute at the Builder after fetching it from the
// given reader.
func (b *Builder) WithCloudProfileObjectFrom(reader client.Reader) *Builder {
	b.cloudProfileFunc = func(ctx context.Context, name string) (*gardencorev1beta1.CloudProfile, error) {
		obj := &gardencorev1beta1.CloudProfile{}
		return obj, reader.Get(ctx, kubernetesutils.Key(name), obj)
	}
	return b
}

// WithCloudProfileObjectFromCluster sets the cloudProfileFunc attribute at the Builder.
func (b *Builder) WithCloudProfileObjectFromCluster(seedClient kubernetes.Interface, shootNamespace string) *Builder {
	b.cloudProfileFunc = func(ctx context.Context, _ string) (*gardencorev1beta1.CloudProfile, error) {
		cluster, err := gardenerextensions.GetCluster(ctx, seedClient.Client(), shootNamespace)
		if err != nil {
			return nil, err
		}
		return cluster.CloudProfile, err
	}
	return b
}

// WithSeedObject sets the seed attribute at the Builder.
func (b *Builder) WithSeedObject(seed *gardencorev1beta1.Seed) *Builder {
	b.seed = seed
	return b
}

// WithShootSecret sets the shootSecretFunc attribute at the Builder.
func (b *Builder) WithShootSecret(secret *corev1.Secret) *Builder {
	b.shootSecretFunc = func(context.Context, string, string) (*corev1.Secret, error) { return secret, nil }
	return b
}

// WithShootSecretFrom sets the shootSecretFunc attribute at the Builder after fetching it from the given reader.
func (b *Builder) WithShootSecretFrom(c client.Reader) *Builder {
	b.shootSecretFunc = func(ctx context.Context, namespace, secretBindingName string) (*corev1.Secret, error) {
		binding := &gardencorev1beta1.SecretBinding{}
		if err := c.Get(ctx, kubernetesutils.Key(namespace, secretBindingName), binding); err != nil {
			return nil, err
		}

		secret := &corev1.Secret{}
		if err := c.Get(ctx, kubernetesutils.Key(binding.SecretRef.Namespace, binding.SecretRef.Name), secret); err != nil {
			return nil, err
		}

		return secret, nil
	}
	return b
}

// WithProjectName sets the projectName attribute at the Builder.
func (b *Builder) WithProjectName(projectName string) *Builder {
	b.projectName = projectName
	return b
}

// WithInternalDomain sets the internalDomain attribute at the Builder.
func (b *Builder) WithInternalDomain(internalDomain *gardenerutils.Domain) *Builder {
	b.internalDomain = internalDomain
	return b
}

// WithDefaultDomains sets the defaultDomains attribute at the Builder.
func (b *Builder) WithDefaultDomains(defaultDomains []*gardenerutils.Domain) *Builder {
	b.defaultDomains = defaultDomains
	return b
}

// Build initializes a new Shoot object.
func (b *Builder) Build(ctx context.Context, c client.Reader) (*Shoot, error) {
	shoot := &Shoot{}

	shootObject, err := b.shootObjectFunc(ctx)
	if err != nil {
		return nil, err
	}
	shoot.SetInfo(shootObject)

	cloudProfile, err := b.cloudProfileFunc(ctx, shootObject.Spec.CloudProfileName)
	if err != nil {
		return nil, err
	}
	shoot.CloudProfile = cloudProfile

	if shootObject.Spec.SecretBindingName != nil {
		secret, err := b.shootSecretFunc(ctx, shootObject.Namespace, *shootObject.Spec.SecretBindingName)
		if err != nil {
			return nil, err
		}
		shoot.Secret = secret
	}

	shoot.HibernationEnabled = v1beta1helper.HibernationIsEnabled(shootObject)
	shoot.SeedNamespace = gardenerutils.ComputeTechnicalID(b.projectName, shootObject)
	shoot.InternalClusterDomain = gardenerutils.ConstructInternalClusterDomain(shootObject.Name, b.projectName, b.internalDomain)
	shoot.ExternalClusterDomain = gardenerutils.ConstructExternalClusterDomain(shootObject)
	shoot.IgnoreAlerts = v1beta1helper.ShootIgnoresAlerts(shootObject)
	shoot.WantsAlertmanager = v1beta1helper.ShootWantsAlertManager(shootObject)
	shoot.WantsVerticalPodAutoscaler = v1beta1helper.ShootWantsVerticalPodAutoscaler(shootObject)
	shoot.Components = &Components{
		Extensions:       &Extensions{},
		ControlPlane:     &ControlPlane{},
		SystemComponents: &SystemComponents{},
		Logging:          &Logging{},
		Monitoring:       &Monitoring{},
		Addons:           &Addons{},
	}

	// Determine information about external domain for shoot cluster.
	externalDomain, err := gardenerutils.ConstructExternalDomain(ctx, c, shootObject, shoot.Secret, b.defaultDomains)
	if err != nil {
		return nil, err
	}
	shoot.ExternalDomain = externalDomain

	// Store the Kubernetes version in the format <major>.<minor> on the Shoot object.
	kubernetesVersion, err := semver.NewVersion(shootObject.Spec.Kubernetes.Version)
	if err != nil {
		return nil, err
	}
	shoot.KubernetesVersion = kubernetesVersion

	gardenerVersion, err := semver.NewVersion(shootObject.Status.Gardener.Version)
	if err != nil {
		return nil, err
	}
	shoot.GardenerVersion = gardenerVersion

	shoot.IsWorkerless = v1beta1helper.IsWorkerless(shoot.GetInfo())

	shoot.VPNHighAvailabilityEnabled = v1beta1helper.IsHAControlPlaneConfigured(shoot.GetInfo())
	if haVPNEnabled, err := strconv.ParseBool(shoot.GetInfo().GetAnnotations()[v1beta1constants.ShootAlphaControlPlaneHAVPN]); err == nil {
		shoot.VPNHighAvailabilityEnabled = haVPNEnabled
	}
	shoot.VPNHighAvailabilityNumberOfSeedServers = vpnseedserver.HighAvailabilityReplicaCount
	shoot.VPNHighAvailabilityNumberOfShootClients = vpnseedserver.HighAvailabilityReplicaCount

	needsClusterAutoscaler, err := v1beta1helper.ShootWantsClusterAutoscaler(shootObject)
	if err != nil {
		return nil, err
	}
	shoot.WantsClusterAutoscaler = needsClusterAutoscaler

	if shootObject.Spec.Networking != nil {
		networks, err := ToNetworks(shootObject, shoot.IsWorkerless)
		if err != nil {
			return nil, err
		}
		shoot.Networks = networks
	}

	shoot.NodeLocalDNSEnabled = v1beta1helper.IsNodeLocalDNSEnabled(shoot.GetInfo().Spec.SystemComponents)
	shoot.Purpose = v1beta1helper.GetPurpose(shootObject)

	shoot.PSPDisabled = v1beta1helper.IsPSPDisabled(shoot.GetInfo())

	if b.seed == nil {
		return nil, fmt.Errorf("seed object is required but not set")
	}
	shoot.TopologyAwareRoutingEnabled = v1beta1helper.IsTopologyAwareRoutingForShootControlPlaneEnabled(b.seed, shootObject)

	backupEntryName, err := gardenerutils.GenerateBackupEntryName(shootObject.Status.TechnicalID, shootObject.UID)
	if err != nil {
		return nil, err
	}
	shoot.BackupEntryName = backupEntryName

	shoot.CloudConfigExecutionMaxDelaySeconds = 300
	if v, ok := shootObject.Annotations[v1beta1constants.AnnotationShootCloudConfigExecutionMaxDelaySeconds]; ok {
		seconds, err := strconv.Atoi(v)
		if err != nil {
			return nil, err
		}

		if seconds <= 1800 {
			shoot.CloudConfigExecutionMaxDelaySeconds = seconds
		}
	}

	if lastOperation := shootObject.Status.LastOperation; lastOperation != nil &&
		lastOperation.Type == gardencorev1beta1.LastOperationTypeRestore &&
		lastOperation.State != gardencorev1beta1.LastOperationStateSucceeded {
		shootState := &gardencorev1beta1.ShootState{ObjectMeta: metav1.ObjectMeta{Name: shootObject.Name, Namespace: shootObject.Namespace}}
		if err := c.Get(ctx, client.ObjectKeyFromObject(shootState), shootState); err != nil {
			return nil, err
		}
		shoot.SetShootState(shootState)
	}

	return shoot, nil
}

// GetInfo returns the shoot resource of this Shoot in a concurrency safe way.
// This method should be used only for reading the data of the returned shoot resource. The returned shoot
// resource MUST NOT BE MODIFIED (except in test code) since this might interfere with other concurrent reads and writes.
// To properly update the shoot resource of this Shoot use UpdateInfo or UpdateInfoStatus.
func (s *Shoot) GetInfo() *gardencorev1beta1.Shoot {
	return s.info.Load().(*gardencorev1beta1.Shoot)
}

// SetInfo sets the shoot resource of this Shoot in a concurrency safe way.
// This method is not protected by a mutex and does not update the shoot resource in the cluster and so
// should be used only in exceptional situations, or as a convenience in test code. The shoot passed as a parameter
// MUST NOT BE MODIFIED after the call to SetInfo (except in test code) since this might interfere with other concurrent reads and writes.
// To properly update the shoot resource of this Shoot use UpdateInfo or UpdateInfoStatus.
func (s *Shoot) SetInfo(shoot *gardencorev1beta1.Shoot) {
	s.info.Store(shoot)
}

// GetShootState returns the shootstate resource of this Shoot in a concurrency safe way.
// This method should be used only for reading the data of the returned shootstate resource. The returned shootstate
// resource MUST NOT BE MODIFIED (except in test code) since this might interfere with other concurrent reads and writes.
// To properly update the shootstate resource of this Shoot use SaveGardenerResourceDataInShootState.
func (s *Shoot) GetShootState() *gardencorev1beta1.ShootState {
	shootState, ok := s.shootState.Load().(*gardencorev1beta1.ShootState)
	if !ok {
		return nil
	}
	return shootState
}

// SetShootState sets the shootstate resource of this Shoot in a concurrency safe way.
// This method is not protected by a mutex and does not update the shootstate resource in the cluster and so
// should be used only in exceptional situations, or as a convenience in test code. The shootstate passed as a parameter
// MUST NOT BE MODIFIED after the call to SetShootState (except in test code) since this might interfere with other concurrent reads and writes.
// To properly update the shootstate resource of this Shoot use SaveGardenerResourceDataInShootState.
func (s *Shoot) SetShootState(shootState *gardencorev1beta1.ShootState) {
	s.shootState.Store(shootState)
}

// UpdateInfo updates the shoot resource of this Shoot in a concurrency safe way,
// using the given context, client, and mutate function.
// It copies the current shoot resource and then uses the copy to patch the resource in the cluster
// using either client.MergeFrom or client.StrategicMergeFrom depending on useStrategicMerge.
// This method is protected by a mutex, so only a single UpdateInfo or UpdateInfoStatus operation can be
// executed at any point in time.
func (s *Shoot) UpdateInfo(ctx context.Context, c client.Client, useStrategicMerge bool, f func(*gardencorev1beta1.Shoot) error) error {
	s.infoMutex.Lock()
	defer s.infoMutex.Unlock()

	shoot := s.info.Load().(*gardencorev1beta1.Shoot).DeepCopy()
	var patch client.Patch
	if useStrategicMerge {
		patch = client.StrategicMergeFrom(shoot.DeepCopy())
	} else {
		patch = client.MergeFrom(shoot.DeepCopy())
	}
	if err := f(shoot); err != nil {
		return err
	}
	if err := c.Patch(ctx, shoot, patch); err != nil {
		return err
	}
	s.info.Store(shoot)
	return nil
}

// UpdateInfoStatus updates the status of the shoot resource of this Shoot in a concurrency safe way,
// using the given context, client, and mutate function.
// It copies the current shoot resource and then uses the copy to patch the resource in the cluster
// using either client.MergeFrom or client.StrategicMergeFrom depending on useStrategicMerge.
// This method is protected by a mutex, so only a single UpdateInfo or UpdateInfoStatus operation can be
// executed at any point in time.
func (s *Shoot) UpdateInfoStatus(ctx context.Context, c client.Client, useStrategicMerge bool, f func(*gardencorev1beta1.Shoot) error) error {
	s.infoMutex.Lock()
	defer s.infoMutex.Unlock()

	shoot := s.info.Load().(*gardencorev1beta1.Shoot).DeepCopy()
	var patch client.Patch
	if useStrategicMerge {
		patch = client.StrategicMergeFrom(shoot.DeepCopy())
	} else {
		patch = client.MergeFrom(shoot.DeepCopy())
	}
	if err := f(shoot); err != nil {
		return err
	}
	if err := c.Status().Patch(ctx, shoot, patch); err != nil {
		return err
	}
	s.info.Store(shoot)
	return nil
}

// GetExtensionComponentsForParallelMigration returns a list of component.DeployMigrateWaiters of
// extension components that should be migrated by the shoot controller in parallel.
// Note that this method does not return ControlPlane and Infrastructure components as they require specific handling during migration.
func (s *Shoot) GetExtensionComponentsForParallelMigration() []component.DeployMigrateWaiter {
	if s.IsWorkerless {
		return []component.DeployMigrateWaiter{}
	}

	return []component.DeployMigrateWaiter{
		s.Components.Extensions.ContainerRuntime,
		s.Components.Extensions.ControlPlaneExposure,
		s.Components.Extensions.Network,
		s.Components.Extensions.OperatingSystemConfig,
		s.Components.Extensions.Worker,
	}
}

// GetDNSRecordComponentsForMigration returns a list of component.DeployMigrateWaiters of DNSRecord components that
// should be migrated by the shoot controller.
func (s *Shoot) GetDNSRecordComponentsForMigration() []component.DeployMigrateWaiter {
	return []component.DeployMigrateWaiter{
		s.Components.Extensions.IngressDNSRecord,
		s.Components.Extensions.ExternalDNSRecord,
		s.Components.Extensions.InternalDNSRecord,
	}
}

// GetIngressFQDN returns the fully qualified domain name of ingress sub-resource for the Shoot cluster. The
// end result is '<subDomain>.<ingressPrefix>.<clusterDomain>'.
func (s *Shoot) GetIngressFQDN(subDomain string) string {
	shoot := s.GetInfo()
	if shoot.Spec.DNS == nil || shoot.Spec.DNS.Domain == nil {
		return ""
	}
	return fmt.Sprintf("%s.%s.%s", subDomain, gardenerutils.IngressPrefix, *shoot.Spec.DNS.Domain)
}

// GetWorkerNames returns a list of names of the worker groups in the Shoot manifest.
func (s *Shoot) GetWorkerNames() []string {
	var workerNames []string
	for _, worker := range s.GetInfo().Spec.Provider.Workers {
		workerNames = append(workerNames, worker.Name)
	}
	return workerNames
}

// GetMinNodeCount returns the sum of all 'minimum' fields of all worker groups of the Shoot.
func (s *Shoot) GetMinNodeCount() int32 {
	var nodeCount int32
	for _, worker := range s.GetInfo().Spec.Provider.Workers {
		nodeCount += worker.Minimum
	}
	return nodeCount
}

// GetMaxNodeCount returns the sum of all 'maximum' fields of all worker groups of the Shoot.
func (s *Shoot) GetMaxNodeCount() int32 {
	var nodeCount int32
	for _, worker := range s.GetInfo().Spec.Provider.Workers {
		nodeCount += worker.Maximum
	}
	return nodeCount
}

// GetReplicas returns the given <wokenUp> number if the shoot is not hibernated, or zero otherwise.
func (s *Shoot) GetReplicas(wokenUp int32) int32 {
	if s.HibernationEnabled {
		return 0
	}
	return wokenUp
}

// ComputeInClusterAPIServerAddress returns the internal address for the shoot API server depending on whether
// the caller runs in the shoot namespace or not.
func (s *Shoot) ComputeInClusterAPIServerAddress(runsInShootNamespace bool) string {
	url := v1beta1constants.DeploymentNameKubeAPIServer
	if !runsInShootNamespace {
		url = fmt.Sprintf("%s.%s.svc", url, s.SeedNamespace)
	}
	return url
}

// ComputeOutOfClusterAPIServerAddress returns the external address for the shoot API server depending on whether
// the caller wants to use the internal cluster domain and whether DNS is disabled on this seed.
func (s *Shoot) ComputeOutOfClusterAPIServerAddress(apiServerAddress string, useInternalClusterDomain bool) string {
	if v1beta1helper.ShootUsesUnmanagedDNS(s.GetInfo()) {
		return gardenerutils.GetAPIServerDomain(s.InternalClusterDomain)
	}

	if useInternalClusterDomain {
		return gardenerutils.GetAPIServerDomain(s.InternalClusterDomain)
	}

	return gardenerutils.GetAPIServerDomain(*s.ExternalClusterDomain)
}

// IPVSEnabled returns true if IPVS is enabled for the shoot.
func (s *Shoot) IPVSEnabled() bool {
	shoot := s.GetInfo()
	return shoot.Spec.Kubernetes.KubeProxy != nil &&
		shoot.Spec.Kubernetes.KubeProxy.Mode != nil &&
		*shoot.Spec.Kubernetes.KubeProxy.Mode == gardencorev1beta1.ProxyModeIPVS
}

// IsShootControlPlaneLoggingEnabled return true if the Shoot controlplane logging is enabled
func (s *Shoot) IsShootControlPlaneLoggingEnabled(c *config.GardenletConfiguration) bool {
	return s.Purpose != gardencorev1beta1.ShootPurposeTesting && gardenlethelper.IsLoggingEnabled(c)
}

// ToNetworks return a network with computed cidrs and ClusterIPs
// for a Shoot
func ToNetworks(s *gardencorev1beta1.Shoot, workerless bool) (*Networks, error) {
	var (
		svc, pods *net.IPNet
		err       error
	)

	if s.Spec.Networking.Pods != nil {
		_, pods, err = net.ParseCIDR(*s.Spec.Networking.Pods)
		if err != nil {
			return nil, fmt.Errorf("cannot parse shoot's network cidr %w", err)
		}
	} else if !workerless {
		return nil, fmt.Errorf("shoot's pods cidr is empty")
	}

	if s.Spec.Networking.Services != nil {
		_, svc, err = net.ParseCIDR(*s.Spec.Networking.Services)
		if err != nil {
			return nil, fmt.Errorf("cannot parse shoot's network cidr %w", err)
		}
	} else {
		return nil, fmt.Errorf("shoot's service cidr is empty")
	}

	apiserver, err := utils.ComputeOffsetIP(svc, 1)
	if err != nil {
		return nil, fmt.Errorf("cannot calculate default/kubernetes ClusterIP: %w", err)
	}

	coreDNS, err := utils.ComputeOffsetIP(svc, 10)
	if err != nil {
		return nil, fmt.Errorf("cannot calculate CoreDNS ClusterIP: %w", err)
	}

	return &Networks{
		CoreDNS:   coreDNS,
		Pods:      pods,
		Services:  svc,
		APIServer: apiserver,
	}, nil
}
