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
	ManagedIstio featuregate.Feature = "ManagedIstio"

	// APIServerSNI allows to use only one LoadBalancer in the Seed cluster
	// for all Shoot clusters. Requires Istio to be installed in the cluster or
	// ManagedIstio feature gate to be enabled.
	// See https://github.com/gardener/gardener/blob/masster/docs/proposals/08-shoot-apiserver-via-sni.md
	// owner @ScheererJ @DockToFuture
	// alpha: v1.7.0
	// beta: v1.19.0
	APIServerSNI featuregate.Feature = "APIServerSNI"

	// CachedRuntimeClients enables a cache in the controller-runtime clients, that Gardener uses.
	// owner @timebertt
	// alpha: v1.7.0
	// beta: v1.34.0
	// GA: v1.45.0
	CachedRuntimeClients featuregate.Feature = "CachedRuntimeClients"

	// SeedChange enables updating the `spec.seedName` field during shoot validation from a non-empty value
	// in order to trigger shoot control plane migration.
	// owner: @stoyanr
	// alpha: v1.12.0
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

	// AdminKubeconfigRequest enables the AdminKubeconfigRequest endpoint on shoot resources.
	// owner: @petersutter
	// alpha: v1.24.0
	// beta: v1.39.0
	// GA: v1.42.0
	AdminKubeconfigRequest featuregate.Feature = "AdminKubeconfigRequest"

	// UseDNSRecords enables using DNSRecords resources for Gardener DNS records instead of DNSProvider and DNSEntry resources.
	// owner: @stoyanr
	// alpha: v1.27.0
	// beta: v1.39.0
	// GA: v1.44.0
	UseDNSRecords featuregate.Feature = "UseDNSRecords"

	// RotateSSHKeypairOnMaintenance enables SSH keypair rotation in the maintenance controller of the gardener-controller-manager.
	// owner: @petersutter
	// alpha: v1.28.0
	RotateSSHKeypairOnMaintenance featuregate.Feature = "RotateSSHKeypairOnMaintenance"

	// DenyInvalidExtensionResources causes the seed-admission-controller to deny invalid extension resources (instead of just logging validation errors).
	// owner: @stoyanr
	// alpha: v1.31.0
	// beta: v1.42.0
	// GA: v1.45.0
	DenyInvalidExtensionResources featuregate.Feature = "DenyInvalidExtensionResources"

	// WorkerPoolKubernetesVersion allows to overwrite the Kubernetes version used for shoot clusters per worker pool.
	// owner: @rfranzke @majst01 @mwennrich
	// alpha: v1.35.0
	WorkerPoolKubernetesVersion featuregate.Feature = "WorkerPoolKubernetesVersion"

	// CopyEtcdBackupsDuringControlPlaneMigration enables the copy of etcd backups from the object store of the source seed
	// to the object store of the destination seed during control plane migration.
	// owner: @plkokanov
	// alpha: v1.37.0
	CopyEtcdBackupsDuringControlPlaneMigration featuregate.Feature = "CopyEtcdBackupsDuringControlPlaneMigration"

	// SecretBindingProviderValidation enables validations on Gardener API server that:
	// - requires the provider type of a SecretBinding to be set (on SecretBinding creation)
	// - requires the SecretBinding provider type to match the Shoot provider type (on Shoot creation)
	// - enforces immutability on the provider type of a SecretBinding
	// owner: @ialidzhikov
	// alpha: v1.38.0
	SecretBindingProviderValidation featuregate.Feature = "SecretBindingProviderValidation"

	// ForceRestore enables forcing the shoot's restoration to the destination seed during control plane migration
	// if the preparation for migration in the source seed is not finished after a certain grace period
	// and is considered unlikely to succeed ("bad case" scenario).
	// owner: @stoyanr
	// alpha: v1.39.0
	ForceRestore featuregate.Feature = "ForceRestore"

	// DisableDNSProviderManagement disables management of `dns.gardener.cloud/v1alpha1.DNSProvider` resources. In this case, the `shoot-dns-service` extension will take this over if it is installed.
	// Only supported if feature `UseDNSRecords` is set to true.
	// owner: @MartinWeindel @timuthy
	// alpha: v1.41
	DisableDNSProviderManagement featuregate.Feature = "DisableDNSProviderManagement"

	// ShootCARotation enables the automated rotation of the shoot CA certificates.
	// owner: @rfranzke
	// alpha: v1.42.0
	ShootCARotation featuregate.Feature = "ShootCARotation"

	// ShootMaxTokenExpirationOverwrite makes the Gardener API server overwriting values in the
	// `.spec.kubernetes.kubeAPIServer.serviceAccountConfig.maxTokenExpiration` field of Shoot specifications to
	// - be at least 720h (30d) when the current value is lower
	// - be at most 2160h (90d) when the current value is higher
	// before persisting the object to etcd.
	// owner: @rfranzke
	// alpha: v1.43.0
	// beta: v1.45.0
	ShootMaxTokenExpirationOverwrite featuregate.Feature = "ShootMaxTokenExpirationOverwrite"

	// ShootMaxTokenExpirationValidation enables validations on Gardener API server that enforce that the value of the
	// `.spec.kubernetes.kubeAPIServer.serviceAccountConfig.maxTokenExpiration` field
	// - is at least 720h (30d).
	// - is at most 2160h (90d).
	// Only enable this after ShootMaxTokenExpirationOverwrite is enabled and all shoots got updated accordingly.
	// owner: @rfranzke
	// alpha: v1.43.0
	ShootMaxTokenExpirationValidation featuregate.Feature = "ShootMaxTokenExpirationValidation"
)

var allFeatureGates = map[featuregate.Feature]featuregate.FeatureSpec{
	HVPA:                          {Default: false, PreRelease: featuregate.Alpha},
	HVPAForShootedSeed:            {Default: false, PreRelease: featuregate.Alpha},
	ManagedIstio:                  {Default: true, PreRelease: featuregate.Beta},
	APIServerSNI:                  {Default: true, PreRelease: featuregate.Beta},
	CachedRuntimeClients:          {Default: true, PreRelease: featuregate.GA, LockToDefault: true},
	SeedChange:                    {Default: false, PreRelease: featuregate.Alpha},
	SeedKubeScheduler:             {Default: false, PreRelease: featuregate.Alpha},
	ReversedVPN:                   {Default: true, PreRelease: featuregate.Beta},
	AdminKubeconfigRequest:        {Default: true, PreRelease: featuregate.GA, LockToDefault: true},
	UseDNSRecords:                 {Default: true, PreRelease: featuregate.GA, LockToDefault: true},
	RotateSSHKeypairOnMaintenance: {Default: false, PreRelease: featuregate.Alpha},
	DenyInvalidExtensionResources: {Default: true, PreRelease: featuregate.GA, LockToDefault: true},
	WorkerPoolKubernetesVersion:   {Default: false, PreRelease: featuregate.Alpha},
	CopyEtcdBackupsDuringControlPlaneMigration: {Default: false, PreRelease: featuregate.Alpha},
	SecretBindingProviderValidation:            {Default: false, PreRelease: featuregate.Alpha},
	ForceRestore:                               {Default: false, PreRelease: featuregate.Alpha},
	DisableDNSProviderManagement:               {Default: false, PreRelease: featuregate.Alpha},
	ShootCARotation:                            {Default: false, PreRelease: featuregate.Alpha},
	ShootMaxTokenExpirationOverwrite:           {Default: true, PreRelease: featuregate.Beta},
	ShootMaxTokenExpirationValidation:          {Default: false, PreRelease: featuregate.Alpha},
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
