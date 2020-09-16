// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package core

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

const (
	// GardenerSeedLeaseNamespace is the namespace in which Gardenlet will report Seeds'
	// status using Lease resources for each Seed
	GardenerSeedLeaseNamespace = "gardener-system-seed-lease"
)

// Object is an core object resource.
type Object interface {
	metav1.Object
	// GetProviderType gets the type of the provider.
	GetProviderType() string
}
