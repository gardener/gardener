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

package botanist

import (
	"context"
	"fmt"
	"net"

	hvpav1alpha1 "github.com/gardener/hvpa-controller/api/v1alpha1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/features"
	gardenletfeatures "github.com/gardener/gardener/pkg/gardenlet/features"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/operation/botanist/controlplane/etcd"
	"github.com/gardener/gardener/pkg/operation/botanist/controlplane/konnectivity"
	kubeapiserverdeployment "github.com/gardener/gardener/pkg/operation/botanist/controlplane/kubeapiserver/deployment"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/version"
)

// DefaultKubeAPIServer returns a deployer for the kube-apiserver.
func (b *Botanist) DefaultKubeAPIServer() (kubeapiserverdeployment.KubeAPIServer, error) {
	sniValues := kubeapiserverdeployment.APIServerSNIValues{
		SNIEnabled: b.APIServerSNIEnabled(),
	}

	if b.APIServerSNIEnabled() {
		sniValues.SNIPodMutatorEnabled = b.APIServerSNIPodMutatorEnabled()
	}

	images, err := b.getAPIServerImages(sniValues.SNIPodMutatorEnabled)
	if err != nil {
		return nil, err
	}

	etcdEncryptionEnabled, err := version.CheckVersionMeetsConstraint(b.Shoot.Info.Spec.Kubernetes.Version, ">= 1.13")
	if err != nil {
		return nil, err
	}

	var hvpaEnabled = gardenletfeatures.FeatureGate.Enabled(features.HVPA)
	if b.ManagedSeed != nil {
		// Override for shooted seeds
		hvpaEnabled = gardenletfeatures.FeatureGate.Enabled(features.HVPAForShootedSeed)
	}

	var nodeNetwork *net.IPNet
	if b.Shoot.GetNodeNetwork() != nil {
		_, nodeNetwork, err = net.ParseCIDR(*b.Shoot.GetNodeNetwork())
		if err != nil {
			return nil, fmt.Errorf("failed to parse Node CIDR %q", *b.Shoot.GetNodeNetwork())
		}
	}

	return kubeapiserverdeployment.New(
		b.Shoot.Info.Spec.Kubernetes.KubeAPIServer,
		b.ManagedSeedAPIServer,
		b.K8sSeedClient,
		b.K8sGardenClient.Client(),
		b.Shoot.KubernetesVersion,
		b.Shoot.SeedNamespace,
		b.Shoot.Info.Namespace,
		b.Shoot.HibernationEnabled,
		b.Shoot.KonnectivityTunnelEnabled,
		etcdEncryptionEnabled,
		gardencorev1beta1helper.ShootWantsBasicAuthentication(b.Shoot.Info),
		hvpaEnabled,
		gardenletfeatures.FeatureGate.Enabled(features.MountHostCADirectories),
		b.Shoot.Info.DeletionTimestamp != nil,
		b.Shoot.Networks.Services,
		b.Shoot.Networks.Pods,
		nodeNetwork,
		b.Shoot.GetMinNodeCount(),
		b.Shoot.GetMaxNodeCount(),
		b.Shoot.Info.Annotations,
		b.Shoot.Info.Spec.Maintenance.TimeWindow,
		sniValues,
		images,
	), nil
}

// DeployKubeControllerManager deploys the Shoot Kubernetes API Server and resources the kube-apiserver deployment depends on.
func (b *Botanist) DeployKubeAPIServer(ctx context.Context) error {
	b.Shoot.Components.ControlPlane.KubeAPIServer.SetSecrets(kubeapiserverdeployment.Secrets{
		CA:                           component.Secret{Name: v1beta1constants.SecretNameCACluster, Checksum: b.CheckSums[v1beta1constants.SecretNameCACluster]},
		CAFrontProxy:                 component.Secret{Name: kubeapiserverdeployment.SecretNameCAFrontProxy, Checksum: b.CheckSums[kubeapiserverdeployment.SecretNameCAFrontProxy]},
		TLSServer:                    component.Secret{Name: kubeapiserverdeployment.SecretNameTLSServer, Checksum: b.CheckSums[kubeapiserverdeployment.SecretNameTLSServer]},
		KubeAggregator:               component.Secret{Name: kubeapiserverdeployment.SecretNameKubeAggregator, Checksum: b.CheckSums[kubeapiserverdeployment.SecretNameKubeAggregator]},
		KubeAPIServerKubelet:         component.Secret{Name: kubeapiserverdeployment.SecretNameKubeAPIserverKubelet, Checksum: b.CheckSums[kubeapiserverdeployment.SecretNameKubeAPIserverKubelet]},
		StaticToken:                  component.Secret{Name: kubeapiserverdeployment.StaticTokenSecretName, Checksum: b.CheckSums[kubeapiserverdeployment.StaticTokenSecretName]},
		ServiceAccountKey:            component.Secret{Name: v1beta1constants.SecretNameServiceAccountKey, Checksum: b.CheckSums[v1beta1constants.SecretNameServiceAccountKey]},
		EtcdCA:                       component.Secret{Name: v1beta1constants.SecretNameCAETCD, Checksum: b.CheckSums[v1beta1constants.SecretNameCAETCD]},
		EtcdClientTLS:                component.Secret{Name: etcd.SecretNameClient, Checksum: b.CheckSums[etcd.SecretNameClient]},
		BasicAuth:                    component.Secret{Name: kubeapiserverdeployment.BasicAuthSecretName, Checksum: b.CheckSums[kubeapiserverdeployment.BasicAuthSecretName]},
		EtcdEncryption:               component.Secret{Name: common.EtcdEncryptionSecretName, Checksum: b.CheckSums[common.EtcdEncryptionSecretName]},
		KonnectivityServerCerts:      component.Secret{Name: konnectivity.SecretNameServerTLS, Checksum: b.CheckSums[konnectivity.SecretNameServerTLS]},
		KonnectivityServerKubeconfig: component.Secret{Name: konnectivity.SecretNameServerKubeconfig, Checksum: b.CheckSums[konnectivity.SecretNameServerKubeconfig]},
		KonnectivityServerClientTLS:  component.Secret{Name: konnectivity.SecretNameServerTLSClient, Checksum: b.CheckSums[konnectivity.SecretNameServerTLSClient]},
		VpnSeed:                      component.Secret{Name: kubeapiserverdeployment.SecretNameVPNSeed, Checksum: b.CheckSums[kubeapiserverdeployment.SecretNameVPNSeed]},
		VpnSeedTLSAuth:               component.Secret{Name: kubeapiserverdeployment.SecretNameVPNSeedTLSAuth, Checksum: b.CheckSums[kubeapiserverdeployment.SecretNameVPNSeedTLSAuth]},
	})

	b.Shoot.Components.ControlPlane.KubeAPIServer.SetHealthCheckToken(b.APIServerHealthCheckToken)

	if b.APIServerSNIEnabled() {
		b.Shoot.Components.ControlPlane.KubeAPIServer.SetShootAPIServerClusterIP(b.APIServerClusterIP)
	}

	b.Shoot.Components.ControlPlane.KubeAPIServer.SetShootOutOfClusterAPIServerAddress(b.Shoot.ComputeOutOfClusterAPIServerAddress(b.APIServerAddress, true))

	return b.Shoot.Components.ControlPlane.KubeAPIServer.Deploy(ctx)
}

func (b *Botanist) getAPIServerImages(apiServerSNIPodMutatorEnabled bool) (kubeapiserverdeployment.APIServerImages, error) {
	images := kubeapiserverdeployment.APIServerImages{}

	kubeAPIServerImageName, err := b.ImageVector.FindImage(common.KubeAPIServerImageName, imagevector.RuntimeVersion(b.SeedVersion()), imagevector.TargetVersion(b.ShootVersion()))
	if err != nil {
		return images, err
	}
	images.KubeAPIServerImageName = kubeAPIServerImageName.String()

	if b.Shoot.KonnectivityTunnelEnabled && !b.APIServerSNIEnabled() {
		konnectivityServerTunnelImageName, err := b.ImageVector.FindImage(konnectivity.ServerImageName, imagevector.RuntimeVersion(b.SeedVersion()), imagevector.TargetVersion(b.ShootVersion()))
		if err != nil {
			return images, err
		}
		images.KonnectivityServerTunnelImageName = konnectivityServerTunnelImageName.String()
	} else {
		vpnSeedImage, err := b.ImageVector.FindImage(common.VPNSeedImageName, imagevector.RuntimeVersion(b.SeedVersion()), imagevector.TargetVersion(b.ShootVersion()))
		if err != nil {
			return images, err
		}
		images.VPNSeedImageName = vpnSeedImage.String()

		alpineIptablesImageName, err := b.ImageVector.FindImage(common.AlpineIptablesImageName, imagevector.RuntimeVersion(b.SeedVersion()), imagevector.TargetVersion(b.ShootVersion()))
		if err != nil {
			return images, err
		}
		images.AlpineIptablesImageName = alpineIptablesImageName.String()
	}

	if apiServerSNIPodMutatorEnabled {
		apiServerProxyPodMutatorWebhookImageName, err := b.ImageVector.FindImage(common.APIServerProxyPodMutatorWebhookImageName, imagevector.RuntimeVersion(b.SeedVersion()), imagevector.TargetVersion(b.ShootVersion()))
		if err != nil {
			return images, err
		}
		images.ApiServerProxyPodMutatorWebhookImageName = apiServerProxyPodMutatorWebhookImageName.String()
	}
	return images, nil
}

// WakeUpKubeAPIServer creates a service and ensures API Server is scaled up
func (b *Botanist) WakeUpKubeAPIServer(ctx context.Context) error {
	sniPhase := b.Shoot.Components.ControlPlane.KubeAPIServerSNIPhase.Done()

	if err := b.DeployKubeAPIService(ctx, sniPhase); err != nil {
		return err
	}
	if err := b.Shoot.Components.ControlPlane.KubeAPIServerService.Wait(ctx); err != nil {
		return err
	}
	if b.APIServerSNIEnabled() {
		if err := b.DeployKubeAPIServerSNI(ctx); err != nil {
			return err
		}
	}
	if err := b.DeployKubeAPIServer(ctx); err != nil {
		return err
	}
	if err := kubernetes.ScaleDeployment(ctx, b.K8sSeedClient.Client(), kutil.Key(b.Shoot.SeedNamespace, v1beta1constants.DeploymentNameKubeAPIServer), 1); err != nil {
		return err
	}
	if err := b.Shoot.Components.ControlPlane.KubeAPIServer.Wait(ctx); err != nil {
		return err
	}

	return nil
}

// ScaleKubeAPIServerToOne scales kube-apiserver replicas to one
func (b *Botanist) ScaleKubeAPIServerToOne(ctx context.Context) error {
	return kubernetes.ScaleDeployment(ctx, b.K8sSeedClient.DirectClient(), kutil.Key(b.Shoot.SeedNamespace, v1beta1constants.DeploymentNameKubeAPIServer), 1)
}

// DeleteKubeAPIServer deletes the kube-apiserver kubeapiserverdeployment in the Seed cluster which holds the Shoot's control plane.
func (b *Botanist) DeleteKubeAPIServer(ctx context.Context) error {
	// invalidate shoot client here before deleting API server
	if err := b.ClientMap.InvalidateClient(keys.ForShoot(b.Shoot.Info)); err != nil {
		return err
	}
	b.K8sShootClient = nil
	return b.Shoot.Components.ControlPlane.KubeAPIServer.Destroy(ctx)
}

// PrepareKubeAPIServerForMigration deletes the kube-apiserver and deletes its hvpa
func (b *Botanist) PrepareKubeAPIServerForMigration(ctx context.Context) error {
	if err := b.K8sSeedClient.Client().Delete(ctx, &hvpav1alpha1.Hvpa{ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.DeploymentNameKubeAPIServer, Namespace: b.Shoot.SeedNamespace}}); client.IgnoreNotFound(err) != nil && !meta.IsNoMatchError(err) {
		return err
	}

	return b.DeleteKubeAPIServer(ctx)
}
