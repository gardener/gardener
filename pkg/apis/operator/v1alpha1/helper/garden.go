// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper

import (
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
)

// GetCARotationPhase returns the specified garden CA rotation phase or an empty string
func GetCARotationPhase(credentials *operatorv1alpha1.Credentials) gardencorev1beta1.CredentialsRotationPhase {
	if credentials != nil && credentials.Rotation != nil && credentials.Rotation.CertificateAuthorities != nil {
		return credentials.Rotation.CertificateAuthorities.Phase
	}
	return ""
}

// MutateCARotation mutates the .status.credentials.rotation.certificateAuthorities field based on the provided
// mutation function. If the field is nil then it is initialized.
func MutateCARotation(garden *operatorv1alpha1.Garden, f func(rotation *gardencorev1beta1.CARotation)) {
	if f == nil {
		return
	}

	if garden.Status.Credentials == nil {
		garden.Status.Credentials = &operatorv1alpha1.Credentials{}
	}
	if garden.Status.Credentials.Rotation == nil {
		garden.Status.Credentials.Rotation = &operatorv1alpha1.CredentialsRotation{}
	}
	if garden.Status.Credentials.Rotation.CertificateAuthorities == nil {
		garden.Status.Credentials.Rotation.CertificateAuthorities = &gardencorev1beta1.CARotation{}
	}

	f(garden.Status.Credentials.Rotation.CertificateAuthorities)
}

// GetServiceAccountKeyRotationPhase returns the specified garden service account key rotation phase or an empty
// string.
func GetServiceAccountKeyRotationPhase(credentials *operatorv1alpha1.Credentials) gardencorev1beta1.CredentialsRotationPhase {
	if credentials != nil && credentials.Rotation != nil && credentials.Rotation.ServiceAccountKey != nil {
		return credentials.Rotation.ServiceAccountKey.Phase
	}
	return ""
}

// MutateServiceAccountKeyRotation mutates the .status.credentials.rotation.serviceAccountKey field based on the
// provided mutation function. If the field is nil then it is initialized.
func MutateServiceAccountKeyRotation(garden *operatorv1alpha1.Garden, f func(*gardencorev1beta1.ServiceAccountKeyRotation)) {
	if f == nil {
		return
	}

	if garden.Status.Credentials == nil {
		garden.Status.Credentials = &operatorv1alpha1.Credentials{}
	}
	if garden.Status.Credentials.Rotation == nil {
		garden.Status.Credentials.Rotation = &operatorv1alpha1.CredentialsRotation{}
	}
	if garden.Status.Credentials.Rotation.ServiceAccountKey == nil {
		garden.Status.Credentials.Rotation.ServiceAccountKey = &gardencorev1beta1.ServiceAccountKeyRotation{}
	}

	f(garden.Status.Credentials.Rotation.ServiceAccountKey)
}

// GetETCDEncryptionKeyRotationPhase returns the specified garden ETCD encryption key rotation phase or an empty
// string.
func GetETCDEncryptionKeyRotationPhase(credentials *operatorv1alpha1.Credentials) gardencorev1beta1.CredentialsRotationPhase {
	if credentials != nil && credentials.Rotation != nil && credentials.Rotation.ETCDEncryptionKey != nil {
		return credentials.Rotation.ETCDEncryptionKey.Phase
	}
	return ""
}

// MutateETCDEncryptionKeyRotation mutates the .status.credentials.rotation.etcdEncryptionKey field based on the
// provided mutation function. If the field is nil then it is initialized.
func MutateETCDEncryptionKeyRotation(garden *operatorv1alpha1.Garden, f func(*gardencorev1beta1.ETCDEncryptionKeyRotation)) {
	if f == nil {
		return
	}

	if garden.Status.Credentials == nil {
		garden.Status.Credentials = &operatorv1alpha1.Credentials{}
	}
	if garden.Status.Credentials.Rotation == nil {
		garden.Status.Credentials.Rotation = &operatorv1alpha1.CredentialsRotation{}
	}
	if garden.Status.Credentials.Rotation.ETCDEncryptionKey == nil {
		garden.Status.Credentials.Rotation.ETCDEncryptionKey = &gardencorev1beta1.ETCDEncryptionKeyRotation{}
	}

	f(garden.Status.Credentials.Rotation.ETCDEncryptionKey)
}

// GetWorkloadIdentityKeyRotationPhase returns the specified garden workload identity key rotation phase or an empty
// string.
func GetWorkloadIdentityKeyRotationPhase(credentials *operatorv1alpha1.Credentials) gardencorev1beta1.CredentialsRotationPhase {
	if credentials != nil && credentials.Rotation != nil && credentials.Rotation.WorkloadIdentityKey != nil {
		return credentials.Rotation.WorkloadIdentityKey.Phase
	}
	return ""
}

// MutateWorkloadIdentityKeyRotation mutates the .status.credentials.rotation.workloadIdentityKey field based on the
// provided mutation function. If the field is nil then it is initialized.
func MutateWorkloadIdentityKeyRotation(garden *operatorv1alpha1.Garden, f func(*operatorv1alpha1.WorkloadIdentityKeyRotation)) {
	if f == nil {
		return
	}

	if garden.Status.Credentials == nil {
		garden.Status.Credentials = &operatorv1alpha1.Credentials{}
	}
	if garden.Status.Credentials.Rotation == nil {
		garden.Status.Credentials.Rotation = &operatorv1alpha1.CredentialsRotation{}
	}
	if garden.Status.Credentials.Rotation.WorkloadIdentityKey == nil {
		garden.Status.Credentials.Rotation.WorkloadIdentityKey = &operatorv1alpha1.WorkloadIdentityKeyRotation{}
	}

	f(garden.Status.Credentials.Rotation.WorkloadIdentityKey)
}

// IsObservabilityRotationInitiationTimeAfterLastCompletionTime returns true when the lastInitiationTime in the
// .status.credentials.rotation.observability field is newer than the lastCompletionTime. This is also true if the
// lastCompletionTime is unset.
func IsObservabilityRotationInitiationTimeAfterLastCompletionTime(credentials *operatorv1alpha1.Credentials) bool {
	if credentials == nil ||
		credentials.Rotation == nil ||
		credentials.Rotation.Observability == nil ||
		credentials.Rotation.Observability.LastInitiationTime == nil {
		return false
	}

	return credentials.Rotation.Observability.LastCompletionTime == nil ||
		credentials.Rotation.Observability.LastCompletionTime.Before(credentials.Rotation.Observability.LastInitiationTime)
}

// MutateObservabilityRotation mutates the .status.credentials.rotation.observability field based on the provided
// mutation function. If the field is nil then it is initialized.
func MutateObservabilityRotation(garden *operatorv1alpha1.Garden, f func(*gardencorev1beta1.ObservabilityRotation)) {
	if f == nil {
		return
	}

	if garden.Status.Credentials == nil {
		garden.Status.Credentials = &operatorv1alpha1.Credentials{}
	}
	if garden.Status.Credentials.Rotation == nil {
		garden.Status.Credentials.Rotation = &operatorv1alpha1.CredentialsRotation{}
	}
	if garden.Status.Credentials.Rotation.Observability == nil {
		garden.Status.Credentials.Rotation.Observability = &gardencorev1beta1.ObservabilityRotation{}
	}

	f(garden.Status.Credentials.Rotation.Observability)
}

// HighAvailabilityEnabled returns true if the high-availability is enabled.
func HighAvailabilityEnabled(garden *operatorv1alpha1.Garden) bool {
	return garden.Spec.VirtualCluster.ControlPlane != nil && garden.Spec.VirtualCluster.ControlPlane.HighAvailability != nil
}

// TopologyAwareRoutingEnabled returns true if the topology-aware routing is enabled.
func TopologyAwareRoutingEnabled(settings *operatorv1alpha1.Settings) bool {
	return settings != nil && settings.TopologyAwareRouting != nil && settings.TopologyAwareRouting.Enabled
}

// GetETCDMainBackup returns the backup configuration for etcd main of the given garden object or nil if not configured.
func GetETCDMainBackup(garden *operatorv1alpha1.Garden) *operatorv1alpha1.Backup {
	if garden != nil && garden.Spec.VirtualCluster.ETCD != nil && garden.Spec.VirtualCluster.ETCD.Main != nil {
		return garden.Spec.VirtualCluster.ETCD.Main.Backup
	}
	return nil
}

// GetDNSProviders returns the DNS providers for the given garden object or nil if non are configured.
func GetDNSProviders(garden *operatorv1alpha1.Garden) []operatorv1alpha1.DNSProvider {
	if garden != nil && garden.Spec.DNS != nil {
		return garden.Spec.DNS.Providers
	}

	return nil
}
