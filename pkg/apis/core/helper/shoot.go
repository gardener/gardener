// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper

import (
	"strconv"

	"github.com/Masterminds/semver/v3"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/pkg/apis/core"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
)

// AccessRestrictionsAreSupported returns true when all the given access restrictions are supported.
func AccessRestrictionsAreSupported(seedAccessRestrictions []core.AccessRestriction, shootAccessRestrictions []core.AccessRestrictionWithOptions) bool {
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

// ShootUsesUnmanagedDNS returns true if the shoot's DNS section is marked as 'unmanaged'.
func ShootUsesUnmanagedDNS(shoot *core.Shoot) bool {
	if shoot.Spec.DNS == nil {
		return false
	}

	primary := FindPrimaryDNSProvider(shoot.Spec.DNS.Providers)
	if primary != nil {
		return *primary.Primary && primary.Type != nil && *primary.Type == core.DNSUnmanaged
	}

	return len(shoot.Spec.DNS.Providers) > 0 && shoot.Spec.DNS.Providers[0].Type != nil && *shoot.Spec.DNS.Providers[0].Type == core.DNSUnmanaged
}

// ShootNeedsForceDeletion determines whether a Shoot should be force deleted or not.
func ShootNeedsForceDeletion(shoot *core.Shoot) bool {
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

// FindPrimaryDNSProvider finds the primary provider among the given `providers`.
// It returns the first provider if multiple candidates are found.
func FindPrimaryDNSProvider(providers []core.DNSProvider) *core.DNSProvider {
	for _, provider := range providers {
		if provider.Primary != nil && *provider.Primary {
			primaryProvider := provider
			return &primaryProvider
		}
	}
	return nil
}

// FindWorkerByName tries to find the worker with the given name. If it cannot be found it returns nil.
func FindWorkerByName(workers []core.Worker, name string) *core.Worker {
	for _, w := range workers {
		if w.Name == name {
			return &w
		}
	}
	return nil
}

// GetShootAuditPolicyConfigMapName returns the Shoot's ConfigMap reference name for the audit policy.
func GetShootAuditPolicyConfigMapName(apiServerConfig *core.KubeAPIServerConfig) string {
	if ref := GetShootAuditPolicyConfigMapRef(apiServerConfig); ref != nil {
		return ref.Name
	}
	return ""
}

// GetShootAuditPolicyConfigMapRef returns the Shoot's ConfigMap reference for the audit policy.
func GetShootAuditPolicyConfigMapRef(apiServerConfig *core.KubeAPIServerConfig) *corev1.ObjectReference {
	if apiServerConfig != nil && apiServerConfig.AuditConfig != nil && apiServerConfig.AuditConfig.AuditPolicy != nil {
		return apiServerConfig.AuditConfig.AuditPolicy.ConfigMapRef
	}
	return nil
}

// GetShootAuthenticationConfigurationConfigMapName returns the Shoot's ConfigMap reference name for the authentication
// configuration.
func GetShootAuthenticationConfigurationConfigMapName(apiServerConfig *core.KubeAPIServerConfig) string {
	if apiServerConfig != nil && apiServerConfig.StructuredAuthentication != nil {
		return apiServerConfig.StructuredAuthentication.ConfigMapName
	}
	return ""
}

// GetShootAuthorizationConfigurationConfigMapName returns the Shoot's ConfigMap reference name for the authorization
// configuration.
func GetShootAuthorizationConfigurationConfigMapName(apiServerConfig *core.KubeAPIServerConfig) string {
	if apiServerConfig != nil && apiServerConfig.StructuredAuthorization != nil {
		return apiServerConfig.StructuredAuthorization.ConfigMapName
	}
	return ""
}

// GetShootServiceAccountConfigIssuer returns the Shoot's ServiceAccountConfig Issuer.
func GetShootServiceAccountConfigIssuer(apiServerConfig *core.KubeAPIServerConfig) *string {
	if apiServerConfig != nil && apiServerConfig.ServiceAccountConfig != nil {
		return apiServerConfig.ServiceAccountConfig.Issuer
	}
	return nil
}

// GetShootServiceAccountConfigAcceptedIssuers returns the Shoot's ServiceAccountConfig AcceptedIssuers.
func GetShootServiceAccountConfigAcceptedIssuers(apiServerConfig *core.KubeAPIServerConfig) []string {
	if apiServerConfig != nil && apiServerConfig.ServiceAccountConfig != nil {
		return apiServerConfig.ServiceAccountConfig.AcceptedIssuers
	}
	return nil
}

// HibernationIsEnabled checks if the given shoot's desired state is hibernated.
func HibernationIsEnabled(shoot *core.Shoot) bool {
	return shoot.Spec.Hibernation != nil && ptr.Deref(shoot.Spec.Hibernation.Enabled, false)
}

// IsShootInHibernation checks if the given shoot is in hibernation or is waking up.
func IsShootInHibernation(shoot *core.Shoot) bool {
	if shoot.Spec.Hibernation != nil && shoot.Spec.Hibernation.Enabled != nil {
		return *shoot.Spec.Hibernation.Enabled || shoot.Status.IsHibernated
	}

	return shoot.Status.IsHibernated
}

// SystemComponentsAllowed checks if the given worker allows system components to be scheduled onto it
func SystemComponentsAllowed(worker *core.Worker) bool {
	return worker.SystemComponents == nil || worker.SystemComponents.Allow
}

// GetResourceByName returns the NamedResourceReference with the given name in the given slice, or nil if not found.
func GetResourceByName(resources []core.NamedResourceReference, name string) *core.NamedResourceReference {
	for _, resource := range resources {
		if resource.Name == name {
			return &resource
		}
	}
	return nil
}

// KubernetesDashboardEnabled returns true if the kubernetes-dashboard addon is enabled in the Shoot manifest.
func KubernetesDashboardEnabled(addons *core.Addons) bool {
	return addons != nil && addons.KubernetesDashboard != nil && addons.KubernetesDashboard.Enabled
}

// NginxIngressEnabled returns true if the nginx-ingress addon is enabled in the Shoot manifest.
func NginxIngressEnabled(addons *core.Addons) bool {
	return addons != nil && addons.NginxIngress != nil && addons.NginxIngress.Enabled
}

// ShootWantsVerticalPodAutoscaler checks if the given Shoot needs a VPA.
func ShootWantsVerticalPodAutoscaler(shoot *core.Shoot) bool {
	return shoot.Spec.Kubernetes.VerticalPodAutoscaler != nil && shoot.Spec.Kubernetes.VerticalPodAutoscaler.Enabled
}

// GetShootCARotationPhase returns the specified shoot CA rotation phase or an empty string
func GetShootCARotationPhase(credentials *core.ShootCredentials) core.CredentialsRotationPhase {
	if credentials != nil && credentials.Rotation != nil && credentials.Rotation.CertificateAuthorities != nil {
		return credentials.Rotation.CertificateAuthorities.Phase
	}
	return ""
}

// GetShootServiceAccountKeyRotationPhase returns the specified shoot service account key rotation phase or an empty
// string.
func GetShootServiceAccountKeyRotationPhase(credentials *core.ShootCredentials) core.CredentialsRotationPhase {
	if credentials != nil && credentials.Rotation != nil && credentials.Rotation.ServiceAccountKey != nil {
		return credentials.Rotation.ServiceAccountKey.Phase
	}
	return ""
}

// GetShootETCDEncryptionKeyRotationPhase returns the specified shoot ETCD encryption key rotation phase or an empty
// string.
func GetShootETCDEncryptionKeyRotationPhase(credentials *core.ShootCredentials) core.CredentialsRotationPhase {
	if credentials != nil && credentials.Rotation != nil && credentials.Rotation.ETCDEncryptionKey != nil {
		return credentials.Rotation.ETCDEncryptionKey.Phase
	}
	return ""
}

// CalculateEffectiveKubernetesVersion if a shoot has kubernetes version specified by worker group, return this,
// otherwise the shoot kubernetes version
func CalculateEffectiveKubernetesVersion(controlPlaneVersion *semver.Version, workerKubernetes *core.WorkerKubernetes) (*semver.Version, error) {
	if workerKubernetes != nil && workerKubernetes.Version != nil {
		return semver.NewVersion(*workerKubernetes.Version)
	}
	return controlPlaneVersion, nil
}

// GetAllZonesFromShoot returns the set of all availability zones defined in the worker pools of the Shoot specification.
func GetAllZonesFromShoot(shoot *core.Shoot) sets.Set[string] {
	out := sets.New[string]()
	for _, worker := range shoot.Spec.Provider.Workers {
		out.Insert(worker.Zones...)
	}
	return out
}

// IsHAControlPlaneConfigured returns true if HA configuration for the shoot control plane has been set.
func IsHAControlPlaneConfigured(shoot *core.Shoot) bool {
	return shoot.Spec.ControlPlane != nil && shoot.Spec.ControlPlane.HighAvailability != nil
}

// IsMultiZonalShootControlPlane checks if the shoot should have a multi-zonal control plane.
func IsMultiZonalShootControlPlane(shoot *core.Shoot) bool {
	return shoot.Spec.ControlPlane != nil && shoot.Spec.ControlPlane.HighAvailability != nil && shoot.Spec.ControlPlane.HighAvailability.FailureTolerance.Type == core.FailureToleranceTypeZone
}

// IsWorkerless checks if the shoot has zero workers.
func IsWorkerless(shoot *core.Shoot) bool {
	return len(shoot.Spec.Provider.Workers) == 0
}

// ShootEnablesSSHAccess returns true if ssh access to worker nodes should be allowed for the given shoot.
func ShootEnablesSSHAccess(shoot *core.Shoot) bool {
	return !IsWorkerless(shoot) &&
		(shoot.Spec.Provider.WorkersSettings == nil || shoot.Spec.Provider.WorkersSettings.SSHAccess == nil || shoot.Spec.Provider.WorkersSettings.SSHAccess.Enabled)
}

// HasManagedIssuer checks if the shoot has managed issuer enabled.
func HasManagedIssuer(shoot *core.Shoot) bool {
	return shoot.GetAnnotations()[v1beta1constants.AnnotationAuthenticationIssuer] == v1beta1constants.AnnotationAuthenticationIssuerManaged
}
