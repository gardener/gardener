// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

// SetDefaults_CredentialsBinding sets default values for CredentialsBinding objects.
func SetDefaults_CredentialsBinding(obj *CredentialsBinding) {
	if len(obj.CredentialsRef.Namespace) == 0 {
		obj.CredentialsRef.Namespace = obj.Namespace
	}

	for i, quota := range obj.Quotas {
		if len(quota.Namespace) == 0 {
			obj.Quotas[i].Namespace = obj.Namespace
		}
	}
}
