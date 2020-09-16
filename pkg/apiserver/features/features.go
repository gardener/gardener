// SPDX-FileCopyrightText: 2018 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package features

import (
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/component-base/featuregate"
)

var (
	// FeatureGate is a shared global FeatureGate for Gardener APIServer flags.
	// right now the Generic API server uses this feature gate as default
	FeatureGate  = featuregate.NewFeatureGate()
	featureGates = map[featuregate.Feature]featuregate.FeatureSpec{}
)

// RegisterFeatureGates registers the feature gates of the Gardener API Server.
func RegisterFeatureGates() {
	utilruntime.Must(FeatureGate.Add(featureGates))
}
