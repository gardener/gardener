// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
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

	// DefaultSeccompProfile defaults the seccomp profile for Gardener managed workload in the seed to RuntimeDefault.
	// owner: @dimityrmirchev
	// alpha: v1.54.0
	DefaultSeccompProfile featuregate.Feature = "DefaultSeccompProfile"

	// InPlaceNodeUpdates enables setting the update strategy of worker pools to `AutoInPlaceUpdate` or `ManualInPlaceUpdate` in the Shoot API.
	// owner: @acumino @ary1992 @shafeeqes
	// alpha: v1.113.0
	InPlaceNodeUpdates featuregate.Feature = "InPlaceNodeUpdates"

	// IstioTLSTermination enables TLS termination for the Istio Ingress Gateway instead of TLS termination at the kube-apiserver.
	// It allows load-balancing of requests to the kube-apiserver on request level instead of connection level.
	// owner: @oliver-goetz
	// alpha: v1.114.0
	IstioTLSTermination featuregate.Feature = "IstioTLSTermination"

	// CloudProfileCapabilities enables the usage of capabilities in the CloudProfile. Capabilities are used to create a relation between
	// machineTypes and machineImages. It allows to validate worker groups of a shoot ensuring the selected image and machine combination
	// will boot up successfully. Capabilities are also used to determine valid upgrade paths during automated maintenance operations.
	// owner: @roncossek
	// alpha: v1.117.0
	CloudProfileCapabilities featuregate.Feature = "CloudProfileCapabilities"

	// VersionClassificationLifecycle enables the features introduced by GEP-0032,
	// including lifecycle-based classification for Kubernetes and machine image versions.
	// owner: @rapsnx
	// alpha: v1.137.0
	VersionClassificationLifecycle featuregate.Feature = "VersionClassificationLifecycle"

	// DoNotCopyBackupCredentials disables the copying of Shoot infrastructure credentials as backup credentials when the Shoot is used as a ManagedSeed.
	// Operators are responsible for providing the credentials for backup explicitly.
	// Credentials that were already copied will be labeled with "secret.backup.gardener.cloud/status=previously-managed" and would have to be cleaned up by operators.
	// owner: @dimityrmirchev
	// alpha: v1.121.0
	// beta: v1.123.0
	// GA: v1.134.0
	DoNotCopyBackupCredentials featuregate.Feature = "DoNotCopyBackupCredentials"

	// OpenTelemetryCollector enables the usage of an OpenTelemetry Collector instance in the Control Plane of Shoot clusters.
	// All logs will be routed through the Collector before they reach the Vali instance.
	// owner: @rrhubenov
	// alpha: v1.124.0
	// beta: v1.136.0
	OpenTelemetryCollector featuregate.Feature = "OpenTelemetryCollector"

	// VictoriaLogsBackend enables the deployment of VictoriaLogs instance in the Control Plane of Shoot clusters.
	// VictoriaLogs will replace Vali as the log aggregation system.
	// owner: @rrhubenov
	// alpha: v1.137.0
	VictoriaLogsBackend featuregate.Feature = "VictoriaLogsBackend"

	// UseUnifiedHTTPProxyPort enables the API server proxy and shoot VPN client to connect to the unified port using the new X-Gardener-Destination header.
	// owner: @hown3d
	// alpha: v1.130.0
	// beta: v1.140.0
	UseUnifiedHTTPProxyPort featuregate.Feature = "UseUnifiedHTTPProxyPort"

	// VPAInPlaceUpdates enables the usage of in-place Pod resource updates in the Vertical Pod Autoscaler resources
	// to perform in-place Pod resource updates.
	// owner: @vitanovs @ialidzhikov
	// alpha: v1.133.0
	// beta: v1.138.0
	VPAInPlaceUpdates featuregate.Feature = "VPAInPlaceUpdates"

	// CustomDNSServerInNodeLocalDNS enables custom server block support for NodeLocalDNS in the custom CoreDNS configuration of Shoot clusters.
	// owner: @docktofuture
	// beta: v1.133.0
	CustomDNSServerInNodeLocalDNS featuregate.Feature = "CustomDNSServerInNodeLocalDNS"

	// VPNBondingModeRoundRobin enables the usage of the "balance-rr" bonding mode for the HA VPN setup.
	// owner: @domdom82
	// alpha: v1.135.0
	VPNBondingModeRoundRobin featuregate.Feature = "VPNBondingModeRoundRobin"

	// PrometheusHealthChecks enables care controllers to query Prometheus for enhanced health checks of monitoring components. Detected health issues
	// are reported in the respective `Shoot`, `Seed`, or `Garden` resource.
	// owner: @vicwicker @istvanballok
	// alpha: v1.135.0
	PrometheusHealthChecks featuregate.Feature = "PrometheusHealthChecks"

	// RemoveVali enables the automatic removal of Vali log aggregation components once VictoriaLogs has been deployed
	// for a sufficient period. Requires VictoriaLogsBackend to be enabled. When both feature gates are enabled,
	// Vali will be destroyed after VictoriaLogs has been running for 2 weeks.
	// owner: @rrhubenov
	// alpha: v1.140.0
	RemoveVali featuregate.Feature = "RemoveVali"

	// DisableNginxIngressInGarden disables the deployment of the nginx ingress controller in the Garden runtime cluster
	// and removes the nginx ingress controller (if existing) from the Garden runtime cluster.
	// owner: @ScheererJ
	// alpha: v1.142.0
	DisableNginxIngressInGarden featuregate.Feature = "DisableNginxIngressInGarden"

	// DisableNginxIngressInSeed disables the deployment of the nginx ingress controller in the Seed cluster
	// and removes the nginx ingress controller (if existing) from the Seed cluster.
	// owner: @ScheererJ
	// alpha: v1.142.0
	DisableNginxIngressInSeed featuregate.Feature = "DisableNginxIngressInSeed"

	// DisableNginxIngressInShoot disables the deployment of the nginx ingress controller in the Shoot cluster
	// and removes the nginx ingress controller (if existing) from the Shoot cluster.
	// If set for the gardener-apiserver, the creation of new Shoot clusters with the addon enabled is blocked.
	// Existing Shoot clusters can only disable the addon, but not enable it anymore.
	// If set for the gardener-controller-manager, the maintenance controller will disable the addon during the next maintenance operation.
	// owner: @ScheererJ
	// alpha: v1.142.0
	DisableNginxIngressInShoot featuregate.Feature = "DisableNginxIngressInShoot"

	// LiveControlPlaneMigration enables live migration of Shoot control planes between seeds
	// without downtime, as described in GEP-0039.
	// owner: @acumino @ary1992 @shafeeqes @seshachalam-yv
	// alpha: v1.142.0
	LiveControlPlaneMigration featuregate.Feature = "LiveControlPlaneMigration"

	// BackupEntryForGarden enables deploying a BackupEntry extension object in the garden controller
	// alongside the BackupBucket when etcd backup is configured. The generic actuator then creates the
	// etcd-backup secret, aligning the garden with the same extension contract that shoot clusters use.
	// owner: @rfranzke
	// alpha: v1.142.0
	BackupEntryForGarden featuregate.Feature = "BackupEntryForGarden"
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
//	features.DefaultFeatureGate.Enabled(features.DefaultSeccompProfile)
//
// With this, code that needs to check a given feature gate's state can be shared across components, e.g. in API
// validation code for Seeds (executed in gardener-apiserver and gardenlet).
// This variable is an alias to the feature gate map in the apiserver library. The library doesn't allow using a custom
// feature gate map for gardener-apiserver. Hence, we reuse it for all our components.
var DefaultFeatureGate = utilfeature.DefaultMutableFeatureGate

// AllFeatureGates is the list of all feature gates.
var AllFeatureGates = map[featuregate.Feature]featuregate.FeatureSpec{
	DefaultSeccompProfile:          {Default: false, PreRelease: featuregate.Alpha},
	InPlaceNodeUpdates:             {Default: false, PreRelease: featuregate.Alpha},
	IstioTLSTermination:            {Default: false, PreRelease: featuregate.Alpha},
	CloudProfileCapabilities:       {Default: false, PreRelease: featuregate.Alpha},
	DoNotCopyBackupCredentials:     {Default: true, PreRelease: featuregate.GA, LockToDefault: true},
	OpenTelemetryCollector:         {Default: true, PreRelease: featuregate.Beta},
	VictoriaLogsBackend:            {Default: false, PreRelease: featuregate.Alpha},
	UseUnifiedHTTPProxyPort:        {Default: true, PreRelease: featuregate.Beta},
	VPAInPlaceUpdates:              {Default: true, PreRelease: featuregate.Beta},
	CustomDNSServerInNodeLocalDNS:  {Default: true, PreRelease: featuregate.Beta},
	VPNBondingModeRoundRobin:       {Default: false, PreRelease: featuregate.Alpha},
	PrometheusHealthChecks:         {Default: false, PreRelease: featuregate.Alpha},
	VersionClassificationLifecycle: {Default: false, PreRelease: featuregate.Alpha},
	RemoveVali:                     {Default: false, PreRelease: featuregate.Alpha},
	DisableNginxIngressInGarden:    {Default: false, PreRelease: featuregate.Alpha},
	DisableNginxIngressInSeed:      {Default: false, PreRelease: featuregate.Alpha},
	DisableNginxIngressInShoot:     {Default: false, PreRelease: featuregate.Alpha},
	LiveControlPlaneMigration:      {Default: false, PreRelease: featuregate.Alpha},
	BackupEntryForGarden:           {Default: false, PreRelease: featuregate.Alpha},
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
