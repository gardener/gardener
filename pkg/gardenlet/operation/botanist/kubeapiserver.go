// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	apiserverv1beta1 "k8s.io/apiserver/pkg/apis/apiserver/v1beta1"
	clientcmdlatest "k8s.io/client-go/tools/clientcmd/api/latest"
	clientcmdv1 "k8s.io/client-go/tools/clientcmd/api/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/component/apiserver"
	resourcemanagerconstants "github.com/gardener/gardener/pkg/component/gardener/resourcemanager/constants"
	kubeapiserver "github.com/gardener/gardener/pkg/component/kubernetes/apiserver"
	"github.com/gardener/gardener/pkg/component/shared"
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
		vpnConfig.IPFamilies = b.Seed.GetInfo().Spec.Networks.IPFamilies
		vpnConfig.DisableNewVPN = !b.Shoot.UsesNewVPN
	}

	return shared.NewKubeAPIServer(
		ctx,
		b.SeedClientSet,
		b.GardenClient,
		b.Shoot.ControlPlaneNamespace,
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
		nil,
		nil,
		nil,
		nil,
	)
}

func (b *Botanist) computeKubeAPIServerAutoscalingConfig() apiserver.AutoscalingConfig {
	var (
		scaleDownDisabled = false
		// kube-apiserver is a control plane component of type "server".
		// The HA webhook sets at least 2 replicas to components of type "server" (w/o HA or with w/ HA).
		// Ref https://github.com/gardener/gardener/blob/master/docs/development/high-availability-of-components.md#control-plane-components.
		// That's why minReplicas is set to 2.
		minReplicas        int32 = 2
		maxReplicas        int32 = 6
		apiServerResources       = corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("250m"),
				corev1.ResourceMemory: resource.MustParse("500Mi"),
			},
		}
	)

	if v1beta1helper.IsHAControlPlaneConfigured(b.Shoot.GetInfo()) {
		minReplicas = 3
	}
	if metav1.HasAnnotation(b.Shoot.GetInfo().ObjectMeta, v1beta1constants.ShootAlphaControlPlaneScaleDownDisabled) {
		minReplicas = 4
		scaleDownDisabled = true
	}

	if b.ManagedSeed != nil {
		if b.ManagedSeedAPIServer != nil {
			minReplicas = *b.ManagedSeedAPIServer.Autoscaler.MinReplicas
			maxReplicas = b.ManagedSeedAPIServer.Autoscaler.MaxReplicas
		}
	}

	return apiserver.AutoscalingConfig{
		APIServerResources: apiServerResources,
		MinReplicas:        minReplicas,
		MaxReplicas:        maxReplicas,
		ScaleDownDisabled:  scaleDownDisabled,
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
func (b *Botanist) DeployKubeAPIServer(ctx context.Context, enableNodeAgentAuthorizer bool) error {
	externalServer := b.Shoot.ComputeOutOfClusterAPIServerAddress(false)

	externalHostname := b.Shoot.ComputeOutOfClusterAPIServerAddress(true)
	serviceAccountConfig, err := b.computeKubeAPIServerServiceAccountConfig(externalHostname)
	if err != nil {
		return err
	}

	if enableNodeAgentAuthorizer {
		caSecret, found := b.SecretsManager.Get(v1beta1constants.SecretNameCACluster)
		if !found {
			return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameCACluster)
		}

		kubeconfig, err := runtime.Encode(clientcmdlatest.Codec, kubernetesutils.NewKubeconfig(
			"authorization-webhook",
			clientcmdv1.Cluster{
				Server:                   fmt.Sprintf("https://%s/webhooks/auth/nodeagent", resourcemanagerconstants.ServiceName),
				CertificateAuthorityData: caSecret.Data[secretsutils.DataKeyCertificateBundle],
			},
			clientcmdv1.AuthInfo{},
		))
		if err != nil {
			return fmt.Errorf("failed generating authorization webhook kubeconfig: %w", err)
		}

		if err := b.Shoot.Components.ControlPlane.KubeAPIServer.AppendAuthorizationWebhook(
			kubeapiserver.AuthorizationWebhook{
				Name:       "node-agent-authorizer",
				Kubeconfig: kubeconfig,
				WebhookConfiguration: apiserverv1beta1.WebhookConfiguration{
					// Set TTL to a very low value since it cannot be set to 0 because of defaulting.
					// See https://github.com/kubernetes/apiserver/blob/3658357fea9fa8b36173d072f2d548f135049e05/pkg/apis/apiserver/v1beta1/defaults.go#L29-L36
					AuthorizedTTL:                            metav1.Duration{Duration: 1 * time.Nanosecond},
					UnauthorizedTTL:                          metav1.Duration{Duration: 1 * time.Nanosecond},
					Timeout:                                  metav1.Duration{Duration: 10 * time.Second},
					FailurePolicy:                            apiserverv1beta1.FailurePolicyDeny,
					SubjectAccessReviewVersion:               "v1",
					MatchConditionSubjectAccessReviewVersion: "v1",
					MatchConditions: []apiserverv1beta1.WebhookMatchCondition{{
						// Only intercept request node-agents
						Expression: fmt.Sprintf("'%s' in request.groups", v1beta1constants.NodeAgentsGroup),
					}},
				},
			}); err != nil {
			return fmt.Errorf("failed appending node-agent-authorizer webhook config to kube-apiserver: %w", err)
		}
	}

	if err := shared.DeployKubeAPIServer(
		ctx,
		b.SeedClientSet.Client(),
		b.Shoot.ControlPlaneNamespace,
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

	// TODO(shafeeqes): Remove this code in gardener v1.120
	{
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

	shouldManageIssuer := v1beta1helper.HasManagedIssuer(b.Shoot.GetInfo())
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

	b.Shoot.Components.ControlPlane.KubeAPIServer.SetSNIConfig(b.computeKubeAPIServerSNIConfig())

	return b.Shoot.Components.ControlPlane.KubeAPIServer.Destroy(ctx)
}

// WakeUpKubeAPIServer creates a service and ensures API Server is scaled up
func (b *Botanist) WakeUpKubeAPIServer(ctx context.Context, enableNodeAgentAuthorizer bool) error {
	if err := b.Shoot.Components.ControlPlane.KubeAPIServerService.Deploy(ctx); err != nil {
		return err
	}
	if err := b.Shoot.Components.ControlPlane.KubeAPIServerService.Wait(ctx); err != nil {
		return err
	}
	if err := b.DeployKubeAPIServer(ctx, enableNodeAgentAuthorizer); err != nil {
		return err
	}
	if b.ShootUsesDNS() {
		if err := b.DeployKubeAPIServerSNI(ctx); err != nil {
			return err
		}
	}
	if err := kubernetesutils.ScaleDeployment(ctx, b.SeedClientSet.Client(), client.ObjectKey{Namespace: b.Shoot.ControlPlaneNamespace, Name: v1beta1constants.DeploymentNameKubeAPIServer}, 1); err != nil {
		return err
	}
	return b.Shoot.Components.ControlPlane.KubeAPIServer.Wait(ctx)
}

// ScaleKubeAPIServerToOne scales kube-apiserver replicas to one.
func (b *Botanist) ScaleKubeAPIServerToOne(ctx context.Context) error {
	b.Shoot.Components.ControlPlane.KubeAPIServer.SetAutoscalingReplicas(ptr.To[int32](1))
	return kubernetesutils.ScaleDeployment(ctx, b.SeedClientSet.Client(), client.ObjectKey{Namespace: b.Shoot.ControlPlaneNamespace, Name: v1beta1constants.DeploymentNameKubeAPIServer}, 1)
}
