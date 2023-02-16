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

	// ManagedIstio installs minimal Istio components in istio-system.
	// Disable this feature if Istio is already installed in the cluster.
	// Istio is not automatically removed if this feature is set to false.
	// See https://github.com/gardener/gardener/blob/master/docs/usage/istio.md
	// owner @ScheererJ @DockToFuture
	// alpha: v1.5.0
	// beta: v1.19.0
	// deprecated: v1.48.0
	ManagedIstio featuregate.Feature = "ManagedIstio"

	// APIServerSNI allows to use only one LoadBalancer in the Seed cluster
	// for all Shoot clusters. Requires Istio to be installed in the cluster or
	// ManagedIstio feature gate to be enabled.
	// See https://github.com/gardener/gardener/blob/master/docs/proposals/08-shoot-apiserver-via-sni.md
	// owner @ScheererJ @DockToFuture
	// alpha: v1.7.0
	// beta: v1.19.0
	// deprecated: v1.48.0
	APIServerSNI featuregate.Feature = "APIServerSNI"

	// SeedChange enables updating the `spec.seedName` field during shoot validation from a non-empty value
	// in order to trigger shoot control plane migration.
	// owner: @plkokanov
	// alpha: v1.12.0
	// beta: v1.53.0
	// GA: v1.69.0
	SeedChange featuregate.Feature = "SeedChange"

	// ReversedVPN moves the openvpn server to the seed.
	// owner: @ScheererJ @DockToFuture
	// alpha: v1.22.0
	// beta: v1.42.0
	// GA: v1.63.0
	ReversedVPN featuregate.Feature = "ReversedVPN"

	// CopyEtcdBackupsDuringControlPlaneMigration enables the copy of etcd backups from the object store of the source seed
	// to the object store of the destination seed during control plane migration.
	// owner: @plkokanov
	// alpha: v1.37.0
	// beta: v1.53.0
	// GA: v1.69.0
	CopyEtcdBackupsDuringControlPlaneMigration featuregate.Feature = "CopyEtcdBackupsDuringControlPlaneMigration"

	// HAControlPlanes allows shoot control planes to be run in high availability mode.
	// owner: @shreyas-s-rao @timuthy
	// alpha: v1.49.0
	HAControlPlanes featuregate.Feature = "HAControlPlanes"

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

	// FullNetworkPoliciesInRuntimeCluster enables gardenlet's NetworkPolicy controller to place 'deny-all' network policies in
	// all relevant namespaces in the seed cluster.
	// owner: @rfranzke
	// alpha: v1.66.0
	FullNetworkPoliciesInRuntimeCluster featuregate.Feature = "FullNetworkPoliciesInRuntimeCluster"
)

// DefaultFeatureGate TODO: Comment
var DefaultFeatureGate = utilfeature.DefaultMutableFeatureGate

var allFeatureGates = map[featuregate.Feature]featuregate.FeatureSpec{
	HVPA:               {Default: false, PreRelease: featuregate.Alpha},
	HVPAForShootedSeed: {Default: false, PreRelease: featuregate.Alpha},
	ManagedIstio:       {Default: true, PreRelease: featuregate.Deprecated, LockToDefault: true},
	APIServerSNI:       {Default: true, PreRelease: featuregate.Deprecated, LockToDefault: true},
	SeedChange:         {Default: true, PreRelease: featuregate.GA, LockToDefault: true},
	ReversedVPN:        {Default: true, PreRelease: featuregate.GA, LockToDefault: true},
	CopyEtcdBackupsDuringControlPlaneMigration: {Default: true, PreRelease: featuregate.GA, LockToDefault: true},
	HAControlPlanes:                     {Default: false, PreRelease: featuregate.Alpha},
	DefaultSeccompProfile:               {Default: false, PreRelease: featuregate.Alpha},
	CoreDNSQueryRewriting:               {Default: false, PreRelease: featuregate.Alpha},
	IPv6SingleStack:                     {Default: false, PreRelease: featuregate.Alpha},
	MutableShootSpecNetworkingNodes:     {Default: false, PreRelease: featuregate.Alpha},
	FullNetworkPoliciesInRuntimeCluster: {Default: false, PreRelease: featuregate.Alpha},
}

// GetFeatures returns a feature gate map with the respective specifications. Non-existing feature gates are ignored.
func GetFeatures(featureGates ...featuregate.Feature) map[featuregate.Feature]featuregate.FeatureSpec {
	out := make(map[featuregate.Feature]featuregate.FeatureSpec)

	for _, fg := range featureGates {
		if spec, ok := allFeatureGates[fg]; ok {
			out[fg] = spec
		}
	}

	return out
}
