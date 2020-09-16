// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

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
	// MyFeature utilfeature.Feature = "MyFeature"

	// Logging enables logging stack for clusters.
	// owner @mvladev
	// alpha: v0.13.0
	Logging featuregate.Feature = "Logging"

	// HVPA enables simultaneous horizontal and vertical scaling in Seed Clusters.
	// owner @ggaurav10, @amshuman-kr
	// alpha: v0.31.0
	HVPA featuregate.Feature = "HVPA"

	// HVPAForShootedSeed enables simultaneous horizontal and vertical scaling in shooted seed Clusters.
	// owner @ggaurav10, @amshuman-kr
	// alpha: v0.32.0
	HVPAForShootedSeed featuregate.Feature = "HVPAForShootedSeed"

	// ManagedIstio installs minimal Istio components in istio-system.
	// Disable this feature if Istio is already installed in the cluster.
	// Istio is not automatically removed if this feature is set to false.
	// See https://github.com/gardener/gardener/blob/master/docs/usage/istio.md
	// owner @mvladev
	// alpha: v1.5.0
	ManagedIstio featuregate.Feature = "ManagedIstio"

	// KonnectivityTunnel enables inverting the connection direction to be shoot->seed instead of seed->shoot (only for Shoots with Kubernetes version >= 1.18).
	// owner @zanetworker
	// alpha: v1.6.0
	KonnectivityTunnel featuregate.Feature = "KonnectivityTunnel"

	// APIServerSNI allows to use only one LoadBalancer in the Seed cluster
	// for all Shoot clusters. Requires Istio to be installed in the cluster or
	// ManagedIstio feature gate to be enabled.
	// See https://github.com/gardener/gardener/blob/masster/docs/proposals/08-shoot-apiserver-via-sni.md
	// owner @mvladev
	// alpha: v1.7.0
	APIServerSNI featuregate.Feature = "APIServerSNI"

	// CachedRuntimeClients enables a cache in the controller-runtime clients, that Gardener uses.
	// If disabled all controller-runtime clients will directly talk to the API server instead of relying on a cache.
	// owner @tim-ebert
	// alpha: v1.7.0
	CachedRuntimeClients featuregate.Feature = "CachedRuntimeClients"

	// NodeLocalDNS enables node-local-dns cache feature.
	// owner @zanetworker
	// alpha: v1.7.0
	NodeLocalDNS featuregate.Feature = "NodeLocalDNS"
)
