// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper

import (
	"fmt"
	"slices"
	"strconv"

	"github.com/Masterminds/semver/v3"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
)

// HibernationIsEnabled checks if the given shoot's desired state is hibernated.
func HibernationIsEnabled(shoot *gardencorev1beta1.Shoot) bool {
	return shoot.Spec.Hibernation != nil && shoot.Spec.Hibernation.Enabled != nil && *shoot.Spec.Hibernation.Enabled
}

// ShootWantsClusterAutoscaler checks if the given Shoot needs a cluster autoscaler.
// This is determined by checking whether one of the Shoot workers has a different
// Maximum than Minimum.
func ShootWantsClusterAutoscaler(shoot *gardencorev1beta1.Shoot) (bool, error) {
	for _, worker := range shoot.Spec.Provider.Workers {
		if worker.Maximum > worker.Minimum {
			return true, nil
		}
	}
	return false, nil
}

// ShootWantsVerticalPodAutoscaler checks if the given Shoot needs a VPA.
func ShootWantsVerticalPodAutoscaler(shoot *gardencorev1beta1.Shoot) bool {
	return shoot.Spec.Kubernetes.VerticalPodAutoscaler != nil && shoot.Spec.Kubernetes.VerticalPodAutoscaler.Enabled
}

// ShootIgnoresAlerts checks if the alerts for the annotated shoot cluster should be ignored.
func ShootIgnoresAlerts(shoot *gardencorev1beta1.Shoot) bool {
	ignore := false
	if value, ok := shoot.Annotations[v1beta1constants.AnnotationShootIgnoreAlerts]; ok {
		ignore, _ = strconv.ParseBool(value)
	}
	return ignore
}

// ShootWantsAlertManager checks if the given shoot specification requires an alert manager.
func ShootWantsAlertManager(shoot *gardencorev1beta1.Shoot) bool {
	return !ShootIgnoresAlerts(shoot) && shoot.Spec.Monitoring != nil && shoot.Spec.Monitoring.Alerting != nil && len(shoot.Spec.Monitoring.Alerting.EmailReceivers) > 0
}

// ShootUsesUnmanagedDNS returns true if the shoot's DNS section is marked as 'unmanaged'.
func ShootUsesUnmanagedDNS(shoot *gardencorev1beta1.Shoot) bool {
	return shoot.Spec.DNS != nil && len(shoot.Spec.DNS.Providers) > 0 && shoot.Spec.DNS.Providers[0].Type != nil && *shoot.Spec.DNS.Providers[0].Type == "unmanaged"
}

// ShootNeedsForceDeletion determines whether a Shoot should be force deleted or not.
func ShootNeedsForceDeletion(shoot *gardencorev1beta1.Shoot) bool {
	if shoot == nil {
		return false
	}

	value, ok := shoot.Annotations[v1beta1constants.AnnotationConfirmationForceDeletion]
	if !ok {
		return false
	}

	forceDelete, _ := strconv.ParseBool(value)
	return forceDelete
}

// ShootSchedulingProfile returns the scheduling profile of the given Shoot.
func ShootSchedulingProfile(shoot *gardencorev1beta1.Shoot) *gardencorev1beta1.SchedulingProfile {
	if shoot.Spec.Kubernetes.KubeScheduler != nil {
		return shoot.Spec.Kubernetes.KubeScheduler.Profile
	}
	return nil
}

// IsHAControlPlaneConfigured returns true if HA configuration for the shoot control plane has been set.
func IsHAControlPlaneConfigured(shoot *gardencorev1beta1.Shoot) bool {
	return shoot.Spec.ControlPlane != nil && shoot.Spec.ControlPlane.HighAvailability != nil
}

// IsHAVPNEnabled checks if the shoot has HA VPN enabled.
func IsHAVPNEnabled(shoot *gardencorev1beta1.Shoot) bool {
	if shoot == nil {
		return false
	}

	haVPN := IsHAControlPlaneConfigured(shoot)
	if haVPNEnabled, err := strconv.ParseBool(shoot.GetAnnotations()[v1beta1constants.ShootAlphaControlPlaneHAVPN]); err == nil {
		haVPN = haVPNEnabled
	}

	return haVPN
}

// IsMultiZonalShootControlPlane checks if the shoot should have a multi-zonal control plane.
func IsMultiZonalShootControlPlane(shoot *gardencorev1beta1.Shoot) bool {
	return shoot.Spec.ControlPlane != nil && shoot.Spec.ControlPlane.HighAvailability != nil && shoot.Spec.ControlPlane.HighAvailability.FailureTolerance.Type == gardencorev1beta1.FailureToleranceTypeZone
}

// IsWorkerless checks if the shoot has zero workers.
func IsWorkerless(shoot *gardencorev1beta1.Shoot) bool {
	return len(shoot.Spec.Provider.Workers) == 0
}

// ShootEnablesSSHAccess returns true if ssh access to worker nodes should be allowed for the given shoot.
func ShootEnablesSSHAccess(shoot *gardencorev1beta1.Shoot) bool {
	return !IsWorkerless(shoot) &&
		(shoot.Spec.Provider.WorkersSettings == nil || shoot.Spec.Provider.WorkersSettings.SSHAccess == nil || shoot.Spec.Provider.WorkersSettings.SSHAccess.Enabled)
}

// GetFailureToleranceType determines the failure tolerance type of the given shoot.
func GetFailureToleranceType(shoot *gardencorev1beta1.Shoot) *gardencorev1beta1.FailureToleranceType {
	if shoot.Spec.ControlPlane != nil && shoot.Spec.ControlPlane.HighAvailability != nil {
		return &shoot.Spec.ControlPlane.HighAvailability.FailureTolerance.Type
	}
	return nil
}

// GetShootCARotationPhase returns the specified shoot CA rotation phase or an empty string
func GetShootCARotationPhase(credentials *gardencorev1beta1.ShootCredentials) gardencorev1beta1.CredentialsRotationPhase {
	if credentials != nil && credentials.Rotation != nil && credentials.Rotation.CertificateAuthorities != nil {
		return credentials.Rotation.CertificateAuthorities.Phase
	}
	return ""
}

// MutateShootCARotation mutates the .status.credentials.rotation.certificateAuthorities field based on the provided
// mutation function. If the field is nil then it is initialized.
func MutateShootCARotation(shoot *gardencorev1beta1.Shoot, f func(*gardencorev1beta1.CARotation)) {
	if f == nil {
		return
	}

	if shoot.Status.Credentials == nil {
		shoot.Status.Credentials = &gardencorev1beta1.ShootCredentials{}
	}
	if shoot.Status.Credentials.Rotation == nil {
		shoot.Status.Credentials.Rotation = &gardencorev1beta1.ShootCredentialsRotation{}
	}
	if shoot.Status.Credentials.Rotation.CertificateAuthorities == nil {
		shoot.Status.Credentials.Rotation.CertificateAuthorities = &gardencorev1beta1.CARotation{}
	}

	f(shoot.Status.Credentials.Rotation.CertificateAuthorities)
}

// MutateShootSSHKeypairRotation mutates the .status.credentials.rotation.sshKeypair field based on the provided
// mutation function. If the field is nil then it is initialized.
func MutateShootSSHKeypairRotation(shoot *gardencorev1beta1.Shoot, f func(*gardencorev1beta1.ShootSSHKeypairRotation)) {
	if f == nil {
		return
	}

	if shoot.Status.Credentials == nil {
		shoot.Status.Credentials = &gardencorev1beta1.ShootCredentials{}
	}
	if shoot.Status.Credentials.Rotation == nil {
		shoot.Status.Credentials.Rotation = &gardencorev1beta1.ShootCredentialsRotation{}
	}
	if shoot.Status.Credentials.Rotation.SSHKeypair == nil {
		shoot.Status.Credentials.Rotation.SSHKeypair = &gardencorev1beta1.ShootSSHKeypairRotation{}
	}

	f(shoot.Status.Credentials.Rotation.SSHKeypair)
}

// IsShootSSHKeypairRotationInitiationTimeAfterLastCompletionTime returns true when the lastInitiationTime in the
// .status.credentials.rotation.sshKeypair field is newer than the lastCompletionTime. This is also true if the
// lastCompletionTime is unset.
func IsShootSSHKeypairRotationInitiationTimeAfterLastCompletionTime(credentials *gardencorev1beta1.ShootCredentials) bool {
	if credentials == nil ||
		credentials.Rotation == nil ||
		credentials.Rotation.SSHKeypair == nil ||
		credentials.Rotation.SSHKeypair.LastInitiationTime == nil {
		return false
	}

	return credentials.Rotation.SSHKeypair.LastCompletionTime == nil ||
		credentials.Rotation.SSHKeypair.LastCompletionTime.Before(credentials.Rotation.SSHKeypair.LastInitiationTime)
}

// MutateObservabilityRotation mutates the .status.credentials.rotation.observability field based on the provided
// mutation function. If the field is nil then it is initialized.
func MutateObservabilityRotation(shoot *gardencorev1beta1.Shoot, f func(*gardencorev1beta1.ObservabilityRotation)) {
	if f == nil {
		return
	}

	if shoot.Status.Credentials == nil {
		shoot.Status.Credentials = &gardencorev1beta1.ShootCredentials{}
	}
	if shoot.Status.Credentials.Rotation == nil {
		shoot.Status.Credentials.Rotation = &gardencorev1beta1.ShootCredentialsRotation{}
	}
	if shoot.Status.Credentials.Rotation.Observability == nil {
		shoot.Status.Credentials.Rotation.Observability = &gardencorev1beta1.ObservabilityRotation{}
	}

	f(shoot.Status.Credentials.Rotation.Observability)
}

// IsShootObservabilityRotationInitiationTimeAfterLastCompletionTime returns true when the lastInitiationTime in the
// .status.credentials.rotation.observability field is newer than the lastCompletionTime. This is also true if the
// lastCompletionTime is unset.
func IsShootObservabilityRotationInitiationTimeAfterLastCompletionTime(credentials *gardencorev1beta1.ShootCredentials) bool {
	if credentials == nil ||
		credentials.Rotation == nil ||
		credentials.Rotation.Observability == nil ||
		credentials.Rotation.Observability.LastInitiationTime == nil {
		return false
	}

	return credentials.Rotation.Observability.LastCompletionTime == nil ||
		credentials.Rotation.Observability.LastCompletionTime.Before(credentials.Rotation.Observability.LastInitiationTime)
}

// GetShootServiceAccountKeyRotationPhase returns the specified shoot service account key rotation phase or an empty
// string.
func GetShootServiceAccountKeyRotationPhase(credentials *gardencorev1beta1.ShootCredentials) gardencorev1beta1.CredentialsRotationPhase {
	if credentials != nil && credentials.Rotation != nil && credentials.Rotation.ServiceAccountKey != nil {
		return credentials.Rotation.ServiceAccountKey.Phase
	}
	return ""
}

// MutateShootServiceAccountKeyRotation mutates the .status.credentials.rotation.serviceAccountKey field based on the
// provided mutation function. If the field is nil then it is initialized.
func MutateShootServiceAccountKeyRotation(shoot *gardencorev1beta1.Shoot, f func(*gardencorev1beta1.ServiceAccountKeyRotation)) {
	if f == nil {
		return
	}

	if shoot.Status.Credentials == nil {
		shoot.Status.Credentials = &gardencorev1beta1.ShootCredentials{}
	}
	if shoot.Status.Credentials.Rotation == nil {
		shoot.Status.Credentials.Rotation = &gardencorev1beta1.ShootCredentialsRotation{}
	}
	if shoot.Status.Credentials.Rotation.ServiceAccountKey == nil {
		shoot.Status.Credentials.Rotation.ServiceAccountKey = &gardencorev1beta1.ServiceAccountKeyRotation{}
	}

	f(shoot.Status.Credentials.Rotation.ServiceAccountKey)
}

// GetShootETCDEncryptionKeyRotationPhase returns the specified shoot ETCD encryption key rotation phase or an empty
// string.
func GetShootETCDEncryptionKeyRotationPhase(credentials *gardencorev1beta1.ShootCredentials) gardencorev1beta1.CredentialsRotationPhase {
	if credentials != nil && credentials.Rotation != nil && credentials.Rotation.ETCDEncryptionKey != nil {
		return credentials.Rotation.ETCDEncryptionKey.Phase
	}
	return ""
}

// MutateShootETCDEncryptionKeyRotation mutates the .status.credentials.rotation.etcdEncryptionKey field based on the
// provided mutation function. If the field is nil then it is initialized.
func MutateShootETCDEncryptionKeyRotation(shoot *gardencorev1beta1.Shoot, f func(*gardencorev1beta1.ETCDEncryptionKeyRotation)) {
	if f == nil {
		return
	}

	if shoot.Status.Credentials == nil {
		shoot.Status.Credentials = &gardencorev1beta1.ShootCredentials{}
	}
	if shoot.Status.Credentials.Rotation == nil {
		shoot.Status.Credentials.Rotation = &gardencorev1beta1.ShootCredentialsRotation{}
	}
	if shoot.Status.Credentials.Rotation.ETCDEncryptionKey == nil {
		shoot.Status.Credentials.Rotation.ETCDEncryptionKey = &gardencorev1beta1.ETCDEncryptionKeyRotation{}
	}

	f(shoot.Status.Credentials.Rotation.ETCDEncryptionKey)
}

// GetAllZonesFromShoot returns the set of all availability zones defined in the worker pools of the Shoot specification.
func GetAllZonesFromShoot(shoot *gardencorev1beta1.Shoot) sets.Set[string] {
	out := sets.New[string]()
	for _, worker := range shoot.Spec.Provider.Workers {
		out.Insert(worker.Zones...)
	}
	return out
}

// ShootItems provides helper functions with ShootLists
type ShootItems gardencorev1beta1.ShootList

// Union returns a set of Shoots that presents either in s or shootList
func (s *ShootItems) Union(shootItems *ShootItems) []gardencorev1beta1.Shoot {
	unionedShoots := make(map[string]gardencorev1beta1.Shoot)
	for _, s := range s.Items {
		unionedShoots[objectKey(s.Namespace, s.Name)] = s
	}

	for _, s := range shootItems.Items {
		unionedShoots[objectKey(s.Namespace, s.Name)] = s
	}

	shoots := make([]gardencorev1beta1.Shoot, 0, len(unionedShoots))
	for _, v := range unionedShoots {
		shoots = append(shoots, v)
	}

	return shoots
}

func objectKey(namespace, name string) string {
	return fmt.Sprintf("%s/%s", namespace, name)
}

// GetShootAuditPolicyConfigMapName returns the Shoot's ConfigMap reference name for the audit policy.
func GetShootAuditPolicyConfigMapName(apiServerConfig *gardencorev1beta1.KubeAPIServerConfig) string {
	if ref := GetShootAuditPolicyConfigMapRef(apiServerConfig); ref != nil {
		return ref.Name
	}
	return ""
}

// GetShootAuditPolicyConfigMapRef returns the Shoot's ConfigMap reference for the audit policy.
func GetShootAuditPolicyConfigMapRef(apiServerConfig *gardencorev1beta1.KubeAPIServerConfig) *corev1.ObjectReference {
	if apiServerConfig != nil && apiServerConfig.AuditConfig != nil && apiServerConfig.AuditConfig.AuditPolicy != nil {
		return apiServerConfig.AuditConfig.AuditPolicy.ConfigMapRef
	}
	return nil
}

// GetShootAuthenticationConfigurationConfigMapName returns the Shoot's ConfigMap reference name for the authentication
// configuration.
func GetShootAuthenticationConfigurationConfigMapName(apiServerConfig *gardencorev1beta1.KubeAPIServerConfig) string {
	if apiServerConfig != nil && apiServerConfig.StructuredAuthentication != nil {
		return apiServerConfig.StructuredAuthentication.ConfigMapName
	}
	return ""
}

// GetShootAuthorizationConfigurationConfigMapName returns the Shoot's ConfigMap reference name for the authorization
// configuration.
func GetShootAuthorizationConfigurationConfigMapName(apiServerConfig *gardencorev1beta1.KubeAPIServerConfig) string {
	if apiServerConfig != nil && apiServerConfig.StructuredAuthorization != nil {
		return apiServerConfig.StructuredAuthorization.ConfigMapName
	}
	return ""
}

// GetShootAuthorizationConfiguration returns the Shoot's authorization configuration.
func GetShootAuthorizationConfiguration(apiServerConfig *gardencorev1beta1.KubeAPIServerConfig) *gardencorev1beta1.StructuredAuthorization {
	if apiServerConfig != nil {
		return apiServerConfig.StructuredAuthorization
	}
	return nil
}

// AnonymousAuthenticationEnabled returns true if anonymous authentication is set explicitly to 'true' and false otherwise.
func AnonymousAuthenticationEnabled(kubeAPIServerConfig *gardencorev1beta1.KubeAPIServerConfig) bool {
	if kubeAPIServerConfig == nil {
		return false
	}
	if kubeAPIServerConfig.EnableAnonymousAuthentication == nil {
		return false
	}
	return *kubeAPIServerConfig.EnableAnonymousAuthentication
}

// KubeAPIServerFeatureGateDisabled returns whether the given feature gate is explicitly disabled for the kube-apiserver for the given Shoot spec.
func KubeAPIServerFeatureGateDisabled(shoot *gardencorev1beta1.Shoot, featureGate string) bool {
	kubeAPIServer := shoot.Spec.Kubernetes.KubeAPIServer
	if kubeAPIServer == nil || kubeAPIServer.FeatureGates == nil {
		return false
	}

	value, ok := kubeAPIServer.FeatureGates[featureGate]
	if !ok {
		return false
	}
	return !value
}

// KubeControllerManagerFeatureGateDisabled returns whether the given feature gate is explicitly disabled for the kube-controller-manager for the given Shoot spec.
func KubeControllerManagerFeatureGateDisabled(shoot *gardencorev1beta1.Shoot, featureGate string) bool {
	kubeControllerManager := shoot.Spec.Kubernetes.KubeControllerManager
	if kubeControllerManager == nil || kubeControllerManager.FeatureGates == nil {
		return false
	}

	value, ok := kubeControllerManager.FeatureGates[featureGate]
	if !ok {
		return false
	}
	return !value
}

// KubeProxyFeatureGateDisabled returns whether the given feature gate is disabled for the kube-proxy for the given Shoot spec.
func KubeProxyFeatureGateDisabled(shoot *gardencorev1beta1.Shoot, featureGate string) bool {
	kubeProxy := shoot.Spec.Kubernetes.KubeProxy
	if kubeProxy == nil || kubeProxy.FeatureGates == nil {
		return false
	}

	value, ok := kubeProxy.FeatureGates[featureGate]
	if !ok {
		return false
	}
	return !value
}

// ConvertShootList converts a list of Shoots to a list of pointers to Shoots.
func ConvertShootList(list []gardencorev1beta1.Shoot) []*gardencorev1beta1.Shoot {
	var result []*gardencorev1beta1.Shoot
	for i := range list {
		result = append(result, &list[i])
	}
	return result
}

// HasManagedIssuer checks if the shoot has managed issuer enabled.
func HasManagedIssuer(shoot *gardencorev1beta1.Shoot) bool {
	return shoot.GetAnnotations()[v1beta1constants.AnnotationAuthenticationIssuer] == v1beta1constants.AnnotationAuthenticationIssuerManaged
}

// GetPurpose returns the purpose of the shoot or 'evaluation' if it's nil.
func GetPurpose(s *gardencorev1beta1.Shoot) gardencorev1beta1.ShootPurpose {
	if v := s.Spec.Purpose; v != nil {
		return *v
	}
	return gardencorev1beta1.ShootPurposeEvaluation
}

// IsTopologyAwareRoutingForShootControlPlaneEnabled returns whether the topology aware routing is enabled for the given Shoot control plane.
// Topology-aware routing is enabled when the corresponding Seed setting is enabled and the Shoot has a multi-zonal control plane.
func IsTopologyAwareRoutingForShootControlPlaneEnabled(seed *gardencorev1beta1.Seed, shoot *gardencorev1beta1.Shoot) bool {
	return SeedSettingTopologyAwareRoutingEnabled(seed.Spec.Settings) && IsMultiZonalShootControlPlane(shoot)
}

// ShootConfinesSpecUpdateRollout returns a bool.
func ShootConfinesSpecUpdateRollout(maintenance *gardencorev1beta1.Maintenance) bool {
	return maintenance != nil && maintenance.ConfineSpecUpdateRollout != nil && *maintenance.ConfineSpecUpdateRollout
}

// KubernetesDashboardEnabled returns true if the kubernetes-dashboard addon is enabled in the Shoot manifest.
func KubernetesDashboardEnabled(addons *gardencorev1beta1.Addons) bool {
	return addons != nil && addons.KubernetesDashboard != nil && addons.KubernetesDashboard.Enabled
}

// NginxIngressEnabled returns true if the nginx-ingress addon is enabled in the Shoot manifest.
func NginxIngressEnabled(addons *gardencorev1beta1.Addons) bool {
	return addons != nil && addons.NginxIngress != nil && addons.NginxIngress.Enabled
}

// KubeProxyEnabled returns true if the kube-proxy is enabled in the Shoot manifest.
func KubeProxyEnabled(config *gardencorev1beta1.KubeProxyConfig) bool {
	return config != nil && config.Enabled != nil && *config.Enabled
}

// FindPrimaryDNSProvider finds the primary provider among the given `providers`.
// It returns the first provider if multiple candidates are found.
func FindPrimaryDNSProvider(providers []gardencorev1beta1.DNSProvider) *gardencorev1beta1.DNSProvider {
	for _, provider := range providers {
		if provider.Primary != nil && *provider.Primary {
			primaryProvider := provider
			return &primaryProvider
		}
	}
	return nil
}

// ShootDNSProviderSecretNamesEqual returns true when all the secretNames in the `.spec.dns.providers[]` list are the
// same.
func ShootDNSProviderSecretNamesEqual(oldDNS, newDNS *gardencorev1beta1.DNS) bool {
	var (
		oldNames = sets.New[string]()
		newNames = sets.New[string]()
	)

	if oldDNS != nil {
		for _, provider := range oldDNS.Providers {
			if provider.SecretName != nil {
				oldNames.Insert(*provider.SecretName)
			}
		}
	}

	if newDNS != nil {
		for _, provider := range newDNS.Providers {
			if provider.SecretName != nil {
				newNames.Insert(*provider.SecretName)
			}
		}
	}

	return oldNames.Equal(newNames)
}

// CalculateEffectiveKubernetesVersion if a shoot has kubernetes version specified by worker group, return this,
// otherwise the shoot kubernetes version
func CalculateEffectiveKubernetesVersion(controlPlaneVersion *semver.Version, workerKubernetes *gardencorev1beta1.WorkerKubernetes) (*semver.Version, error) {
	if workerKubernetes != nil && workerKubernetes.Version != nil {
		return semver.NewVersion(*workerKubernetes.Version)
	}
	return controlPlaneVersion, nil
}

// CalculateEffectiveKubeletConfiguration returns the worker group specific kubelet configuration if available.
// Otherwise the shoot kubelet configuration is returned
func CalculateEffectiveKubeletConfiguration(shootKubelet *gardencorev1beta1.KubeletConfig, workerKubernetes *gardencorev1beta1.WorkerKubernetes) *gardencorev1beta1.KubeletConfig {
	if workerKubernetes != nil && workerKubernetes.Kubelet != nil {
		return workerKubernetes.Kubelet
	}
	return shootKubelet
}

// SystemComponentsAllowed checks if the given worker allows system components to be scheduled onto it
func SystemComponentsAllowed(worker *gardencorev1beta1.Worker) bool {
	return worker.SystemComponents == nil || worker.SystemComponents.Allow
}

// SumResourceReservations adds together the given *gardencorev1beta1.KubeletConfigReserved values.
// The func is suitable to calculate the sum of kubeReserved and systemReserved.
func SumResourceReservations(left, right *gardencorev1beta1.KubeletConfigReserved) *gardencorev1beta1.KubeletConfigReserved {
	if left == nil {
		return right
	} else if right == nil {
		return left
	}

	return &gardencorev1beta1.KubeletConfigReserved{
		CPU:              sumQuantities(left.CPU, right.CPU),
		Memory:           sumQuantities(left.Memory, right.Memory),
		PID:              sumQuantities(left.PID, right.PID),
		EphemeralStorage: sumQuantities(left.EphemeralStorage, right.EphemeralStorage),
	}
}

func sumQuantities(left, right *resource.Quantity) *resource.Quantity {
	if left == nil {
		return right
	} else if right == nil {
		return left
	}

	copy := left.DeepCopy()
	copy.Add(*right)
	return &copy
}

// IsCoreDNSAutoscalingModeUsed indicates whether the specified autoscaling mode of CoreDNS is enabled or not.
func IsCoreDNSAutoscalingModeUsed(systemComponents *gardencorev1beta1.SystemComponents, autoscalingMode gardencorev1beta1.CoreDNSAutoscalingMode) bool {
	isDefaultMode := autoscalingMode == gardencorev1beta1.CoreDNSAutoscalingModeHorizontal
	if systemComponents == nil {
		return isDefaultMode
	}

	if systemComponents.CoreDNS == nil {
		return isDefaultMode
	}

	if systemComponents.CoreDNS.Autoscaling == nil {
		return isDefaultMode
	}

	return systemComponents.CoreDNS.Autoscaling.Mode == autoscalingMode
}

// IsNodeLocalDNSEnabled indicates whether the node local DNS cache is enabled or not.
func IsNodeLocalDNSEnabled(systemComponents *gardencorev1beta1.SystemComponents) bool {
	return systemComponents != nil && systemComponents.NodeLocalDNS != nil && systemComponents.NodeLocalDNS.Enabled
}

// GetNodeLocalDNS returns a pointer to the NodeLocalDNS spec.
func GetNodeLocalDNS(systemComponents *gardencorev1beta1.SystemComponents) *gardencorev1beta1.NodeLocalDNS {
	if systemComponents != nil {
		return systemComponents.NodeLocalDNS
	}
	return nil
}

// GetResourceByName returns the NamedResourceReference with the given name in the given slice, or nil if not found.
func GetResourceByName(resources []gardencorev1beta1.NamedResourceReference, name string) *gardencorev1beta1.NamedResourceReference {
	for _, resource := range resources {
		if resource.Name == name {
			return &resource
		}
	}
	return nil
}

// AccessRestrictionsAreSupported returns true when all the given access restrictions are supported.
func AccessRestrictionsAreSupported(seedAccessRestrictions []gardencorev1beta1.AccessRestriction, shootAccessRestrictions []gardencorev1beta1.AccessRestrictionWithOptions) bool {
	if len(shootAccessRestrictions) == 0 {
		return true
	}
	if len(shootAccessRestrictions) > len(seedAccessRestrictions) {
		return false
	}

	seedAccessRestrictionsNames := sets.New[string]()
	for _, seedAccessRestriction := range seedAccessRestrictions {
		seedAccessRestrictionsNames.Insert(seedAccessRestriction.Name)
	}

	for _, accessRestriction := range shootAccessRestrictions {
		if !seedAccessRestrictionsNames.Has(accessRestriction.Name) {
			return false
		}
	}

	return true
}

// ShouldPrepareShootForMigration determines whether the controller should prepare the shoot control plane for migration
// to another seed.
func ShouldPrepareShootForMigration(shoot *gardencorev1beta1.Shoot) bool {
	return shoot.Status.SeedName != nil && shoot.Spec.SeedName != nil && *shoot.Spec.SeedName != *shoot.Status.SeedName
}

// LastInitiationTimeForWorkerPool returns the last initiation time for the worker pool when found in the given list of
// pending workers rollouts. If the worker pool is not found in the list, the global last initiation time is returned.
func LastInitiationTimeForWorkerPool(name string, pendingWorkersRollout []gardencorev1beta1.PendingWorkersRollout, globalLastInitiationTime *metav1.Time) *metav1.Time {
	if i := slices.IndexFunc(pendingWorkersRollout, func(rollout gardencorev1beta1.PendingWorkersRollout) bool {
		return rollout.Name == name
	}); i != -1 {
		return pendingWorkersRollout[i].LastInitiationTime
	}
	return globalLastInitiationTime
}

// IsShootAutonomous returns true if the shoot has a worker pool dedicated for running the control plane components.
func IsShootAutonomous(shoot *gardencorev1beta1.Shoot) bool {
	return slices.ContainsFunc(shoot.Spec.Provider.Workers, func(worker gardencorev1beta1.Worker) bool {
		return worker.ControlPlane != nil
	})
}

// ControlPlaneWorkerPoolForShoot returns the worker pool running the control plane in case the shoot is autonomous.
func ControlPlaneWorkerPoolForShoot(shoot *gardencorev1beta1.Shoot) *gardencorev1beta1.Worker {
	if !IsShootAutonomous(shoot) {
		return nil
	}

	idx := slices.IndexFunc(shoot.Spec.Provider.Workers, func(worker gardencorev1beta1.Worker) bool {
		return worker.ControlPlane != nil
	})
	if idx == -1 {
		return nil
	}

	return &shoot.Spec.Provider.Workers[idx]
}

// ControlPlaneNamespaceForShoot returns the control plane namespace for the shoot. If it is an autonomous shoot,
// kube-system is returned. Otherwise, it is the technical ID of the shoot.
func ControlPlaneNamespaceForShoot(shoot *gardencorev1beta1.Shoot) string {
	if IsShootAutonomous(shoot) {
		return metav1.NamespaceSystem
	}
	return shoot.Status.TechnicalID
}

// IsUpdateStrategyInPlace returns true if the given machine update strategy is either AutoInPlaceUpdate or ManualInPlaceUpdate.
func IsUpdateStrategyInPlace(updateStrategy *gardencorev1beta1.MachineUpdateStrategy) bool {
	if updateStrategy == nil {
		return false
	}

	return *updateStrategy == gardencorev1beta1.AutoInPlaceUpdate || *updateStrategy == gardencorev1beta1.ManualInPlaceUpdate
}

// IsShootIstioTLSTerminationEnabled returns true if the Istio TLS termination for the shoot kube-apiserver is enabled.
func IsShootIstioTLSTerminationEnabled(shoot *gardencorev1beta1.Shoot) bool {
	value, ok := shoot.Annotations[v1beta1constants.ShootDisableIstioTLSTermination]
	if !ok {
		return true
	}

	noTLSTermination, _ := strconv.ParseBool(value)
	return !noTLSTermination
}
