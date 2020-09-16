// SPDX-FileCopyrightText: 2018 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package features

import (
	"github.com/gardener/gardener/pkg/features"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/component-base/featuregate"
)

var (
	// FeatureGate is a shared global FeatureGate for Gardenlet flags.
	FeatureGate  = featuregate.NewFeatureGate()
	featureGates = map[featuregate.Feature]featuregate.FeatureSpec{
		features.Logging:              {Default: false, PreRelease: featuregate.Alpha},
		features.HVPA:                 {Default: false, PreRelease: featuregate.Alpha},
		features.HVPAForShootedSeed:   {Default: false, PreRelease: featuregate.Alpha},
		features.ManagedIstio:         {Default: false, PreRelease: featuregate.Alpha},
		features.KonnectivityTunnel:   {Default: false, PreRelease: featuregate.Alpha},
		features.APIServerSNI:         {Default: false, PreRelease: featuregate.Alpha},
		features.CachedRuntimeClients: {Default: false, PreRelease: featuregate.Alpha},
		features.NodeLocalDNS:         {Default: false, PreRelease: featuregate.Alpha},
	}
)

// RegisterFeatureGates registers the feature gates of the Gardenlet.
func RegisterFeatureGates() {
	utilruntime.Must(FeatureGate.Add(featureGates))
}
