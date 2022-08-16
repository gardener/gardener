// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	// See https://github.com/gardener/gardener/blob/masster/docs/proposals/08-shoot-apiserver-via-sni.md
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
	SeedChange featuregate.Feature = "SeedChange"

	// SeedKubeScheduler adds an additional kube-scheduler in seed clusters where the feature is enabled.
	// owner: @ialidzhikov
	// alpha: v1.15.0
	SeedKubeScheduler featuregate.Feature = "SeedKubeScheduler"

	// ReversedVPN moves the openvpn server to the seed.
	// owner: @ScheererJ @DockToFuture
	// alpha: v1.22.0
	// beta: v1.42.0
	ReversedVPN featuregate.Feature = "ReversedVPN"

	// CopyEtcdBackupsDuringControlPlaneMigration enables the copy of etcd backups from the object store of the source seed
	// to the object store of the destination seed during control plane migration.
	// owner: @plkokanov
	// alpha: v1.37.0
	// beta: v1.53.0
	CopyEtcdBackupsDuringControlPlaneMigration featuregate.Feature = "CopyEtcdBackupsDuringControlPlaneMigration"

	// SecretBindingProviderValidation enables validations on Gardener API server that:
	// - requires the provider type of a SecretBinding to be set (on SecretBinding creation)
	// - requires the SecretBinding provider type to match the Shoot provider type (on Shoot creation)
	// - enforces immutability on the provider type of a SecretBinding
	// owner: @ialidzhikov
	// alpha: v1.38.0
	// beta: v1.51.0
	// GA: v1.53.0
	SecretBindingProviderValidation featuregate.Feature = "SecretBindingProviderValidation"

	// ForceRestore enables forcing the shoot's restoration to the destination seed during control plane migration
	// if the preparation for migration in the source seed is not finished after a certain grace period
	// and is considered unlikely to succeed ("bad case" scenario).
	// owner: @stoyanr
	// alpha: v1.39.0
	ForceRestore featuregate.Feature = "ForceRestore"

	// DisableDNSProviderManagement disables management of `dns.gardener.cloud/v1alpha1.DNSProvider` resources.
	// In this case, the `shoot-dns-service` extension can take this over if it is installed and following prerequisites
	// are given:
	// - The `shoot-dns-service` extension must be installed in a version >= `v1.20.0`.
	// - The controller deployment of the `shoot-dns-service` sets `providerConfig.values.dnsProviderManagement.enabled=true`
	// - Its admission controller (`gardener-extension-admission-shoot-dns-service`) is deployed on the garden cluster
	// owner: @MartinWeindel @timuthy
	// alpha: v1.41
	// beta: v1.50
	// GA: v1.52.0
	DisableDNSProviderManagement featuregate.Feature = "DisableDNSProviderManagement"

	// ShootCARotation enables the automated rotation of the shoot CA certificates.
	// owner: @rfranzke
	// alpha: v1.42.0
	// beta: v1.51.0
	ShootCARotation featuregate.Feature = "ShootCARotation"

	// ShootSARotation enables the automated rotation of the shoot service account signing key.
	// owner: @rfranzke
	// alpha: v1.48.0
	// beta: v1.51.0
	ShootSARotation featuregate.Feature = "ShootSARotation"

	// HAControlPlanes allows shoot control planes to be run in high availability mode.
	// owner: @shreyas-s-rao @timuthy
	// alpha: v1.49.0
	HAControlPlanes featuregate.Feature = "HAControlPlanes"

	// DefaultSeccompProfile defaults the seccomp profile for Gardener managed workload in the seed to RuntimeDefault.
	// owner: @dimityrmirchev
	// alpha: v1.54.0
	DefaultSeccompProfile featuregate.Feature = "DefaultSeccompProfile"
)

var allFeatureGates = map[featuregate.Feature]featuregate.FeatureSpec{
	HVPA:               {Default: false, PreRelease: featuregate.Alpha},
	HVPAForShootedSeed: {Default: false, PreRelease: featuregate.Alpha},
	ManagedIstio:       {Default: true, PreRelease: featuregate.Beta},
	APIServerSNI:       {Default: true, PreRelease: featuregate.Beta},
	SeedChange:         {Default: true, PreRelease: featuregate.Beta},
	SeedKubeScheduler:  {Default: false, PreRelease: featuregate.Alpha},
	ReversedVPN:        {Default: true, PreRelease: featuregate.Beta},
	CopyEtcdBackupsDuringControlPlaneMigration: {Default: true, PreRelease: featuregate.Beta},
	SecretBindingProviderValidation:            {Default: true, PreRelease: featuregate.GA, LockToDefault: true},
	ForceRestore:                               {Default: false, PreRelease: featuregate.Alpha},
	DisableDNSProviderManagement:               {Default: true, PreRelease: featuregate.GA, LockToDefault: true},
	ShootCARotation:                            {Default: true, PreRelease: featuregate.Beta},
	ShootSARotation:                            {Default: true, PreRelease: featuregate.Beta},
	HAControlPlanes:                            {Default: false, PreRelease: featuregate.Alpha},
	DefaultSeccompProfile:                      {Default: false, PreRelease: featuregate.Alpha},
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
