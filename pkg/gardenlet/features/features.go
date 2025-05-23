// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package features

import (
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/component-base/featuregate"

	"github.com/gardener/gardener/pkg/features"
)

// RegisterFeatureGates registers the feature gates of gardenlet.
func RegisterFeatureGates() {
	utilruntime.Must(features.DefaultFeatureGate.Add(features.GetFeatures(GetFeatures()...)))
}

// GetFeatures returns all gardenlet features.
func GetFeatures() []featuregate.Feature {
	return []featuregate.Feature{
		features.DefaultSeccompProfile,
		features.NewWorkerPoolHash,
		features.NewVPN,
		features.NodeAgentAuthorizer,
		features.RemoveAPIServerProxyLegacyPort,
		features.IstioTLSTermination,
	}
}
