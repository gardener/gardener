// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package features

import (
	"github.com/gardener/gardener/pkg/features"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/component-base/featuregate"
)

var (
	// FeatureGate is a shared global FeatureGate for Gardener Scheduler flags.
	FeatureGate  = featuregate.NewFeatureGate()
	featureGates = map[featuregate.Feature]featuregate.FeatureSpec{
		features.CachedRuntimeClients: {Default: false, PreRelease: featuregate.Alpha},
	}
)

// RegisterFeatureGates registers the feature gates of the Gardener Scheduler.
func RegisterFeatureGates() {
	utilruntime.Must(FeatureGate.Add(featureGates))
}
