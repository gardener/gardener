// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package features

import (
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/component-base/featuregate"

	apiserverfeatures "github.com/gardener/gardener/pkg/apiserver/features"
	"github.com/gardener/gardener/pkg/features"
	gardenletfeatures "github.com/gardener/gardener/pkg/gardenlet/features"
)

// RegisterFeatureGates registers the feature gates of gardenadm.
func RegisterFeatureGates() {
	utilruntime.Must(features.DefaultFeatureGate.Add(features.GetFeatures(GetFeatures()...)))
	apiserverfeatures.RegisterFeatureGates()
	// Self-hosted shoots always use AutoInPlaceUpdate for unmanaged infrastructure workers (defaulted in
	// defaults_shoot.go), so enable InPlaceNodeUpdates unconditionally in gardenadm.
	utilruntime.Must(features.DefaultFeatureGate.SetFromMap(map[string]bool{
		string(features.InPlaceNodeUpdates): true,
	}))
}

// GetFeatures returns all gardenadm features. This includes all gardenlet features.
func GetFeatures() []featuregate.Feature {
	return gardenletfeatures.GetFeatures()
}
