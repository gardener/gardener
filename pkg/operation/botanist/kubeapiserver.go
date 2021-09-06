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

	"github.com/gardener/gardener/charts"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/features"
	gardenletfeatures "github.com/gardener/gardener/pkg/gardenlet/features"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/operation/botanist/component/etcd"
	"github.com/gardener/gardener/pkg/operation/botanist/component/kubeapiserver"
	"github.com/gardener/gardener/pkg/operation/botanist/component/vpnseedserver"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

// DefaultKubeAPIServer returns a deployer for the kube-apiserver.
func (b *Botanist) DefaultKubeAPIServer(ctx context.Context) (kubeapiserver.Interface, error) {
	images, err := b.computeKubeAPIServerImages()
	if err != nil {
		return nil, err
	}

	var (
		admissionPlugins     = kutil.GetAdmissionPluginsForVersion(b.Shoot.GetInfo().Spec.Kubernetes.Version)
		auditConfig          *kubeapiserver.AuditConfig
		oidcConfig           *gardencorev1beta1.OIDCConfig
		serviceAccountConfig *kubeapiserver.ServiceAccountConfig
	)

	if apiServerConfig := b.Shoot.GetInfo().Spec.Kubernetes.KubeAPIServer; apiServerConfig != nil {
		admissionPlugins = b.computeKubeAPIServerAdmissionPlugins(admissionPlugins, apiServerConfig.AdmissionPlugins)

		auditConfig, err = b.computeKubeAPIServerAuditConfig(ctx, apiServerConfig.AuditConfig)
		if err != nil {
			return nil, err
		}

		oidcConfig = apiServerConfig.OIDCConfig

		serviceAccountConfig, err = b.computeKubeAPIServerServiceAccountConfig(ctx, apiServerConfig.ServiceAccountConfig)
		if err != nil {
			return nil, err
		}
	}

	return kubeapiserver.New(
		b.K8sSeedClient,
		b.Shoot.SeedNamespace,
		kubeapiserver.Values{
			AdmissionPlugins:           admissionPlugins,
			Audit:                      auditConfig,
			Autoscaling:                b.computeKubeAPIServerAutoscalingConfig(),
			BasicAuthenticationEnabled: gardencorev1beta1helper.ShootWantsBasicAuthentication(b.Shoot.GetInfo()),
			Images:                     images,
			OIDC:                       oidcConfig,
			ServiceAccountConfig:       serviceAccountConfig,
			SNI:                        b.computeKubeAPIServerSNIConfig(),
			Version:                    b.Shoot.KubernetesVersion,
			VPN: kubeapiserver.VPNConfig{
				ReversedVPNEnabled: b.Shoot.ReversedVPNEnabled,
				PodNetworkCIDR:     b.Shoot.Networks.Pods.String(),
				ServiceNetworkCIDR: b.Shoot.Networks.Services.String(),
				NodeNetworkCIDR:    b.Shoot.GetInfo().Spec.Networking.Nodes,
			},
		},
	), nil
}

func (b *Botanist) computeKubeAPIServerAdmissionPlugins(defaultPlugins, configuredPlugins []gardencorev1beta1.AdmissionPlugin) []gardencorev1beta1.AdmissionPlugin {
	for _, plugin := range configuredPlugins {
		pluginOverwritesDefault := false

		for i, defaultPlugin := range defaultPlugins {
			if defaultPlugin.Name == plugin.Name {
				pluginOverwritesDefault = true
				defaultPlugins[i] = plugin
				break
			}
		}

		if !pluginOverwritesDefault {
			defaultPlugins = append(defaultPlugins, plugin)
		}
	}

	return defaultPlugins
}

func (b *Botanist) computeKubeAPIServerAuditConfig(ctx context.Context, config *gardencorev1beta1.AuditConfig) (*kubeapiserver.AuditConfig, error) {
	if config == nil || config.AuditPolicy == nil || config.AuditPolicy.ConfigMapRef == nil {
		return nil, nil
	}

	out := &kubeapiserver.AuditConfig{}

	configMap := &corev1.ConfigMap{}
	if err := b.K8sGardenClient.Client().Get(ctx, kutil.Key(b.Shoot.GetInfo().Namespace, config.AuditPolicy.ConfigMapRef.Name), configMap); err != nil {
		// Ignore missing audit configuration on shoot deletion to prevent failing redeployments of the
		// kube-apiserver in case the end-user deleted the configmap before/simultaneously to the shoot
		// deletion.
		if !apierrors.IsNotFound(err) || b.Shoot.GetInfo().DeletionTimestamp == nil {
			return nil, fmt.Errorf("retrieving audit policy from the ConfigMap '%v' failed with reason '%w'", config.AuditPolicy.ConfigMapRef.Name, err)
		}
	} else {
		policy, ok := configMap.Data["policy"]
		if !ok {
			return nil, fmt.Errorf("missing '.data.policy' in audit policy configmap %v/%v", b.Shoot.GetInfo().Namespace, config.AuditPolicy.ConfigMapRef.Name)
		}
		out.Policy = &policy
	}

	return out, nil
}

func (b *Botanist) computeKubeAPIServerAutoscalingConfig() kubeapiserver.AutoscalingConfig {
	var (
		hvpaEnabled               = gardenletfeatures.FeatureGate.Enabled(features.HVPA)
		useMemoryMetricForHvpaHPA = false
		scaleDownDisabledForHvpa  = false
		defaultReplicas           *int32
		minReplicas               int32 = 1
		maxReplicas               int32 = 4
	)

	if b.Shoot.Purpose == gardencorev1beta1.ShootPurposeProduction {
		minReplicas = 2
	}

	if metav1.HasAnnotation(b.Shoot.GetInfo().ObjectMeta, v1beta1constants.ShootAlphaControlPlaneScaleDownDisabled) {
		minReplicas = 4
		scaleDownDisabledForHvpa = true
	}

	if b.ManagedSeed != nil {
		hvpaEnabled = gardenletfeatures.FeatureGate.Enabled(features.HVPAForShootedSeed)
		useMemoryMetricForHvpaHPA = true

		if b.ManagedSeedAPIServer != nil {
			minReplicas = *b.ManagedSeedAPIServer.Autoscaler.MinReplicas
			maxReplicas = b.ManagedSeedAPIServer.Autoscaler.MaxReplicas

			if !hvpaEnabled {
				defaultReplicas = b.ManagedSeedAPIServer.Replicas
			}
		}
	}

	return kubeapiserver.AutoscalingConfig{
		HVPAEnabled:               hvpaEnabled,
		Replicas:                  defaultReplicas,
		MinReplicas:               minReplicas,
		MaxReplicas:               maxReplicas,
		UseMemoryMetricForHvpaHPA: useMemoryMetricForHvpaHPA,
		ScaleDownDisabledForHvpa:  scaleDownDisabledForHvpa,
	}
}

func (b *Botanist) computeKubeAPIServerImages() (kubeapiserver.Images, error) {
	imageAlpineIPTables, err := b.ImageVector.FindImage(charts.ImageNameAlpineIptables, imagevector.RuntimeVersion(b.SeedVersion()), imagevector.TargetVersion(b.ShootVersion()))
	if err != nil {
		return kubeapiserver.Images{}, err
	}

	imageApiserverProxyPodWebhook, err := b.ImageVector.FindImage(charts.ImageNameApiserverProxyPodWebhook, imagevector.RuntimeVersion(b.SeedVersion()), imagevector.TargetVersion(b.ShootVersion()))
	if err != nil {
		return kubeapiserver.Images{}, err
	}

	imageVPNSeed, err := b.ImageVector.FindImage(charts.ImageNameVpnSeed, imagevector.RuntimeVersion(b.SeedVersion()), imagevector.TargetVersion(b.ShootVersion()))
	if err != nil {
		return kubeapiserver.Images{}, err
	}

	return kubeapiserver.Images{
		AlpineIPTables:           imageAlpineIPTables.String(),
		APIServerProxyPodWebhook: imageApiserverProxyPodWebhook.String(),
		VPNSeed:                  imageVPNSeed.String(),
	}, nil
}

func (b *Botanist) computeKubeAPIServerServiceAccountConfig(ctx context.Context, config *gardencorev1beta1.ServiceAccountConfig) (*kubeapiserver.ServiceAccountConfig, error) {
	if config == nil {
		return nil, nil
	}

	out := &kubeapiserver.ServiceAccountConfig{}

	if signingKeySecret := config.SigningKeySecret; signingKeySecret != nil {
		secret := &corev1.Secret{}
		if err := b.K8sGardenClient.Client().Get(ctx, kutil.Key(b.Shoot.GetInfo().Namespace, signingKeySecret.Name), secret); err != nil {
			return nil, err
		}

		data, ok := secret.Data[kubeapiserver.SecretServiceAccountSigningKeyDataKeySigningKey]
		if !ok {
			return nil, fmt.Errorf("no signing key in secret %s/%s at .data.%s", secret.Namespace, secret.Name, kubeapiserver.SecretServiceAccountSigningKeyDataKeySigningKey)
		}
		out.SigningKey = data
	}

	return out, nil
}

func (b *Botanist) computeKubeAPIServerSNIConfig() kubeapiserver.SNIConfig {
	var config kubeapiserver.SNIConfig

	if b.APIServerSNIEnabled() {
		config.Enabled = true

		if b.APIServerSNIPodMutatorEnabled() {
			config.PodMutatorEnabled = true
			config.APIServerFQDN = b.Shoot.ComputeOutOfClusterAPIServerAddress(b.APIServerAddress, true)
		}
	}

	return config
}

func (b *Botanist) computeKubeAPIServerReplicas(deployment *appsv1.Deployment) *int32 {
	autoscalingConfig := b.Shoot.Components.ControlPlane.KubeAPIServer.GetValues().Autoscaling

	switch {
	case autoscalingConfig.Replicas != nil:
		// If the replicas were already set then don't change them.
		return autoscalingConfig.Replicas
	case deployment == nil:
		// If the Deployment does not yet exist then set the desired replicas to the minimum replicas.
		return &autoscalingConfig.MinReplicas
	case deployment.Spec.Replicas != nil && *deployment.Spec.Replicas > 0:
		// If the Deployment exists then don't interfere with the replicas because they are controlled via HVPA or HPA.
		return deployment.Spec.Replicas
	case b.Shoot.HibernationEnabled && (deployment.Spec.Replicas == nil || *deployment.Spec.Replicas == 0):
		// If the Shoot is hibernated and the deployment has already been scaled down then we want to keep it scaled
		// down. If it has not yet been scaled down then above case applies (replicas are kept) - the scale-down will
		// happen at a later point in the flow.
		return pointer.Int32(0)
	default:
		// If none of the above cases applies then a default value has to be returned.
		return pointer.Int32(1)
	}
}

// GetLegacyDeployKubeAPIServerFunc is exposed for testing.
// TODO: Remove this once the kube-apiserver component refactoring has been completed.
var GetLegacyDeployKubeAPIServerFunc = func(botanist *Botanist) func(context.Context) error {
	return botanist.deployKubeAPIServer
}

// DeployKubeAPIServer deploys the Kubernetes API server.
func (b *Botanist) DeployKubeAPIServer(ctx context.Context) error {
	if err := GetLegacyDeployKubeAPIServerFunc(b)(ctx); err != nil {
		return err
	}

	deployment := &appsv1.Deployment{}
	if err := b.K8sSeedClient.Client().Get(ctx, kutil.Key(b.Shoot.SeedNamespace, v1beta1constants.DeploymentNameKubeAPIServer), deployment); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		deployment = nil
	}

	b.Shoot.Components.ControlPlane.KubeAPIServer.SetAutoscalingReplicas(b.computeKubeAPIServerReplicas(deployment))

	var (
		values  = b.Shoot.Components.ControlPlane.KubeAPIServer.GetValues()
		secrets = kubeapiserver.Secrets{
			CA:                     component.Secret{Name: v1beta1constants.SecretNameCACluster, Checksum: b.LoadCheckSum(v1beta1constants.SecretNameCACluster)},
			CAEtcd:                 component.Secret{Name: etcd.SecretNameCA, Checksum: b.LoadCheckSum(etcd.SecretNameCA)},
			CAFrontProxy:           component.Secret{Name: v1beta1constants.SecretNameCAFrontProxy, Checksum: b.LoadCheckSum(v1beta1constants.SecretNameCAFrontProxy)},
			Etcd:                   component.Secret{Name: etcd.SecretNameClient, Checksum: b.LoadCheckSum(etcd.SecretNameClient)},
			EtcdEncryptionConfig:   component.Secret{Name: kubeapiserver.SecretNameEtcdEncryption, Checksum: b.LoadCheckSum(kubeapiserver.SecretNameEtcdEncryption)},
			KubeAggregator:         component.Secret{Name: kubeapiserver.SecretNameKubeAggregator, Checksum: b.LoadCheckSum(kubeapiserver.SecretNameKubeAggregator)},
			KubeAPIServerToKubelet: component.Secret{Name: kubeapiserver.SecretNameKubeAPIServerToKubelet, Checksum: b.LoadCheckSum(kubeapiserver.SecretNameKubeAPIServerToKubelet)},
			Server:                 component.Secret{Name: kubeapiserver.SecretNameServer, Checksum: b.LoadCheckSum(kubeapiserver.SecretNameServer)},
			ServiceAccountKey:      component.Secret{Name: v1beta1constants.SecretNameServiceAccountKey, Checksum: b.LoadCheckSum(v1beta1constants.SecretNameServiceAccountKey)},
			StaticToken:            component.Secret{Name: kubeapiserver.SecretNameStaticToken, Checksum: b.LoadCheckSum(kubeapiserver.SecretNameStaticToken)},
		}
	)

	if values.BasicAuthenticationEnabled {
		secrets.BasicAuthentication = &component.Secret{Name: kubeapiserver.SecretNameBasicAuth, Checksum: b.LoadCheckSum(kubeapiserver.SecretNameBasicAuth)}
	}

	if values.VPN.ReversedVPNEnabled {
		secrets.VPNSeedServerTLSAuth = &component.Secret{Name: vpnseedserver.VpnSeedServerTLSAuth, Checksum: b.LoadCheckSum(vpnseedserver.VpnSeedServerTLSAuth)}
	} else {
		secrets.VPNSeed = &component.Secret{Name: kubeapiserver.SecretNameVPNSeed, Checksum: b.LoadCheckSum(kubeapiserver.SecretNameVPNSeed)}
		secrets.VPNSeedTLSAuth = &component.Secret{Name: kubeapiserver.SecretNameVPNSeedTLSAuth, Checksum: b.LoadCheckSum(kubeapiserver.SecretNameVPNSeedTLSAuth)}
	}

	b.Shoot.Components.ControlPlane.KubeAPIServer.SetSecrets(secrets)

	return b.Shoot.Components.ControlPlane.KubeAPIServer.Deploy(ctx)
}

// DeleteKubeAPIServer deletes the kube-apiserver deployment in the Seed cluster which holds the Shoot's control plane.
func (b *Botanist) DeleteKubeAPIServer(ctx context.Context) error {
	// invalidate shoot client here before deleting API server
	if err := b.ClientMap.InvalidateClient(keys.ForShoot(b.Shoot.GetInfo())); err != nil {
		return err
	}
	b.K8sShootClient = nil

	return b.Shoot.Components.ControlPlane.KubeAPIServer.Destroy(ctx)
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
	return b.Shoot.Components.ControlPlane.KubeAPIServer.Wait(ctx)
}

// ScaleKubeAPIServerToOne scales kube-apiserver replicas to one.
func (b *Botanist) ScaleKubeAPIServerToOne(ctx context.Context) error {
	return kubernetes.ScaleDeployment(ctx, b.K8sSeedClient.Client(), kutil.Key(b.Shoot.SeedNamespace, v1beta1constants.DeploymentNameKubeAPIServer), 1)
}
