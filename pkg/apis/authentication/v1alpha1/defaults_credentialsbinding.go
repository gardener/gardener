// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

// SetDefaults_CredentialsBinding sets default values for CredentialsBinding objects.
func SetDefaults_CredentialsBinding(obj *CredentialsBinding) {
	if obj.Credentials.SecretRef != nil && len(obj.Credentials.SecretRef.Namespace) == 0 {
		obj.Credentials.SecretRef.Namespace = obj.Namespace
	}

	if obj.Credentials.WorkloadIdentityRef != nil && len(obj.Credentials.WorkloadIdentityRef.Namespace) == 0 {
		obj.Credentials.WorkloadIdentityRef.Namespace = obj.Namespace
	}

	for i, quota := range obj.Quotas {
		if len(quota.Namespace) == 0 {
			obj.Quotas[i].Namespace = obj.Namespace
		}
	}
}
