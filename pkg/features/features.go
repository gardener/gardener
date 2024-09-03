// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

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

	// VPAForETCD enables using plain VPA for etcd-main and etcd-events, even if HVPA is enabled for the other components.
	// owner @voelzmo
	// alpha: v1.94.0
	// beta: v1.97.0
	VPAForETCD featuregate.Feature = "VPAForETCD"

	// DefaultSeccompProfile defaults the seccomp profile for Gardener managed workload in the seed to RuntimeDefault.
	// owner: @dimityrmirchev
	// alpha: v1.54.0
	DefaultSeccompProfile featuregate.Feature = "DefaultSeccompProfile"

	// IPv6SingleStack allows creating shoot clusters with IPv6 single-stack networking (GEP-21).
	// owner: @timebertt
	// alpha: v1.63.0
	IPv6SingleStack featuregate.Feature = "IPv6SingleStack"

	// ShootForceDeletion allows force deletion of Shoots.
	// See https://github.com/gardener/gardener/blob/master/docs/usage/shoot_operations.md#shoot-force-deletion for more details.
	// owner: @acumino @ary1992 @shafeeqes
	// alpha: v1.81.0
	// beta: v1.91.0
	ShootForceDeletion featuregate.Feature = "ShootForceDeletion"

	// UseNamespacedCloudProfile enables the usage of the NamespacedCloudProfile API object
	// nodes.
	// owner: @timuthy @benedictweis @LucaBernstein
	// alpha: v1.92.0
	UseNamespacedCloudProfile featuregate.Feature = "UseNamespacedCloudProfile"

	// ShootManagedIssuer enables the shoot managed issuer functionality described in GEP 24.
	// If enabled it will force gardenlet to fail if shoot service account hostname is not configured.
	// owner: @dimityrmirchev
	// alpha: v1.93.0
	ShootManagedIssuer featuregate.Feature = "ShootManagedIssuer"

	// VPAAndHPAForAPIServer an autoscaling mechanism for kube-apiserver of shoot or virtual garden clusters, and the gardener-apiserver.
	// They are scaled simultaneously by VPA and HPA on the same metric (CPU and memory usage).
	// The pod-trashing cycle between VPA and HPA scaling on the same metric is avoided
	// by configuring the HPA to scale on average usage (not on average utilization) and
	// by picking the target average utilization values in sync with VPA's allowed maximums.
	// The feature gate takes precedence over the `HVPA` feature gate when they are both enabled.
	// owner: @ialidzhikov
	// alpha: v1.95.0
	// beta: v1.101.0
	VPAAndHPAForAPIServer featuregate.Feature = "VPAAndHPAForAPIServer"

	// ShootCredentialsBinding enables the usage of the CredentialsBindingName API in shoot spec.
	// owner: @vpnachev @dimityrmirchev
	// alpha: v1.98.0
	ShootCredentialsBinding featuregate.Feature = "ShootCredentialsBinding"

	// NewWorkerPoolHash enables a new calculation method for the worker pool hash. The new
	// calculation supports rolling worker pools if `kubeReserved`, `systemReserved`, `evicitonHard` or `cpuManagerPolicy`
	// in the `kubelet` configuration are changed. All provider extensions must be upgraded
	// to support this feature first.
	// owner: @MichaelEischer
	// alpha: v1.98.0
	NewWorkerPoolHash featuregate.Feature = "NewWorkerPoolHash"

	// NewVPN enables the new implementation of the VPN (go rewrite) using an IPv6 transfer network.
	// owner: @MartinWeindel @ScheererJ @axel7born @DockToFuture
	// alpha: v1.104.0
	NewVPN featuregate.Feature = "NewVPN"
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
	HVPA:                      {Default: false, PreRelease: featuregate.Alpha},
	HVPAForShootedSeed:        {Default: false, PreRelease: featuregate.Alpha},
	VPAForETCD:                {Default: true, PreRelease: featuregate.Beta},
	DefaultSeccompProfile:     {Default: false, PreRelease: featuregate.Alpha},
	IPv6SingleStack:           {Default: false, PreRelease: featuregate.Alpha},
	ShootManagedIssuer:        {Default: false, PreRelease: featuregate.Alpha},
	ShootForceDeletion:        {Default: true, PreRelease: featuregate.Beta},
	UseNamespacedCloudProfile: {Default: false, PreRelease: featuregate.Alpha},
	VPAAndHPAForAPIServer:     {Default: true, PreRelease: featuregate.Beta},
	ShootCredentialsBinding:   {Default: false, PreRelease: featuregate.Alpha},
	NewWorkerPoolHash:         {Default: false, PreRelease: featuregate.Alpha},
	NewVPN:                    {Default: false, PreRelease: featuregate.Alpha},
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
