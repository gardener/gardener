// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shoot

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	vpnseedserver "github.com/gardener/gardener/pkg/component/networking/vpn/seedserver"
	sharedcomponent "github.com/gardener/gardener/pkg/component/shared"
	gardenerextensions "github.com/gardener/gardener/pkg/extensions"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	gardenlethelper "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1/helper"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

// NewBuilder returns a new Builder.
func NewBuilder() *Builder {
	return &Builder{
		shootObjectFunc: func(context.Context) (*gardencorev1beta1.Shoot, error) {
			return nil, fmt.Errorf("shoot object is required but not set")
		},
		cloudProfileFunc: func(context.Context, *gardencorev1beta1.Shoot) (*gardencorev1beta1.CloudProfile, error) {
			return nil, fmt.Errorf("cloudprofile object is required but not set")
		},
		shootCredentialsFunc: func(context.Context, string, string, bool) (client.Object, error) {
			return nil, fmt.Errorf("shoot credentials object is required but not set")
		},
		serviceAccountIssuerHostname: func() (*string, error) {
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
func (b *Builder) WithShootObjectFromCluster(seedClient kubernetes.Interface, clusterName string) *Builder {
	b.shootObjectFunc = func(ctx context.Context) (*gardencorev1beta1.Shoot, error) {
		cluster, err := gardenerextensions.GetCluster(ctx, seedClient.Client(), clusterName)
		if err != nil {
			return nil, err
		}
		return cluster.Shoot, err
	}
	return b
}

// WithCloudProfileObject sets the cloudProfileFunc attribute at the Builder.
func (b *Builder) WithCloudProfileObject(cloudProfileObject *gardencorev1beta1.CloudProfile) *Builder {
	b.cloudProfileFunc = func(context.Context, *gardencorev1beta1.Shoot) (*gardencorev1beta1.CloudProfile, error) {
		return cloudProfileObject, nil
	}
	return b
}

// WithCloudProfileObjectFrom sets the cloudProfileFunc attribute at the Builder after fetching it from the
// given reader.
func (b *Builder) WithCloudProfileObjectFrom(reader client.Reader) *Builder {
	b.cloudProfileFunc = func(ctx context.Context, shoot *gardencorev1beta1.Shoot) (*gardencorev1beta1.CloudProfile, error) {
		return gardenerutils.GetCloudProfile(ctx, reader, shoot)
	}
	return b
}

// WithCloudProfileObjectFromCluster sets the cloudProfileFunc attribute at the Builder.
func (b *Builder) WithCloudProfileObjectFromCluster(seedClient kubernetes.Interface, clusterName string) *Builder {
	b.cloudProfileFunc = func(ctx context.Context, _ *gardencorev1beta1.Shoot) (*gardencorev1beta1.CloudProfile, error) {
		cluster, err := gardenerextensions.GetCluster(ctx, seedClient.Client(), clusterName)
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

// WithExposureClassObject sets the exposureClass attribute at the Builder.
func (b *Builder) WithExposureClassObject(exposureClass *gardencorev1beta1.ExposureClass) *Builder {
	b.exposureClass = exposureClass
	return b
}

// WithShootCredentialsFrom sets the shootCredentialsFunc attribute at the Builder after fetching it from the given reader.
func (b *Builder) WithShootCredentialsFrom(c client.Reader) *Builder {
	b.shootCredentialsFunc = func(ctx context.Context, namespace, bindingName string, fromSecretBinding bool) (client.Object, error) {
		var key types.NamespacedName
		if fromSecretBinding {
			binding := &gardencorev1beta1.SecretBinding{}
			if err := c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: bindingName}, binding); err != nil {
				return nil, err
			}
			key = client.ObjectKey{Namespace: binding.SecretRef.Namespace, Name: binding.SecretRef.Name}
		} else {
			binding := &securityv1alpha1.CredentialsBinding{}
			if err := c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: bindingName}, binding); err != nil {
				return nil, err
			}
			key = client.ObjectKey{Namespace: binding.CredentialsRef.Namespace, Name: binding.CredentialsRef.Name}

			if binding.CredentialsRef.GroupVersionKind() == securityv1alpha1.SchemeGroupVersion.WithKind("WorkloadIdentity") {
				workloadIdentity := &securityv1alpha1.WorkloadIdentity{}
				if err := c.Get(ctx, key, workloadIdentity); err != nil {
					return nil, err
				}
				return workloadIdentity, nil
			}
		}

		secret := &corev1.Secret{}
		if err := c.Get(ctx, key, secret); err != nil {
			return nil, err
		}

		return secret, nil
	}
	return b
}

// WithoutShootCredentials sets the shootCredentialsFunc attribute at the builder to return empty Secret as credentials.
func (b *Builder) WithoutShootCredentials() *Builder {
	b.shootCredentialsFunc = func(context.Context, string, string, bool) (client.Object, error) { return &corev1.Secret{}, nil }
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

// WithServiceAccountIssuerHostname prepares the [Builder] for initialization of the service account issuer hostname.
// Should be called before [Builder.Build].
func (b *Builder) WithServiceAccountIssuerHostname(secret *corev1.Secret) *Builder {
	b.serviceAccountIssuerHostname = func() (*string, error) {
		if secret == nil {
			return nil, nil
		}
		host, ok := secret.Data["hostname"]
		if !ok {
			return nil, errors.New("service account issuer secret is missing a hostname key")
		}
		hostname := string(host)
		if strings.TrimSpace(hostname) == "" {
			return nil, errors.New("service account issuer secret has an empty hostname key")
		}
		return &hostname, nil
	}
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

	cloudProfile, err := b.cloudProfileFunc(ctx, shootObject)
	if err != nil {
		return nil, err
	}
	shoot.CloudProfile = cloudProfile
	shoot.ExposureClass = b.exposureClass

	if shootObject.Spec.SecretBindingName != nil {
		credentials, err := b.shootCredentialsFunc(ctx, shootObject.Namespace, *shootObject.Spec.SecretBindingName, true)
		if err != nil {
			return nil, err
		}
		shoot.Credentials = credentials
	} else if shootObject.Spec.CredentialsBindingName != nil {
		credentials, err := b.shootCredentialsFunc(ctx, shootObject.Namespace, *shootObject.Spec.CredentialsBindingName, false)
		if err != nil {
			return nil, err
		}
		shoot.Credentials = credentials
	}

	shoot.HibernationEnabled = v1beta1helper.HibernationIsEnabled(shootObject)
	shoot.ControlPlaneNamespace = v1beta1helper.ControlPlaneNamespaceForShoot(shootObject)
	shoot.InternalClusterDomain = gardenerutils.ConstructInternalClusterDomain(shootObject.Name, b.projectName, b.internalDomain)
	shoot.ExternalClusterDomain = gardenerutils.ConstructExternalClusterDomain(shootObject)
	shoot.IgnoreAlerts = v1beta1helper.ShootIgnoresAlerts(shootObject)
	shoot.WantsAlertmanager = v1beta1helper.ShootWantsAlertManager(shootObject)
	shoot.WantsVerticalPodAutoscaler = v1beta1helper.ShootWantsVerticalPodAutoscaler(shootObject)
	shoot.Components = &Components{
		Extensions:       &Extensions{},
		ControlPlane:     &ControlPlane{},
		SystemComponents: &SystemComponents{},
		Addons:           &Addons{},
	}

	serviceAccountIssuerHostname, err := b.serviceAccountIssuerHostname()
	if err != nil {
		return nil, err
	}
	shoot.ServiceAccountIssuerHostname = serviceAccountIssuerHostname

	// Determine information about external domain for shoot cluster.
	externalDomain, err := gardenerutils.ConstructExternalDomain(ctx, c, shootObject, shoot.Credentials, b.defaultDomains)
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

	shoot.IsWorkerless = v1beta1helper.IsWorkerless(shoot.GetInfo())

	shoot.VPNHighAvailabilityEnabled = v1beta1helper.IsHAVPNEnabled(shoot.GetInfo())
	shoot.VPNHighAvailabilityNumberOfSeedServers = vpnseedserver.HighAvailabilityReplicaCount
	shoot.VPNHighAvailabilityNumberOfShootClients = vpnseedserver.HighAvailabilityReplicaCount
	if vpnVPAUpdateDisabled, err := strconv.ParseBool(shoot.GetInfo().GetAnnotations()[v1beta1constants.ShootAlphaControlPlaneVPNVPAUpdateDisabled]); err == nil {
		shoot.VPNVPAUpdateDisabled = vpnVPAUpdateDisabled
	}

	needsClusterAutoscaler, err := v1beta1helper.ShootWantsClusterAutoscaler(shootObject)
	if err != nil {
		return nil, err
	}
	shoot.WantsClusterAutoscaler = needsClusterAutoscaler

	if shoot.IsWorkerless && shootObject.Spec.Networking != nil {
		networks, err := ToNetworks(shootObject, shoot.IsWorkerless)
		if err != nil {
			return nil, err
		}
		shoot.Networks = networks
	}

	shoot.NodeLocalDNSEnabled = v1beta1helper.IsNodeLocalDNSEnabled(shoot.GetInfo().Spec.SystemComponents)
	shoot.Purpose = v1beta1helper.GetPurpose(shootObject)

	if shoot.GetInfo().Spec.Kubernetes.KubeAPIServer != nil {
		shoot.ResourcesToEncrypt = sharedcomponent.NormalizeResources(sharedcomponent.GetResourcesForEncryptionFromConfig(shoot.GetInfo().Spec.Kubernetes.KubeAPIServer.EncryptionConfig))
	}
	if len(shoot.GetInfo().Status.EncryptedResources) > 0 {
		shoot.EncryptedResources = sharedcomponent.NormalizeResources(shoot.GetInfo().Status.EncryptedResources)
	}

	if b.seed != nil {
		shoot.TopologyAwareRoutingEnabled = v1beta1helper.IsTopologyAwareRoutingForShootControlPlaneEnabled(b.seed, shootObject)
	}

	backupEntryName, err := gardenerutils.GenerateBackupEntryName(shoot.ControlPlaneNamespace, shootObject.Status.UID, shootObject.UID)
	if err != nil {
		return nil, err
	}
	shoot.BackupEntryName = backupEntryName

	oscSyncJitterPeriod := 300
	if v, ok := shootObject.Annotations[v1beta1constants.AnnotationShootCloudConfigExecutionMaxDelaySeconds]; ok {
		seconds, err := strconv.Atoi(v)
		if err != nil {
			return nil, err
		}

		if seconds <= 1800 {
			oscSyncJitterPeriod = seconds
		}
	}
	shoot.OSCSyncJitterPeriod = &metav1.Duration{Duration: time.Duration(oscSyncJitterPeriod) * time.Second}

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
func (s *Shoot) UpdateInfo(ctx context.Context, c client.Client, useStrategicMerge, mergeWithOptimisticLock bool, f func(*gardencorev1beta1.Shoot) error) error {
	s.infoMutex.Lock()
	defer s.infoMutex.Unlock()

	shoot := s.info.Load().(*gardencorev1beta1.Shoot).DeepCopy()
	var patch client.Patch
	if useStrategicMerge {
		patch = client.StrategicMergeFrom(shoot.DeepCopy())
		if mergeWithOptimisticLock {
			patch = client.StrategicMergeFrom(shoot.DeepCopy(), client.MergeFromWithOptimisticLock{})
		}
	} else {
		patch = client.MergeFrom(shoot.DeepCopy())
		if mergeWithOptimisticLock {
			patch = client.MergeFromWithOptions(shoot.DeepCopy(), client.MergeFromWithOptimisticLock{})
		}
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
func (s *Shoot) UpdateInfoStatus(ctx context.Context, c client.Client, useStrategicMerge, mergeWithOptimisticLock bool, f func(*gardencorev1beta1.Shoot) error) error {
	s.infoMutex.Lock()
	defer s.infoMutex.Unlock()

	shoot := s.info.Load().(*gardencorev1beta1.Shoot).DeepCopy()
	var patch client.Patch
	if useStrategicMerge {
		patch = client.StrategicMergeFrom(shoot.DeepCopy())
		if mergeWithOptimisticLock {
			patch = client.StrategicMergeFrom(shoot.DeepCopy(), client.MergeFromWithOptimisticLock{})
		}
	} else {
		patch = client.MergeFrom(shoot.DeepCopy())
		if mergeWithOptimisticLock {
			patch = client.MergeFromWithOptions(shoot.DeepCopy(), client.MergeFromWithOptimisticLock{})
		}
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
	if s.RunsControlPlane() {
		return fmt.Sprintf("kubernetes.%s.svc", metav1.NamespaceDefault)
	}

	url := v1beta1constants.DeploymentNameKubeAPIServer
	if !runsInShootNamespace {
		url = fmt.Sprintf("%s.%s.svc", url, s.ControlPlaneNamespace)
	}

	return url
}

// ComputeOutOfClusterAPIServerAddress returns the external address for the shoot API server depending on whether
// the caller wants to use the internal cluster domain and whether DNS is disabled on this seed.
func (s *Shoot) ComputeOutOfClusterAPIServerAddress(useInternalClusterDomain bool) string {
	if v1beta1helper.ShootUsesUnmanagedDNS(s.GetInfo()) {
		return gardenerutils.GetAPIServerDomain(s.InternalClusterDomain)
	}

	if useInternalClusterDomain || s.ExternalClusterDomain == nil {
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
func (s *Shoot) IsShootControlPlaneLoggingEnabled(c *gardenletconfigv1alpha1.GardenletConfiguration) bool {
	return s.Purpose != gardencorev1beta1.ShootPurposeTesting && gardenlethelper.IsLoggingEnabled(c)
}

func sortByIPFamilies(ipfamilies []gardencorev1beta1.IPFamily, cidrs []net.IPNet) []net.IPNet {
	var result []net.IPNet
	for _, ipfamily := range ipfamilies {
		switch ipfamily {
		case gardencorev1beta1.IPFamilyIPv4:
			for _, cidr := range cidrs {
				if cidr.IP.To4() != nil {
					result = append(result, cidr)
				}
			}
		case gardencorev1beta1.IPFamilyIPv6:
			for _, cidr := range cidrs {
				if cidr.IP.To4() == nil {
					result = append(result, cidr)
				}
			}
		}
	}
	return result
}

func getPrimaryCIDRs(cidrs []net.IPNet, ipFamilies []gardencorev1beta1.IPFamily) []net.IPNet {
	var result []net.IPNet
	isIPv4 := ipFamilies[0] == gardencorev1beta1.IPFamilyIPv4
	for _, cidr := range cidrs {
		if (isIPv4 && cidr.IP.To4() != nil) || (!isIPv4 && cidr.IP.To4() == nil) {
			result = append(result, cidr)
		}
	}
	return result
}

// ToNetworks return a network with computed cidrs and ClusterIPs
// for a Shoot
func ToNetworks(shoot *gardencorev1beta1.Shoot, workerless bool) (*Networks, error) {
	var (
		services, pods, nodes, egressCIDRs []net.IPNet
		apiServerIPs, dnsIPs               []net.IP
	)

	if shoot.Spec.Networking.Pods != nil {
		_, p, err := net.ParseCIDR(*shoot.Spec.Networking.Pods)
		if err != nil {
			return nil, fmt.Errorf("cannot parse shoot's pod cidr %w", err)
		}
		pods = append(pods, *p)
	} else if !workerless && !gardencorev1beta1.IsIPv6SingleStack(shoot.Spec.Networking.IPFamilies) {
		return nil, fmt.Errorf("shoot's pods cidr is empty")
	}

	if shoot.Spec.Networking.Services != nil {
		_, s, err := net.ParseCIDR(*shoot.Spec.Networking.Services)
		if err != nil {
			return nil, fmt.Errorf("cannot parse shoot's network cidr %w", err)
		}
		services = append(services, *s)
	} else if !gardencorev1beta1.IsIPv6SingleStack(shoot.Spec.Networking.IPFamilies) {
		return nil, fmt.Errorf("shoot's service cidr is empty")
	}

	if shoot.Spec.Networking.Nodes != nil {
		_, n, err := net.ParseCIDR(*shoot.Spec.Networking.Nodes)
		if err != nil {
			return nil, fmt.Errorf("cannot parse shoot's node cidr %w", err)
		}
		nodes = append(nodes, *n)
	} else if !workerless && !gardencorev1beta1.IsIPv6SingleStack(shoot.Spec.Networking.IPFamilies) {
		return nil, fmt.Errorf("shoot's node cidr is empty")
	}

	if shoot.Status.Networking != nil {
		if result, err := copyUniqueCIDRs(shoot.Status.Networking.Pods, pods, "pod"); err != nil {
			return nil, err
		} else {
			pods = sortByIPFamilies(shoot.Spec.Networking.IPFamilies, result)
		}
		if result, err := copyUniqueCIDRs(shoot.Status.Networking.Services, services, "service"); err != nil {
			return nil, err
		} else {
			services = sortByIPFamilies(shoot.Spec.Networking.IPFamilies, result)
		}
		if result, err := copyUniqueCIDRs(shoot.Status.Networking.Nodes, nodes, "node"); err != nil {
			return nil, err
		} else {
			nodes = sortByIPFamilies(shoot.Spec.Networking.IPFamilies, result)
		}
		if result, err := copyUniqueCIDRs(shoot.Status.Networking.EgressCIDRs, egressCIDRs, "egressCIDRs"); err != nil {
			return nil, err
		} else {
			egressCIDRs = sortByIPFamilies(shoot.Spec.Networking.IPFamilies, result)
		}
	}

	// During dual-stack migration, until nodes are migrated to  dual-stack, we only use the primary addresses.
	condition := v1beta1helper.GetCondition(shoot.Status.Constraints, gardencorev1beta1.ShootDualStackNodesMigrationReady)
	if condition != nil && condition.Status != gardencorev1beta1.ConditionTrue {
		nodes = getPrimaryCIDRs(nodes, shoot.Spec.Networking.IPFamilies)
		services = getPrimaryCIDRs(services, shoot.Spec.Networking.IPFamilies)
		pods = getPrimaryCIDRs(pods, shoot.Spec.Networking.IPFamilies)
	}

	for _, cidr := range services {
		apiserver, err := utils.ComputeOffsetIP(&cidr, 1)
		if err != nil {
			return nil, fmt.Errorf("cannot calculate default/kubernetes ClusterIP for service network '%s': %w", cidr.String(), err)
		}
		apiServerIPs = append(apiServerIPs, apiserver)

		coreDNS, err := utils.ComputeOffsetIP(&cidr, 10)
		if err != nil {
			return nil, fmt.Errorf("cannot calculate CoreDNS ClusterIP for service network '%s': %w", cidr.String(), err)
		}
		dnsIPs = append(dnsIPs, coreDNS)
	}

	return &Networks{
		CoreDNS:     dnsIPs,
		Pods:        pods,
		Services:    services,
		Nodes:       nodes,
		EgressCIDRs: egressCIDRs,
		APIServer:   apiServerIPs,
	}, nil
}

func copyUniqueCIDRs(src []string, dst []net.IPNet, networkType string) ([]net.IPNet, error) {
	existing := sets.New[string]()
	for _, cidr := range dst {
		existing.Insert(cidr.String())
	}
	for _, s := range src {
		if !existing.Has(s) {
			_, cidr, err := net.ParseCIDR(s)
			if err != nil {
				return nil, fmt.Errorf("cannot parse shoot's %s cidr '%s': %w", networkType, s, err)
			}
			dst = append(dst, *cidr)
		}
	}
	return dst, nil
}

// IsAutonomous returns true in case of an autonomous shoot cluster.
func (s *Shoot) IsAutonomous() bool {
	return v1beta1helper.IsShootAutonomous(s.GetInfo())
}

// RunsControlPlane returns true in case the Kubernetes control plane runs inside the cluster.
// In contrast to IsAutonomous, this function returns false when bootstrapping autonomous shoot clusters using
// `gardenadm bootstrap` (medium-touch scenario).
func (s *Shoot) RunsControlPlane() bool {
	return s.ControlPlaneNamespace == metav1.NamespaceSystem
}
