// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"
	"errors"
	"fmt"
	"net"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/component/apiserver"
	kubeapiserver "github.com/gardener/gardener/pkg/component/kubernetes/apiserver"
	"github.com/gardener/gardener/pkg/component/shared"
	"github.com/gardener/gardener/pkg/features"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
)

// DefaultKubeAPIServer returns a deployer for the kube-apiserver.
func (b *Botanist) DefaultKubeAPIServer(ctx context.Context) (kubeapiserver.Interface, error) {
	var (
		vpnConfig = kubeapiserver.VPNConfig{
			Enabled: false,
		}
	)

	if !b.Shoot.IsWorkerless {
		vpnConfig.Enabled = true
		vpnConfig.HighAvailabilityEnabled = b.Shoot.VPNHighAvailabilityEnabled
		vpnConfig.HighAvailabilityNumberOfSeedServers = b.Shoot.VPNHighAvailabilityNumberOfSeedServers
		vpnConfig.HighAvailabilityNumberOfShootClients = b.Shoot.VPNHighAvailabilityNumberOfShootClients
		// Pod/service/node network CIDRs are set on deployment to handle dynamic network CIDRs
		vpnConfig.IPFamilies = b.Shoot.GetInfo().Spec.Networking.IPFamilies
		vpnConfig.DisableNewVPN = !b.Shoot.UsesNewVPN
	}

	return shared.NewKubeAPIServer(
		ctx,
		b.SeedClientSet,
		b.GardenClient,
		b.Shoot.SeedNamespace,
		b.Shoot.GetInfo().ObjectMeta,
		b.Seed.KubernetesVersion,
		b.Shoot.KubernetesVersion,
		b.SecretsManager,
		"",
		b.Shoot.GetInfo().Spec.Kubernetes.KubeAPIServer,
		b.computeKubeAPIServerAutoscalingConfig(),
		vpnConfig,
		v1beta1constants.PriorityClassNameShootControlPlane500,
		b.Shoot.IsWorkerless,
		b.Shoot.GetInfo().Spec.Kubernetes.EnableStaticTokenKubeconfig,
		nil,
		nil,
		nil,
		nil,
	)
}

func (b *Botanist) computeKubeAPIServerAutoscalingConfig() apiserver.AutoscalingConfig {
	var (
		autoscalingMode           = b.autoscalingMode()
		useMemoryMetricForHvpaHPA = false
		scaleDownDisabled         = false
		defaultReplicas           *int32
		// kube-apiserver is a control plane component of type "server".
		// The HA webhook sets at least 2 replicas to components of type "server" (w/o HA or with w/ HA).
		// Ref https://github.com/gardener/gardener/blob/master/docs/development/high-availability.md#control-plane-components.
		// That's why minReplicas is set to 2.
		minReplicas        int32 = 2
		maxReplicas        int32 = 3
		apiServerResources corev1.ResourceRequirements
	)

	if v1beta1helper.IsHAControlPlaneConfigured(b.Shoot.GetInfo()) {
		minReplicas = 3
	}
	if metav1.HasAnnotation(b.Shoot.GetInfo().ObjectMeta, v1beta1constants.ShootAlphaControlPlaneScaleDownDisabled) {
		minReplicas = 4
		maxReplicas = 4
		scaleDownDisabled = true
	}
	if autoscalingMode == apiserver.AutoscalingModeVPAAndHPA {
		maxReplicas = 6
	}

	nodeCount := b.Shoot.GetMinNodeCount()

	switch autoscalingMode {
	case apiserver.AutoscalingModeHVPA:
		apiServerResources = corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("500m"),
				corev1.ResourceMemory: resource.MustParse("1Gi"),
			},
		}
	case apiserver.AutoscalingModeVPAAndHPA:
		apiServerResources = corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("250m"),
				corev1.ResourceMemory: resource.MustParse("500Mi"),
			},
		}
	default:
		apiServerResources = resourcesRequirementsForKubeAPIServerInBaselineMode(nodeCount)
	}

	if b.ManagedSeed != nil {
		useMemoryMetricForHvpaHPA = true

		if b.ManagedSeedAPIServer != nil {
			minReplicas = *b.ManagedSeedAPIServer.Autoscaler.MinReplicas
			maxReplicas = b.ManagedSeedAPIServer.Autoscaler.MaxReplicas

			if autoscalingMode == apiserver.AutoscalingModeBaseline {
				defaultReplicas = b.ManagedSeedAPIServer.Replicas
				apiServerResources = corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("1750m"),
						corev1.ResourceMemory: resource.MustParse("2Gi"),
					},
				}
			}
		}
	}

	return apiserver.AutoscalingConfig{
		Mode:                      autoscalingMode,
		APIServerResources:        apiServerResources,
		Replicas:                  defaultReplicas,
		MinReplicas:               minReplicas,
		MaxReplicas:               maxReplicas,
		UseMemoryMetricForHvpaHPA: useMemoryMetricForHvpaHPA,
		ScaleDownDisabled:         scaleDownDisabled,
	}
}

func (b *Botanist) autoscalingMode() apiserver.AutoscalingMode {
	// The VPAAndHPAForAPIServer feature gate takes precedence over the HVPA feature gate.
	if features.DefaultFeatureGate.Enabled(features.VPAAndHPAForAPIServer) {
		return apiserver.AutoscalingModeVPAAndHPA
	}

	hvpaEnabled := features.DefaultFeatureGate.Enabled(features.HVPA)
	if b.ManagedSeed != nil {
		hvpaEnabled = features.DefaultFeatureGate.Enabled(features.HVPAForShootedSeed)
	}

	if hvpaEnabled {
		return apiserver.AutoscalingModeHVPA
	}
	return apiserver.AutoscalingModeBaseline
}

func resourcesRequirementsForKubeAPIServerInBaselineMode(nodeCount int32) corev1.ResourceRequirements {
	var cpuRequest, memoryRequest string

	switch {
	case nodeCount <= 2:
		cpuRequest, memoryRequest = "800m", "800Mi"
	case nodeCount <= 10:
		cpuRequest, memoryRequest = "1000m", "1100Mi"
	case nodeCount <= 50:
		cpuRequest, memoryRequest = "1200m", "1600Mi"
	case nodeCount <= 100:
		cpuRequest, memoryRequest = "2500m", "5200Mi"
	default:
		cpuRequest, memoryRequest = "3000m", "5200Mi"
	}

	return corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(cpuRequest),
			corev1.ResourceMemory: resource.MustParse(memoryRequest),
		},
	}
}

func (b *Botanist) computeKubeAPIServerServerCertificateConfig() kubeapiserver.ServerCertificateConfig {
	var (
		ipAddresses = []net.IP{}
		dnsNames    = []string{
			gardenerutils.GetAPIServerDomain(b.Shoot.InternalClusterDomain),
			b.Shoot.GetInfo().Status.TechnicalID,
		}
	)

	if b.Shoot.Networks != nil {
		ipAddresses = append(ipAddresses, b.Shoot.Networks.APIServer...)
	}

	if b.Shoot.ExternalClusterDomain != nil {
		dnsNames = append(dnsNames, *(b.Shoot.GetInfo().Spec.DNS.Domain), gardenerutils.GetAPIServerDomain(*b.Shoot.ExternalClusterDomain))
	}

	return kubeapiserver.ServerCertificateConfig{
		ExtraIPAddresses: ipAddresses,
		ExtraDNSNames:    dnsNames,
	}
}

func (b *Botanist) computeKubeAPIServerSNIConfig() kubeapiserver.SNIConfig {
	var config kubeapiserver.SNIConfig

	if b.ShootUsesDNS() {
		config.Enabled = true
		config.AdvertiseAddress = b.APIServerClusterIP
	}

	// Add control plane wildcard certificate to TLS SNI config if it is available.
	if b.ControlPlaneWildcardCert != nil {
		config.TLS = append(config.TLS, kubeapiserver.TLSSNIConfig{SecretName: &b.ControlPlaneWildcardCert.Name, DomainPatterns: []string{b.ComputeKubeAPIServerHost()}})
	}

	return config
}

// DeployKubeAPIServer deploys the Kubernetes API server.
func (b *Botanist) DeployKubeAPIServer(ctx context.Context) error {
	externalServer := b.Shoot.ComputeOutOfClusterAPIServerAddress(false)

	externalHostname := b.Shoot.ComputeOutOfClusterAPIServerAddress(true)
	serviceAccountConfig, err := b.computeKubeAPIServerServiceAccountConfig(externalHostname)
	if err != nil {
		return err
	}

	if err := shared.DeployKubeAPIServer(
		ctx,
		b.SeedClientSet.Client(),
		b.Shoot.SeedNamespace,
		b.Shoot.Components.ControlPlane.KubeAPIServer,
		serviceAccountConfig,
		b.computeKubeAPIServerServerCertificateConfig(),
		b.computeKubeAPIServerSNIConfig(),
		externalHostname,
		externalServer,
		b.Shoot.Networks.Nodes,
		b.Shoot.Networks.Services,
		b.Shoot.Networks.Pods,
		b.Shoot.ResourcesToEncrypt,
		b.Shoot.EncryptedResources,
		v1beta1helper.GetShootETCDEncryptionKeyRotationPhase(b.Shoot.GetInfo().Status.Credentials),
		b.Shoot.HibernationEnabled,
	); err != nil {
		return err
	}

	if enableStaticTokenKubeconfig := b.Shoot.GetInfo().Spec.Kubernetes.EnableStaticTokenKubeconfig; enableStaticTokenKubeconfig == nil || *enableStaticTokenKubeconfig {
		userKubeconfigSecret, found := b.SecretsManager.Get(kubeapiserver.SecretNameUserKubeconfig)
		if !found {
			return fmt.Errorf("secret %q not found", kubeapiserver.SecretNameUserKubeconfig)
		}

		// add CA bundle as ca.crt to kubeconfig secret for backwards-compatibility
		caBundleSecret, found := b.SecretsManager.Get(v1beta1constants.SecretNameCACluster)
		if !found {
			return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameCACluster)
		}

		kubeconfigSecretData := userKubeconfigSecret.DeepCopy().Data
		kubeconfigSecretData[secretsutils.DataKeyCertificateCA] = caBundleSecret.Data[secretsutils.DataKeyCertificateBundle]

		if err := b.syncShootCredentialToGarden(
			ctx,
			gardenerutils.ShootProjectSecretSuffixKubeconfig,
			map[string]string{v1beta1constants.GardenRole: v1beta1constants.GardenRoleKubeconfig},
			map[string]string{"url": "https://" + externalServer},
			kubeconfigSecretData,
		); err != nil {
			return err
		}
	} else {
		secretName := gardenerutils.ComputeShootProjectResourceName(b.Shoot.GetInfo().Name, gardenerutils.ShootProjectSecretSuffixKubeconfig)
		if err := kubernetesutils.DeleteObject(ctx, b.GardenClient, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: secretName, Namespace: b.Shoot.GetInfo().Namespace}}); err != nil {
			return err
		}
	}

	return nil
}

func (b *Botanist) computeKubeAPIServerServiceAccountConfig(externalHostname string) (kubeapiserver.ServiceAccountConfig, error) {
	var config *gardencorev1beta1.ServiceAccountConfig
	if b.Shoot.GetInfo().Spec.Kubernetes.KubeAPIServer != nil && b.Shoot.GetInfo().Spec.Kubernetes.KubeAPIServer.ServiceAccountConfig != nil {
		config = b.Shoot.GetInfo().Spec.Kubernetes.KubeAPIServer.ServiceAccountConfig.DeepCopy()
	}

	shouldManageIssuer := v1beta1helper.HasManagedIssuer(b.Shoot.GetInfo()) && features.DefaultFeatureGate.Enabled(features.ShootManagedIssuer)
	canManageIssuer := b.Shoot.ServiceAccountIssuerHostname != nil
	if shouldManageIssuer && !canManageIssuer {
		return kubeapiserver.ServiceAccountConfig{}, errors.New("shoot requires managed issuer, but gardener does not have shoot service account hostname configured")
	}

	var jwksURI *string
	if shouldManageIssuer && canManageIssuer {
		if config == nil {
			config = &gardencorev1beta1.ServiceAccountConfig{}
		}
		config.Issuer = ptr.To(fmt.Sprintf("https://%s/projects/%s/shoots/%s/issuer", *b.Shoot.ServiceAccountIssuerHostname, b.Garden.Project.Name, b.Shoot.GetInfo().ObjectMeta.UID))
		jwksURI = ptr.To(fmt.Sprintf("https://%s/projects/%s/shoots/%s/issuer/jwks", *b.Shoot.ServiceAccountIssuerHostname, b.Garden.Project.Name, b.Shoot.GetInfo().ObjectMeta.UID))
	}

	serviceAccountConfig := kubeapiserver.ComputeKubeAPIServerServiceAccountConfig(
		config,
		externalHostname,
		v1beta1helper.GetShootServiceAccountKeyRotationPhase(b.Shoot.GetInfo().Status.Credentials),
	)
	serviceAccountConfig.JWKSURI = jwksURI

	return serviceAccountConfig, nil
}

// DeleteKubeAPIServer deletes the kube-apiserver deployment in the Seed cluster which holds the Shoot's control plane.
func (b *Botanist) DeleteKubeAPIServer(ctx context.Context) error {
	// invalidate shoot client here before deleting API server
	if err := b.ShootClientMap.InvalidateClient(keys.ForShoot(b.Shoot.GetInfo())); err != nil {
		return err
	}
	b.ShootClientSet = nil

	return b.Shoot.Components.ControlPlane.KubeAPIServer.Destroy(ctx)
}

// WakeUpKubeAPIServer creates a service and ensures API Server is scaled up
func (b *Botanist) WakeUpKubeAPIServer(ctx context.Context) error {
	if err := b.Shoot.Components.ControlPlane.KubeAPIServerService.Deploy(ctx); err != nil {
		return err
	}
	if err := b.Shoot.Components.ControlPlane.KubeAPIServerService.Wait(ctx); err != nil {
		return err
	}
	if b.ShootUsesDNS() {
		if err := b.DeployKubeAPIServerSNI(ctx); err != nil {
			return err
		}
	}
	if err := b.DeployKubeAPIServer(ctx); err != nil {
		return err
	}
	if err := kubernetes.ScaleDeployment(ctx, b.SeedClientSet.Client(), client.ObjectKey{Namespace: b.Shoot.SeedNamespace, Name: v1beta1constants.DeploymentNameKubeAPIServer}, 1); err != nil {
		return err
	}
	return b.Shoot.Components.ControlPlane.KubeAPIServer.Wait(ctx)
}

// ScaleKubeAPIServerToOne scales kube-apiserver replicas to one.
func (b *Botanist) ScaleKubeAPIServerToOne(ctx context.Context) error {
	return kubernetes.ScaleDeployment(ctx, b.SeedClientSet.Client(), client.ObjectKey{Namespace: b.Shoot.SeedNamespace, Name: v1beta1constants.DeploymentNameKubeAPIServer}, 1)
}
