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

package deployment_test

import (
	"net"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	. "github.com/gardener/gardener/pkg/operation/botanist/controlplane/kubeapiserver/deployment"

	"github.com/Masterminds/semver"
	. "github.com/onsi/gomega"
)

// KubeAPIServerBuilder is a builder for the KubeAPIServer component for testing purposes
type KubeAPIServerBuilder interface {
	SetAPIServerConfig(*gardencorev1beta1.KubeAPIServerConfig) KubeAPIServerBuilder
	SetManagedSeedAPIServer(server *gardencorev1beta1helper.ShootedSeedAPIServer) KubeAPIServerBuilder
	SetHibernationEnabled(bool) KubeAPIServerBuilder
	SetKonnectivityTunnelEnabled(bool) KubeAPIServerBuilder
	SetEtcdEncryptionEnabled(bool) KubeAPIServerBuilder
	SetBasicAuthenticationEnabled(bool) KubeAPIServerBuilder
	SetHvpaEnabled(bool) KubeAPIServerBuilder
	SetMountHostCADirectories(bool) KubeAPIServerBuilder
	SetShootHasDeletionTimestamp(bool) KubeAPIServerBuilder
	SetShootKubernetesVersion(string) KubeAPIServerBuilder
	SetSeedKubernetesVersion(string) KubeAPIServerBuilder
	SetNodeNetwork(*net.IPNet) KubeAPIServerBuilder
	SetShootAnnotations(map[string]string) KubeAPIServerBuilder
	SetMaintenanceWindow(*gardencorev1beta1.MaintenanceTimeWindow) KubeAPIServerBuilder
	SetSNIValues(*APIServerSNIValues) KubeAPIServerBuilder
	// implies that the apiserver deployment already exists
	// used in the deploy function to calculate apiserver resources and replicas
	SetDeploymentReplicas(int32) KubeAPIServerBuilder
	SetMinimumNodeCount(int32) KubeAPIServerBuilder
	SetMaximumNodeCount(int32) KubeAPIServerBuilder
	Build() (KubeAPIServer, KubeAPIServerValuesProvider)
}

type KubeAPIServerValuesProvider interface {
	GetAPIServerConfig() *gardencorev1beta1.KubeAPIServerConfig
	GetManagedSeedAPIServer() *gardencorev1beta1helper.ShootedSeedAPIServer
	IsHibernationEnabled() bool
	IsKonnectivityTunnelEnabled() bool
	IsEtcdEncryptionEnabled() bool
	IsBasicAuthenticationEnabled() bool
	IsHvpaEnabled() bool
	IsSNIEnabled() bool
	IsSNIPodMutatorEnabled() bool
	MountHostCADirectories() bool
	ShootHasDeletionTimestamp(bool) bool
	GetShootKubernetesVersion() string
	GetSeedKubernetesVersion() string
	GetNodeNetwork() *net.IPNet
	GetShootAnnotations() map[string]string
	GetMaintenanceWindow() *gardencorev1beta1.MaintenanceTimeWindow
	GetSNIValues() *APIServerSNIValues
	GetDeploymentReplicas() *int32
	GetMinimumNodeCount() int32
	GetMaximumNodeCount() int32
	DeploymentAlreadyExists() bool
}

type kubeAPIServerTesting struct {
	config                     *gardencorev1beta1.KubeAPIServerConfig
	managedSeedAPIServer       *gardencorev1beta1helper.ShootedSeedAPIServer
	hibernationEnabled         bool
	konnectivityTunnelEnabled  bool
	etcdEncryptionEnabled      bool
	basicAuthenticationEnabled bool
	hvpaEnabled                bool
	mountHostCADirectories     bool
	shootHasDeletionTimestamp  bool

	shootKubernetesVersion string
	seedKubernetesVersion  string
	nodeNetwork            *net.IPNet
	shootAnnotations       map[string]string
	maintenanceWindow      *gardencorev1beta1.MaintenanceTimeWindow
	sniValues              *APIServerSNIValues
	deploymentReplicas     *int32
	minimumNodeCount       *int32
	maximumNodeCount       *int32
}

func NewAPIServerBuilder() KubeAPIServerBuilder {
	return &kubeAPIServerTesting{}
}

func (k *kubeAPIServerTesting) SetAPIServerConfig(config *gardencorev1beta1.KubeAPIServerConfig) KubeAPIServerBuilder {
	k.config = config
	return k
}

func (k *kubeAPIServerTesting) SetManagedSeedAPIServer(s *gardencorev1beta1helper.ShootedSeedAPIServer) KubeAPIServerBuilder {
	k.managedSeedAPIServer = s
	return k
}

func (k *kubeAPIServerTesting) SetHibernationEnabled(e bool) KubeAPIServerBuilder {
	k.hibernationEnabled = e
	return k
}

func (k *kubeAPIServerTesting) SetKonnectivityTunnelEnabled(e bool) KubeAPIServerBuilder {
	k.konnectivityTunnelEnabled = e
	return k
}

func (k *kubeAPIServerTesting) SetEtcdEncryptionEnabled(e bool) KubeAPIServerBuilder {
	k.etcdEncryptionEnabled = e
	return k
}

func (k *kubeAPIServerTesting) SetBasicAuthenticationEnabled(e bool) KubeAPIServerBuilder {
	k.basicAuthenticationEnabled = e
	return k
}

func (k *kubeAPIServerTesting) SetHvpaEnabled(e bool) KubeAPIServerBuilder {
	k.hvpaEnabled = e
	return k
}

func (k *kubeAPIServerTesting) SetMountHostCADirectories(e bool) KubeAPIServerBuilder {
	k.mountHostCADirectories = e
	return k
}

func (k *kubeAPIServerTesting) SetShootHasDeletionTimestamp(e bool) KubeAPIServerBuilder {
	k.shootHasDeletionTimestamp = e
	return k
}

func (k *kubeAPIServerTesting) SetShootKubernetesVersion(v string) KubeAPIServerBuilder {
	k.shootKubernetesVersion = v
	return k
}

func (k *kubeAPIServerTesting) SetSeedKubernetesVersion(v string) KubeAPIServerBuilder {
	k.seedKubernetesVersion = v
	return k
}

func (k *kubeAPIServerTesting) SetNodeNetwork(ipNet *net.IPNet) KubeAPIServerBuilder {
	k.nodeNetwork = ipNet
	return k
}

func (k *kubeAPIServerTesting) SetShootAnnotations(m map[string]string) KubeAPIServerBuilder {
	k.shootAnnotations = m
	return k
}

func (k *kubeAPIServerTesting) SetMaintenanceWindow(window *gardencorev1beta1.MaintenanceTimeWindow) KubeAPIServerBuilder {
	k.maintenanceWindow = window
	return k
}

func (k *kubeAPIServerTesting) SetDeploymentReplicas(r int32) KubeAPIServerBuilder {
	k.deploymentReplicas = &r
	return k
}

func (k *kubeAPIServerTesting) SetMinimumNodeCount(r int32) KubeAPIServerBuilder {
	k.minimumNodeCount = &r
	return k
}

func (k *kubeAPIServerTesting) SetMaximumNodeCount(r int32) KubeAPIServerBuilder {
	k.maximumNodeCount = &r
	return k
}

func (k *kubeAPIServerTesting) SetSNIValues(values *APIServerSNIValues) KubeAPIServerBuilder {
	k.sniValues = values
	return k
}

func (k *kubeAPIServerTesting) Build() (KubeAPIServer, KubeAPIServerValuesProvider) {
	if len(k.shootKubernetesVersion) == 0 {
		k.shootKubernetesVersion = defaultShootK8sVersion
	}

	if len(k.seedKubernetesVersion) == 0 {
		k.seedKubernetesVersion = defaultSeedK8sVersion
	}

	semVer, err := semver.NewVersion(k.shootKubernetesVersion)
	Expect(err).ToNot(HaveOccurred())

	if k.sniValues == nil {
		k.sniValues = &APIServerSNIValues{}
	}

	if k.minimumNodeCount == nil {
		k.minimumNodeCount = &defaultMinNodeCount
	}

	if k.maximumNodeCount == nil {
		k.maximumNodeCount = &defaultMaxNodeCount
	}

	apiServer := New(
		k.config,
		k.managedSeedAPIServer,
		mockSeedInterface,
		mockGardenClient,
		semVer,
		defaultSeedNamespace,
		defaultGardenNamespace,
		k.hibernationEnabled,
		k.konnectivityTunnelEnabled,
		k.etcdEncryptionEnabled,
		k.basicAuthenticationEnabled,
		k.hvpaEnabled,
		k.mountHostCADirectories,
		k.shootHasDeletionTimestamp,
		defaultServiceNetwork,
		defaultPodNetwork,
		k.nodeNetwork,
		*k.minimumNodeCount,
		*k.maximumNodeCount,
		k.shootAnnotations,
		k.maintenanceWindow,
		*k.sniValues,
		APIServerImages{
			KubeAPIServerImageName:                   apiServerImageName,
			AlpineIptablesImageName:                  alpineIptablesImageName,
			VPNSeedImageName:                         vpnSeedImageName,
			KonnectivityServerTunnelImageName:        konnectivityServerTunnelImageName,
			ApiServerProxyPodMutatorWebhookImageName: apiServerProxyPodMutatorWebhookImageName,
		},
	)

	// set required configuration
	apiServer.SetSecrets(Secrets{
		CA:                           component.Secret{Name: "ca", Checksum: checksumCA},
		CAFrontProxy:                 component.Secret{Name: "ca-front-proxy", Checksum: checksumCaFrontProxy},
		TLSServer:                    component.Secret{Name: "kube-apiserver", Checksum: checksumTLSServer},
		KubeAggregator:               component.Secret{Name: "kube-aggregator", Checksum: checksumKubeAggregator},
		KubeAPIServerKubelet:         component.Secret{Name: "kube-apiserver-kubelet", Checksum: checksumKubeAPIServerKubelet},
		StaticToken:                  component.Secret{Name: "static-token", Checksum: checksumStaticToken},
		ServiceAccountKey:            component.Secret{Name: "service-account-key", Checksum: checksumServiceAccountKey},
		EtcdCA:                       component.Secret{Name: "ca-etcd", Checksum: checksumCAEtcd},
		EtcdClientTLS:                component.Secret{Name: "etcd-client-tls", Checksum: checksumETCDClientTLS},
		BasicAuth:                    component.Secret{Name: "kube-apiserver-basic-auth", Checksum: checksumBasicAuth},
		EtcdEncryption:               component.Secret{Name: "etcd-encryption-secret", Checksum: checksumETCDEncryptionSecret},
		KonnectivityServerCerts:      component.Secret{Name: "konnectivity-server", Checksum: checksumKonnectivityServer},
		KonnectivityServerKubeconfig: component.Secret{Name: "konnectivity-server-kubeconfig", Checksum: checksumKonnectivityServerKubeconfig},
		KonnectivityServerClientTLS:  component.Secret{Name: "konnectivity-server-client-tls", Checksum: checksumKonnectivityServerClientTLS},
		VpnSeed:                      component.Secret{Name: "vpn-seed", Checksum: checksumVPNSeed},
		VpnSeedTLSAuth:               component.Secret{Name: "vpn-seed-tlsauth", Checksum: checksumVPNSeedTLSAuth},
	})
	apiServer.SetHealthCheckToken(defaultHealthCheckToken)
	apiServer.SetShootOutOfClusterAPIServerAddress(defaultShootOutOfClusterAPIServerAddress)

	if k.sniValues.SNIEnabled {
		apiServer.SetShootAPIServerClusterIP(defaultShootAPIServerClusterIP)
	}

	return apiServer, k
}

// Values provider
func (k *kubeAPIServerTesting) GetAPIServerConfig() *gardencorev1beta1.KubeAPIServerConfig {
	return k.config
}

func (k *kubeAPIServerTesting) GetManagedSeedAPIServer() *gardencorev1beta1helper.ShootedSeedAPIServer {
	return k.managedSeedAPIServer
}

func (k *kubeAPIServerTesting) IsHibernationEnabled() bool {
	return k.hibernationEnabled
}

func (k *kubeAPIServerTesting) IsKonnectivityTunnelEnabled() bool {
	return k.konnectivityTunnelEnabled
}

func (k *kubeAPIServerTesting) IsEtcdEncryptionEnabled() bool {
	return k.etcdEncryptionEnabled
}

func (k *kubeAPIServerTesting) IsBasicAuthenticationEnabled() bool {
	return k.basicAuthenticationEnabled
}

func (k *kubeAPIServerTesting) IsHvpaEnabled() bool {
	return k.hvpaEnabled
}

func (k *kubeAPIServerTesting) IsSNIEnabled() bool {
	return k.sniValues != nil && k.sniValues.SNIEnabled
}

func (k *kubeAPIServerTesting) IsSNIPodMutatorEnabled() bool {
	return k.sniValues != nil && k.sniValues.SNIPodMutatorEnabled
}

func (k *kubeAPIServerTesting) MountHostCADirectories() bool {
	return k.mountHostCADirectories
}

func (k *kubeAPIServerTesting) ShootHasDeletionTimestamp(b2 bool) bool {
	return k.shootHasDeletionTimestamp
}

func (k *kubeAPIServerTesting) GetShootKubernetesVersion() string {
	return k.shootKubernetesVersion
}

func (k *kubeAPIServerTesting) GetSeedKubernetesVersion() string {
	return k.seedKubernetesVersion
}

func (k *kubeAPIServerTesting) GetNodeNetwork() *net.IPNet {
	return k.nodeNetwork
}

func (k *kubeAPIServerTesting) GetShootAnnotations() map[string]string {
	return k.shootAnnotations
}

func (k *kubeAPIServerTesting) GetMaintenanceWindow() *gardencorev1beta1.MaintenanceTimeWindow {
	return k.maintenanceWindow
}

func (k *kubeAPIServerTesting) GetSNIValues() *APIServerSNIValues {
	return k.sniValues
}

func (k *kubeAPIServerTesting) GetDeploymentReplicas() *int32 {
	return k.deploymentReplicas
}

func (k *kubeAPIServerTesting) GetMinimumNodeCount() int32 {
	return *k.minimumNodeCount
}

func (k *kubeAPIServerTesting) GetMaximumNodeCount() int32 {
	return *k.maximumNodeCount
}

func (k *kubeAPIServerTesting) DeploymentAlreadyExists() bool {
	return k.deploymentReplicas != nil
}
