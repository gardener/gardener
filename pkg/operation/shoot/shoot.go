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
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	gardenerextensions "github.com/gardener/gardener/pkg/extensions"
	"github.com/gardener/gardener/pkg/features"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	gardenlethelper "github.com/gardener/gardener/pkg/gardenlet/apis/config/helper"
	gardenletfeatures "github.com/gardener/gardener/pkg/gardenlet/features"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/operation/garden"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/version"

	"github.com/Masterminds/semver"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
		exposureClassFunc: func(context.Context, string) (*gardencorev1alpha1.ExposureClass, error) {
			return nil, nil
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
		return obj, reader.Get(ctx, kutil.Key(name), obj)
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

// WithShootSecret sets the shootSecretFunc attribute at the Builder.
func (b *Builder) WithShootSecret(secret *corev1.Secret) *Builder {
	b.shootSecretFunc = func(context.Context, string, string) (*corev1.Secret, error) { return secret, nil }
	return b
}

// WithShootSecretFrom sets the shootSecretFunc attribute at the Builder after fetching it from the given reader.
func (b *Builder) WithShootSecretFrom(c client.Reader) *Builder {
	b.shootSecretFunc = func(ctx context.Context, namespace, secretBindingName string) (*corev1.Secret, error) {
		binding := &gardencorev1beta1.SecretBinding{}
		if err := c.Get(ctx, kutil.Key(namespace, secretBindingName), binding); err != nil {
			return nil, err
		}

		secret := &corev1.Secret{}
		if err := c.Get(ctx, kutil.Key(binding.SecretRef.Namespace, binding.SecretRef.Name), secret); err != nil {
			return nil, err
		}

		return secret, nil
	}
	return b
}

// WithExposureClassFrom sets the exposureClassFunc attribute at the Builder after fetching
// the exposure class with the given reader.
func (b *Builder) WithExposureClassFrom(c client.Reader) *Builder {
	b.exposureClassFunc = func(ctx context.Context, exposureClassName string) (*gardencorev1alpha1.ExposureClass, error) {
		exposureClass := &gardencorev1alpha1.ExposureClass{}
		if err := c.Get(ctx, kutil.Key(exposureClassName), exposureClass); err != nil {
			return nil, err
		}
		return exposureClass, nil
	}
	return b
}

// WithDisableDNS sets the disableDNS attribute at the Builder.
func (b *Builder) WithDisableDNS(disableDNS bool) *Builder {
	b.disableDNS = disableDNS
	return b
}

// WithProjectName sets the projectName attribute at the Builder.
func (b *Builder) WithProjectName(projectName string) *Builder {
	b.projectName = projectName
	return b
}

// WithInternalDomain sets the internalDomain attribute at the Builder.
func (b *Builder) WithInternalDomain(internalDomain *garden.Domain) *Builder {
	b.internalDomain = internalDomain
	return b
}

// WithDefaultDomains sets the defaultDomains attribute at the Builder.
func (b *Builder) WithDefaultDomains(defaultDomains []*garden.Domain) *Builder {
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

	secret, err := b.shootSecretFunc(ctx, shootObject.Namespace, shootObject.Spec.SecretBindingName)
	if err != nil {
		return nil, err
	}
	shoot.Secret = secret

	if shootObject.Spec.ExposureClassName != nil {
		exposureClass, err := b.exposureClassFunc(ctx, *shootObject.Spec.ExposureClassName)
		if err != nil {
			return nil, err
		}
		shoot.ExposureClass = exposureClass
	}

	shoot.DisableDNS = b.disableDNS
	shoot.HibernationEnabled = gardencorev1beta1helper.HibernationIsEnabled(shootObject)
	shoot.SeedNamespace = ComputeTechnicalID(b.projectName, shootObject)
	shoot.InternalClusterDomain = ConstructInternalClusterDomain(shootObject.Name, b.projectName, b.internalDomain)
	shoot.ExternalClusterDomain = ConstructExternalClusterDomain(shootObject)
	shoot.IgnoreAlerts = gardencorev1beta1helper.ShootIgnoresAlerts(shootObject)
	shoot.WantsAlertmanager = gardencorev1beta1helper.ShootWantsAlertManager(shootObject)
	shoot.WantsVerticalPodAutoscaler = gardencorev1beta1helper.ShootWantsVerticalPodAutoscaler(shootObject)
	shoot.Components = &Components{
		Extensions:       &Extensions{},
		ControlPlane:     &ControlPlane{},
		SystemComponents: &SystemComponents{},
		Logging:          &Logging{},
	}

	// Determine information about external domain for shoot cluster.
	externalDomain, err := ConstructExternalDomain(ctx, c, shootObject, secret, b.defaultDomains)
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

	kubernetesVersionGeq118 := version.ConstraintK8sGreaterEqual118.Check(kubernetesVersion)
	shoot.ReversedVPNEnabled = gardenletfeatures.FeatureGate.Enabled(features.ReversedVPN) && kubernetesVersionGeq118
	if reversedVPNEnabled, err := strconv.ParseBool(shoot.GetInfo().Annotations[v1beta1constants.AnnotationReversedVPN]); err == nil && kubernetesVersionGeq118 {
		if gardenletfeatures.FeatureGate.Enabled(features.APIServerSNI) {
			shoot.ReversedVPNEnabled = reversedVPNEnabled
		}
	}

	needsClusterAutoscaler, err := gardencorev1beta1helper.ShootWantsClusterAutoscaler(shootObject)
	if err != nil {
		return nil, err
	}
	shoot.WantsClusterAutoscaler = needsClusterAutoscaler

	networks, err := ToNetworks(shootObject)
	if err != nil {
		return nil, err
	}
	shoot.Networks = networks

	shoot.NodeLocalDNSEnabled = helper.IsNodeLocalDNSEnabled(shoot.GetInfo().Spec.SystemComponents, shoot.GetInfo().Annotations)
	shoot.Purpose = gardencorev1beta1helper.GetPurpose(shootObject)

	shoot.PSPDisabled = gardencorev1beta1helper.IsPSPDisabled(shoot.GetInfo())

	backupEntryName, err := gutil.GenerateBackupEntryName(shootObject.Status.TechnicalID, shootObject.UID)
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
	return []component.DeployMigrateWaiter{
		s.Components.Extensions.ContainerRuntime,
		s.Components.Extensions.ControlPlaneExposure,
		s.Components.Extensions.Extension,
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
		s.Components.Extensions.OwnerDNSRecord,
	}
}

// GetIngressFQDN returns the fully qualified domain name of ingress sub-resource for the Shoot cluster. The
// end result is '<subDomain>.<ingressPrefix>.<clusterDomain>'.
func (s *Shoot) GetIngressFQDN(subDomain string) string {
	shoot := s.GetInfo()
	if shoot.Spec.DNS == nil || shoot.Spec.DNS.Domain == nil {
		return ""
	}
	return fmt.Sprintf("%s.%s.%s", subDomain, gutil.IngressPrefix, *shoot.Spec.DNS.Domain)
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
	if s.DisableDNS {
		return apiServerAddress
	}

	if gardencorev1beta1helper.ShootUsesUnmanagedDNS(s.GetInfo()) {
		return gutil.GetAPIServerDomain(s.InternalClusterDomain)
	}

	if useInternalClusterDomain {
		return gutil.GetAPIServerDomain(s.InternalClusterDomain)
	}

	return gutil.GetAPIServerDomain(*s.ExternalClusterDomain)
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

// TechnicalIDPrefix is a prefix used for a shoot's technical id.
const TechnicalIDPrefix = "shoot--"

// ComputeTechnicalID determines the technical id of that Shoot which is later used for the name of the
// namespace and for tagging all the resources created in the infrastructure.
func ComputeTechnicalID(projectName string, shoot *gardencorev1beta1.Shoot) string {
	// Use the stored technical ID in the Shoot's status field if it's there.
	// For backwards compatibility we keep the pattern as it was before we had to change it
	// (double hyphens).
	if len(shoot.Status.TechnicalID) > 0 {
		return shoot.Status.TechnicalID
	}

	// New clusters shall be created with the new technical id (double hyphens).
	return fmt.Sprintf("%s%s--%s", TechnicalIDPrefix, projectName, shoot.Name)
}

// ConstructInternalClusterDomain constructs the internal base domain pof this shoot cluster.
// It is only used for internal purposes (all kubeconfigs except the one which is received by the
// user will only talk with the kube-apiserver via a DNS record of domain). In case the given <internalDomain>
// already contains "internal", the result is constructed as "<shootName>.<shootProject>.<internalDomain>."
// In case it does not, the word "internal" will be appended, resulting in
// "<shootName>.<shootProject>.internal.<internalDomain>".
func ConstructInternalClusterDomain(shootName, shootProject string, internalDomain *garden.Domain) string {
	if internalDomain == nil {
		return ""
	}
	if strings.Contains(internalDomain.Domain, gutil.InternalDomainKey) {
		return fmt.Sprintf("%s.%s.%s", shootName, shootProject, internalDomain.Domain)
	}
	return fmt.Sprintf("%s.%s.%s.%s", shootName, shootProject, gutil.InternalDomainKey, internalDomain.Domain)
}

// ConstructExternalClusterDomain constructs the external Shoot cluster domain, i.e. the domain which will be put
// into the Kubeconfig handed out to the user.
func ConstructExternalClusterDomain(shoot *gardencorev1beta1.Shoot) *string {
	if shoot.Spec.DNS == nil || shoot.Spec.DNS.Domain == nil {
		return nil
	}
	return shoot.Spec.DNS.Domain
}

// ConstructExternalDomain constructs an object containing all relevant information of the external domain that
// shall be used for a shoot cluster - based on the configuration of the Garden cluster and the shoot itself.
func ConstructExternalDomain(ctx context.Context, c client.Reader, shoot *gardencorev1beta1.Shoot, shootSecret *corev1.Secret, defaultDomains []*garden.Domain) (*garden.Domain, error) {
	externalClusterDomain := ConstructExternalClusterDomain(shoot)
	if externalClusterDomain == nil {
		return nil, nil
	}

	var (
		externalDomain  = &garden.Domain{Domain: *shoot.Spec.DNS.Domain}
		defaultDomain   = garden.DomainIsDefaultDomain(*externalClusterDomain, defaultDomains)
		primaryProvider = gardencorev1beta1helper.FindPrimaryDNSProvider(shoot.Spec.DNS.Providers)
	)

	switch {
	case defaultDomain != nil:
		externalDomain.SecretData = defaultDomain.SecretData
		externalDomain.Provider = defaultDomain.Provider
		externalDomain.Zone = defaultDomain.Zone
		externalDomain.IncludeDomains = defaultDomain.IncludeDomains
		externalDomain.ExcludeDomains = defaultDomain.ExcludeDomains
		externalDomain.IncludeZones = defaultDomain.IncludeZones
		externalDomain.ExcludeZones = defaultDomain.ExcludeZones

	case primaryProvider != nil:
		if primaryProvider.SecretName != nil {
			secret := &corev1.Secret{}
			if err := c.Get(ctx, kutil.Key(shoot.Namespace, *primaryProvider.SecretName), secret); err != nil {
				return nil, fmt.Errorf("could not get dns provider secret %q: %+v", *shoot.Spec.DNS.Providers[0].SecretName, err)
			}
			externalDomain.SecretData = secret.Data
		} else {
			externalDomain.SecretData = shootSecret.Data
		}
		if primaryProvider.Type != nil {
			externalDomain.Provider = *primaryProvider.Type
		}
		if domains := primaryProvider.Domains; domains != nil {
			externalDomain.IncludeDomains = domains.Include
			externalDomain.ExcludeDomains = domains.Exclude
		}
		if zones := primaryProvider.Zones; zones != nil {
			externalDomain.IncludeZones = zones.Include
			externalDomain.ExcludeZones = zones.Exclude
			if len(zones.Include) == 1 {
				externalDomain.Zone = zones.Include[0]
			}
		}

	default:
		return nil, &IncompleteDNSConfigError{}
	}

	return externalDomain, nil
}

// ToNetworks return a network with computed cidrs and ClusterIPs
// for a Shoot
func ToNetworks(s *gardencorev1beta1.Shoot) (*Networks, error) {
	if s.Spec.Networking.Services == nil {
		return nil, fmt.Errorf("shoot's service cidr is empty")
	}

	if s.Spec.Networking.Pods == nil {
		return nil, fmt.Errorf("shoot's pods cidr is empty")
	}

	_, svc, err := net.ParseCIDR(*s.Spec.Networking.Services)
	if err != nil {
		return nil, fmt.Errorf("cannot parse shoot's network cidr %w", err)
	}

	_, pods, err := net.ParseCIDR(*s.Spec.Networking.Pods)
	if err != nil {
		return nil, fmt.Errorf("cannot parse shoot's network cidr %w", err)
	}

	apiserver, err := common.ComputeOffsetIP(svc, 1)
	if err != nil {
		return nil, fmt.Errorf("cannot calculate default/kubernetes ClusterIP: %w", err)
	}

	coreDNS, err := common.ComputeOffsetIP(svc, 10)
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

// ComputeRequiredExtensions compute the extension kind/type combinations that are required for the
// reconciliation flow.
func ComputeRequiredExtensions(shoot *gardencorev1beta1.Shoot, seed *gardencorev1beta1.Seed, controllerRegistrationList *gardencorev1beta1.ControllerRegistrationList, internalDomain, externalDomain *garden.Domain) sets.String {
	requiredExtensions := sets.NewString()

	if seed.Spec.Backup != nil {
		requiredExtensions.Insert(gardenerextensions.Id(extensionsv1alpha1.BackupBucketResource, seed.Spec.Backup.Provider))
		requiredExtensions.Insert(gardenerextensions.Id(extensionsv1alpha1.BackupEntryResource, seed.Spec.Backup.Provider))
	}
	// Hint: This is actually a temporary work-around to request the control plane extension of the seed provider type as
	// it might come with webhooks that are configuring the exposure of shoot control planes. The ControllerRegistration resource
	// does not reflect this today.
	requiredExtensions.Insert(gardenerextensions.Id(extensionsv1alpha1.ControlPlaneResource, seed.Spec.Provider.Type))

	requiredExtensions.Insert(gardenerextensions.Id(extensionsv1alpha1.ControlPlaneResource, shoot.Spec.Provider.Type))
	requiredExtensions.Insert(gardenerextensions.Id(extensionsv1alpha1.InfrastructureResource, shoot.Spec.Provider.Type))
	requiredExtensions.Insert(gardenerextensions.Id(extensionsv1alpha1.NetworkResource, shoot.Spec.Networking.Type))
	requiredExtensions.Insert(gardenerextensions.Id(extensionsv1alpha1.WorkerResource, shoot.Spec.Provider.Type))

	disabledExtensions := sets.NewString()
	for _, extension := range shoot.Spec.Extensions {
		id := gardenerextensions.Id(extensionsv1alpha1.ExtensionResource, extension.Type)

		if pointer.BoolDeref(extension.Disabled, false) {
			disabledExtensions.Insert(id)
		} else {
			requiredExtensions.Insert(id)
		}
	}

	for _, pool := range shoot.Spec.Provider.Workers {
		if pool.Machine.Image != nil {
			requiredExtensions.Insert(gardenerextensions.Id(extensionsv1alpha1.OperatingSystemConfigResource, pool.Machine.Image.Name))
		}
		if pool.CRI != nil {
			for _, cr := range pool.CRI.ContainerRuntimes {
				requiredExtensions.Insert(gardenerextensions.Id(extensionsv1alpha1.ContainerRuntimeResource, cr.Type))
			}
		}
	}

	if seed.Spec.Settings.ShootDNS.Enabled {
		if shoot.Spec.DNS != nil {
			for _, provider := range shoot.Spec.DNS.Providers {
				if provider.Type != nil && *provider.Type != core.DNSUnmanaged {
					if provider.Primary != nil && *provider.Primary {
						requiredExtensions.Insert(gardenerextensions.Id(extensionsv1alpha1.DNSRecordResource, *provider.Type))
					}
				}
			}
		}

		if internalDomain != nil && internalDomain.Provider != core.DNSUnmanaged {
			requiredExtensions.Insert(gardenerextensions.Id(extensionsv1alpha1.DNSRecordResource, internalDomain.Provider))
		}

		if externalDomain != nil && externalDomain.Provider != core.DNSUnmanaged {
			requiredExtensions.Insert(gardenerextensions.Id(extensionsv1alpha1.DNSRecordResource, externalDomain.Provider))
		}
	}

	for _, controllerRegistration := range controllerRegistrationList.Items {
		for _, resource := range controllerRegistration.Spec.Resources {
			id := gardenerextensions.Id(extensionsv1alpha1.ExtensionResource, resource.Type)
			if resource.Kind == extensionsv1alpha1.ExtensionResource && resource.GloballyEnabled != nil && *resource.GloballyEnabled && !disabledExtensions.Has(id) {
				requiredExtensions.Insert(id)
			}
		}
	}

	return requiredExtensions
}
