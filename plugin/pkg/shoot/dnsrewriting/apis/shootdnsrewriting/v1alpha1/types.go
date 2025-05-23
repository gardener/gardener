// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Configuration provides configuration for the ShootDNSRewriting admission controller.
type Configuration struct {
	metav1.TypeMeta
	// CommonSuffixes are expected to be the suffix of a fully qualified domain name.
	// Each suffix should contain at least one or two dots ('.') to prevent accidental clashes.
	CommonSuffixes []string `json:"commonSuffixes,omitempty"`
}
