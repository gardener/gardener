// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

// GetServiceAccountKeyRotationPhase returns the specified shoot service account key rotation phase or an empty
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

// GetETCDEncryptionKeyRotationPhase returns the specified shoot ETCD encryption key rotation phase or an empty
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

// HighAvailabilityEnabled returns true if the high-availability is enabled.
func HighAvailabilityEnabled(garden *operatorv1alpha1.Garden) bool {
	return garden.Spec.VirtualCluster.ControlPlane != nil && garden.Spec.VirtualCluster.ControlPlane.HighAvailability != nil
}

// TopologyAwareRoutingEnabled returns true if the topology-aware routing is enabled.
func TopologyAwareRoutingEnabled(settings *operatorv1alpha1.Settings) bool {
	return settings != nil && settings.TopologyAwareRouting != nil && settings.TopologyAwareRouting.Enabled
}
