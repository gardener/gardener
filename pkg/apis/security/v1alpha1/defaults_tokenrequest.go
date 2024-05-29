// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

// SetDefaults_TokenRequest sets default values for TokenRequest objects.
func SetDefaults_TokenRequest(obj *TokenRequest) {
	var defaultDurationSeconds int64 = 60 * 60
	if obj.Spec.DurationSeconds == nil {
		obj.Spec.DurationSeconds = &defaultDurationSeconds
	}
}
