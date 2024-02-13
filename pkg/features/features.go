// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package features

import (
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	"k8s.io/component-base/featuregate"
)

const (
	// Every feature gate should add method here following this template:
	//
	// // MyFeature enable Foo.
	// // owner: @username
	// // alpha: v5.X
	// MyFeature featuregate.Feature = "MyFeature"

	// HVPA enables simultaneous horizontal and vertical scaling in Seed Clusters.
	// owner @shreyas-s-rao @voelzmo
	// alpha: v0.31.0
	HVPA featuregate.Feature = "HVPA"

	// HVPAForShootedSeed enables simultaneous horizontal and vertical scaling in shooted seed Clusters.
	// owner @shreyas-s-rao @voelzmo
	// alpha: v0.32.0
	HVPAForShootedSeed featuregate.Feature = "HVPAForShootedSeed"

	// DefaultSeccompProfile defaults the seccomp profile for Gardener managed workload in the seed to RuntimeDefault.
	// owner: @dimityrmirchev
	// alpha: v1.54.0
	DefaultSeccompProfile featuregate.Feature = "DefaultSeccompProfile"

	// CoreDNSQueryRewriting enables automatic DNS query rewriting in shoot cluster's CoreDNS to shortcut name resolution of
	// fully qualified in-cluster and out-of-cluster names, which follow a user-defined pattern.
	// owner: @ScheererJ @DockToFuture
	// alpha: v1.55.0
	CoreDNSQueryRewriting featuregate.Feature = "CoreDNSQueryRewriting"

	// IPv6SingleStack allows creating shoot clusters with IPv6 single-stack networking (GEP-21).
	// owner: @timebertt
	// alpha: v1.63.0
	IPv6SingleStack featuregate.Feature = "IPv6SingleStack"

	// MutableShootSpecNetworkingNodes allows updating the field `spec.networking.nodes`.
	// owner: @axel7born @ScheererJ @DockToFuture @kon-angelo
	// alpha: v1.64.0
	MutableShootSpecNetworkingNodes featuregate.Feature = "MutableShootSpecNetworkingNodes"

	// ShootForceDeletion allows force deletion of Shoots.
	// See https://github.com/gardener/gardener/blob/master/docs/usage/shoot_operations.md#shoot-force-deletion for more details.
	// owner: @acumino @ary1992 @shafeeqes
	// alpha: v1.81.0
	ShootForceDeletion featuregate.Feature = "ShootForceDeletion"

	// MachineControllerManagerDeployment enables Gardener to take over the deployment of the
	// machine-controller-manager. If enabled, all registered provider extensions must support injecting the
	// provider-specific MCM provider sidecar container into the deployment via the `controlplane` webhook.
	// owner: @rfranzke @JensAc @mreiger
	// alpha: v1.73.0
	// beta: v1.81.0
	// GA: v1.82.0
	// TODO(scheererj): Do not remove this feature gate before v1.90 to give extensions enough time to adopt v1.83.
	// Otherwise, extensions might end up believing they need to manage MCM again when the feature gate is removed.
	MachineControllerManagerDeployment featuregate.Feature = "MachineControllerManagerDeployment"

	// APIServerFastRollout enables fast rollouts for Shoot kube-apiservers on the given Seed.
	// When enabled, maxSurge for Shoot kube-apiserver deployments is set to 100%.
	// owner: @oliver-goetz
	// beta: v1.82.0
	APIServerFastRollout featuregate.Feature = "APIServerFastRollout"

	// UseGardenerNodeAgent enables the `gardener-node-agent` instead of the `cloud-config-downloader` for shoot worker
	// nodes.
	// owner: @rfranzke @oliver-goetz
	// alpha: v1.82.0
	// beta: v1.89.0
	UseGardenerNodeAgent featuregate.Feature = "UseGardenerNodeAgent"
)

// DefaultFeatureGate is the central feature gate map used by all gardener components.
// On startup, the component needs to register all feature gates that are available for this component via `Add`, e.g.:
//
//	 utilruntime.Must(features.DefaultFeatureGate.Add(features.GetFeatures(
//			features.MyFeatureGateName,
//		)))
//
// With this, every component has its individual set of available feature gates (different to Kubernetes, where all
// components have all feature gates even if irrelevant).
// Additionally, the component needs to set the feature gates' states based on the operator's configuration, e.g.:
//
//	features.DefaultFeatureGate.SetFromMap(o.config.FeatureGates)
//
// For checking whether a given feature gate is enabled (regardless of which component the code is executed in), use:
//
//	features.DefaultFeatureGate.Enabled(features.IPv6SingleStack)
//
// With this, code that needs to check a given feature gate's state can be shared across components, e.g. in API
// validation code for Seeds (executed in gardener-apiserver and gardenlet).
// This variable is an alias to the feature gate map in the apiserver library. The library doesn't allow using a custom
// feature gate map for gardener-apiserver. Hence, we reuse it for all our components.
var DefaultFeatureGate = utilfeature.DefaultMutableFeatureGate

// AllFeatureGates is the list of all feature gates.
var AllFeatureGates = map[featuregate.Feature]featuregate.FeatureSpec{
	HVPA:                               {Default: false, PreRelease: featuregate.Alpha},
	HVPAForShootedSeed:                 {Default: false, PreRelease: featuregate.Alpha},
	DefaultSeccompProfile:              {Default: false, PreRelease: featuregate.Alpha},
	CoreDNSQueryRewriting:              {Default: false, PreRelease: featuregate.Alpha},
	IPv6SingleStack:                    {Default: false, PreRelease: featuregate.Alpha},
	MutableShootSpecNetworkingNodes:    {Default: false, PreRelease: featuregate.Alpha},
	ShootForceDeletion:                 {Default: false, PreRelease: featuregate.Alpha},
	MachineControllerManagerDeployment: {Default: true, PreRelease: featuregate.GA, LockToDefault: true},
	APIServerFastRollout:               {Default: true, PreRelease: featuregate.Beta},
	UseGardenerNodeAgent:               {Default: true, PreRelease: featuregate.Beta},
}

// GetFeatures returns a feature gate map with the respective specifications. Non-existing feature gates are ignored.
func GetFeatures(featureGates ...featuregate.Feature) map[featuregate.Feature]featuregate.FeatureSpec {
	out := make(map[featuregate.Feature]featuregate.FeatureSpec)

	for _, fg := range featureGates {
		if spec, ok := AllFeatureGates[fg]; ok {
			out[fg] = spec
		}
	}

	return out
}
