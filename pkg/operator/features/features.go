// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package features

import (
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	"github.com/gardener/gardener/pkg/features"
)

// RegisterFeatureGates registers the feature gates of gardener-operator.
func RegisterFeatureGates() {
	utilruntime.Must(features.DefaultFeatureGate.Add(features.GetFeatures(
		features.DefaultSeccompProfile,
		features.IstioTLSTermination,
	)))
}
