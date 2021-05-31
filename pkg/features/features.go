// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

	// Logging enables logging stack for clusters.
	// owner @mvladev
	// alpha: v0.13.0
	Logging featuregate.Feature = "Logging"

	// HVPA enables simultaneous horizontal and vertical scaling in Seed Clusters.
	// owner @amshuman-kr
	// alpha: v0.31.0
	HVPA featuregate.Feature = "HVPA"

	// HVPAForShootedSeed enables simultaneous horizontal and vertical scaling in shooted seed Clusters.
	// owner @amshuman-kr
	// alpha: v0.32.0
	HVPAForShootedSeed featuregate.Feature = "HVPAForShootedSeed"

	// ManagedIstio installs minimal Istio components in istio-system.
	// Disable this feature if Istio is already installed in the cluster.
	// Istio is not automatically removed if this feature is set to false.
	// See https://github.com/gardener/gardener/blob/master/docs/usage/istio.md
	// owner @mvladev
	// alpha: v1.5.0
	// beta: v1.19.0
	ManagedIstio featuregate.Feature = "ManagedIstio"

	// KonnectivityTunnel enables inverting the connection direction to be shoot->seed instead of seed->shoot (only for Shoots with Kubernetes version >= 1.18).
	// owner @DockToFuture @mvladev
	// alpha: v1.6.0
	KonnectivityTunnel featuregate.Feature = "KonnectivityTunnel"

	// APIServerSNI allows to use only one LoadBalancer in the Seed cluster
	// for all Shoot clusters. Requires Istio to be installed in the cluster or
	// ManagedIstio feature gate to be enabled.
	// See https://github.com/gardener/gardener/blob/masster/docs/proposals/08-shoot-apiserver-via-sni.md
	// owner @mvladev
	// alpha: v1.7.0
	// beta: v1.19.0
	APIServerSNI featuregate.Feature = "APIServerSNI"

	// CachedRuntimeClients enables a cache in the controller-runtime clients, that Gardener uses.
	// If disabled all controller-runtime clients will directly talk to the API server instead of relying on a cache.
	// owner @tim-ebert
	// alpha: v1.7.0
	CachedRuntimeClients featuregate.Feature = "CachedRuntimeClients"

	// NodeLocalDNS enables node-local-dns cache feature.
	// owner @DockToFuture
	// alpha: v1.7.0
	NodeLocalDNS featuregate.Feature = "NodeLocalDNS"

	// MountHostCADirectories enables mounting common CA certificate directories in the Shoot API server pod that might be required for webhooks or OIDC.
	// owner @danielfoehrKn
	// alpha: v1.11.0
	MountHostCADirectories featuregate.Feature = "MountHostCADirectories"

	// SeedChange enables updating the `spec.seedName` field during shoot validation from a non-empty value
	// in order to trigger shoot control plane migration.
	// owner: @stoyanr
	// alpha: v1.12.0
	SeedChange featuregate.Feature = "SeedChange"

	// SeedKubeScheduler adds an additional kube-scheduler in seed clusters where the feature is enabled.
	// owner: @mvladev
	// alpha: v1.15.0
	SeedKubeScheduler featuregate.Feature = "SeedKubeScheduler"

	// ReversedVPN moves the openvpn server to the seed.
	// owner: @scheererj @docktofuture
	// alpha: v1.22.0
	ReversedVPN featuregate.Feature = "ReversedVPN"

	// AdminKubeconfigRequest enables the AdminKubeconfigRequest endpoint on shoot resources.
	// owner: @mvladev
	// alpha: v1.23.0
	AdminKubeconfigRequest featuregate.Feature = "AdminKubeconfigRequest"
)
