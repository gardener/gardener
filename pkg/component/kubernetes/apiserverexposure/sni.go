// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package apiserverexposure

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"text/template"
	"time"

	"github.com/Masterminds/sprig/v3"
	istioapinetworkingv1beta1 "istio.io/api/networking/v1beta1"
	istionetworkingv1alpha3 "istio.io/client-go/pkg/apis/networking/v1alpha3"
	istionetworkingv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/kubernetes/apiserver"
	kubeapiserverconstants "github.com/gardener/gardener/pkg/component/kubernetes/apiserver/constants"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/utils/istio"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	netutils "github.com/gardener/gardener/pkg/utils/net"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

const (
	// MutualTLSServiceNameSuffix is used to create a second service instance for
	// use with mutual tls authentication from istio using the ca-front-proxy secrets.
	MutualTLSServiceNameSuffix = "-mtls"
	// AuthenticationDynamicMetadataKey is the key used to configure the istio envoy filter.
	AuthenticationDynamicMetadataKey = "authenticated-kube-apiserver-host"

	// authenticationDynamicMetadataKeyAPIServerProxy is the key used to configure the istio envoy filter for the APIServer proxy.
	authenticationDynamicMetadataKeyAPIServerProxy = "authenticated-shoot"

	// istioMTLSSecretSuffix is the suffix for the secret used for mutual tls authentication to kube-apiserver.
	istioMTLSSecretSuffix = "-kube-apiserver-istio-mtls" // #nosec G101 -- No credential.
	// istioTLSSecretSuffix is the suffix for secret used for TLS termination for connections to kube-apiserver.
	istioTLSSecretSuffix = "-kube-apiserver-tls" // #nosec G101 -- No credential.

	managedResourceName                = "kube-apiserver-sni"
	managedResourceNameIstioTLSSecrets = "istio-tls-secrets" // #nosec G101 -- No credential.

	secretNameIstioClientCertificate = "istio-client-certificate" // #nosec G101 -- No credential.
)

var (
	//go:embed templates/envoyfilter-apiserver-proxy.yaml
	envoyFilterAPIServerProxyTemplateContent string
	envoyFilterAPIServerProxyTemplate        *template.Template
	//go:embed templates/envoyfilter-istio-tls-termination.yaml
	envoyFilterIstioTLSTerminationTemplateContent string
	envoyFilterIstioTLSTerminationTemplate        *template.Template
)

func init() {
	envoyFilterAPIServerProxyTemplate = template.Must(template.
		New("envoy-filter-apiserver-proxy").
		Funcs(sprig.TxtFuncMap()).
		Parse(envoyFilterAPIServerProxyTemplateContent),
	)
	envoyFilterIstioTLSTerminationTemplate = template.Must(template.
		New("envoy-filter-istio-tls-termination").
		Funcs(sprig.TxtFuncMap()).
		Parse(envoyFilterIstioTLSTerminationTemplateContent),
	)
}

// SNIValues configure the kube-apiserver service SNI.
type SNIValues struct {
	Hosts               []string
	APIServerProxy      *APIServerProxy
	IstioIngressGateway IstioIngressGateway
	IstioTLSTermination bool
}

// APIServerProxy contains values for the APIServer proxy protocol configuration.
type APIServerProxy struct {
	APIServerClusterIP string
}

// IstioIngressGateway contains the values for istio ingress gateway configuration.
type IstioIngressGateway struct {
	Namespace string
	Labels    map[string]string
}

// NewSNI creates a new instance of DeployWaiter which deploys Istio resources for
// kube-apiserver SNI access.
func NewSNI(
	client client.Client,
	name string,
	namespace string,
	secretsManager secretsmanager.Interface,
	valuesFunc func() *SNIValues,
) component.DeployWaiter {
	if valuesFunc == nil {
		valuesFunc = func() *SNIValues { return &SNIValues{} }
	}

	return &sni{
		client:         client,
		name:           name,
		namespace:      namespace,
		secretsManager: secretsManager,
		valuesFunc:     valuesFunc,
	}
}

type sni struct {
	client         client.Client
	name           string
	namespace      string
	secretsManager secretsmanager.Interface
	valuesFunc     func() *SNIValues
}

type envoyFilterAPIServerProxyTemplateValues struct {
	*APIServerProxy
	IngressGatewayLabels                      map[string]string
	Name                                      string
	Namespace                                 string
	ControlPlaneNamespace                     string
	Host                                      string
	MTLSHost                                  string
	Port                                      int
	APIServerClusterIPPrefixLen               int
	APIServerRequestHeaderUserName            string
	APIServerRequestHeaderGroup               string
	APIServerAuthenticationDynamicMetadataKey string
	IstioTLSTermination                       bool
	IstioTLSSecret                            string
	TargetClusterProxyProtocol                string
}

type envoyFilterIstioTLSTerminationTemplateValues struct {
	AuthenticationDynamicMetadataKey string
	Hosts                            []string
	IngressGatewayLabels             map[string]string
	Name                             string
	Namespace                        string
	MutualTLSHost                    string
	Port                             int
	RouteConfigurationName           string
}

func (s *sni) Deploy(ctx context.Context) error {
	var (
		values = s.valuesFunc()

		destinationRule     = s.emptyDestinationRule()
		mTLSDestinationRule = s.emptyMTLSDestinationRule()
		gateway             = s.emptyGateway()
		virtualService      = s.emptyVirtualService()

		hostName                       = fmt.Sprintf("%s.%s.svc.%s", s.name, s.namespace, gardencorev1beta1.DefaultDomain)
		mTLSHostName                   = fmt.Sprintf("%s%s.%s.svc.%s", s.name, MutualTLSServiceNameSuffix, s.namespace, gardencorev1beta1.DefaultDomain)
		routeConfigurationName         = fmt.Sprintf("https.%d.tls.%s.%s", kubeapiserverconstants.Port, s.name, s.namespace)
		envoyFilterAPIServerProxy      bytes.Buffer
		envoyFilterIstioTLSTermination bytes.Buffer
	)

	registry := managedresources.NewRegistry(kubernetes.SeedScheme, kubernetes.SeedCodec, kubernetes.SeedSerializer)

	if values.APIServerProxy != nil {
		envoyFilter := s.emptyEnvoyFilterAPIServerProxy()
		apiServerClusterIPPrefixLen, err := netutils.GetBitLen(values.APIServerProxy.APIServerClusterIP)
		if err != nil {
			return err
		}
		targetClusterProxyProtocol := fmt.Sprintf("outbound|%d||%s", kubeapiserverconstants.Port, hostName)
		if values.IstioTLSTermination {
			targetClusterProxyProtocol = GetAPIServerProxyTargetClusterName(s.namespace)
		}

		if err := envoyFilterAPIServerProxyTemplate.Execute(&envoyFilterAPIServerProxy, envoyFilterAPIServerProxyTemplateValues{
			APIServerProxy:                 values.APIServerProxy,
			IngressGatewayLabels:           values.IstioIngressGateway.Labels,
			Name:                           envoyFilter.Name,
			Namespace:                      envoyFilter.Namespace,
			ControlPlaneNamespace:          s.namespace,
			Host:                           hostName,
			MTLSHost:                       mTLSHostName,
			Port:                           kubeapiserverconstants.Port,
			APIServerClusterIPPrefixLen:    apiServerClusterIPPrefixLen,
			APIServerRequestHeaderUserName: kubeapiserverconstants.RequestHeaderUserName,
			APIServerRequestHeaderGroup:    kubeapiserverconstants.RequestHeaderGroup,
			APIServerAuthenticationDynamicMetadataKey: authenticationDynamicMetadataKeyAPIServerProxy,
			IstioTLSTermination:                       values.IstioTLSTermination,
			IstioTLSSecret:                            s.emptyIstioTLSSecret().Name,
			TargetClusterProxyProtocol:                targetClusterProxyProtocol,
		}); err != nil {
			return err
		}

		filename := fmt.Sprintf("envoyfilter__%s__%s.yaml", envoyFilter.Namespace, envoyFilter.Name)
		registry.AddSerialized(filename, envoyFilterAPIServerProxy.Bytes())
	}

	if err := s.reconcileIstioTLSSecrets(ctx); err != nil {
		return err
	}

	if values.IstioTLSTermination {
		envoyFilter := s.emptyEnvoyFilterIstioTLSTermination()

		if err := envoyFilterIstioTLSTerminationTemplate.Execute(&envoyFilterIstioTLSTermination, envoyFilterIstioTLSTerminationTemplateValues{
			AuthenticationDynamicMetadataKey: AuthenticationDynamicMetadataKey,
			Hosts:                            s.valuesFunc().Hosts,
			IngressGatewayLabels:             values.IstioIngressGateway.Labels,
			Name:                             envoyFilter.Name,
			Namespace:                        envoyFilter.Namespace,
			Port:                             kubeapiserverconstants.Port,
			MutualTLSHost:                    mTLSHostName,
			RouteConfigurationName:           routeConfigurationName,
		}); err != nil {
			return err
		}

		filename := fmt.Sprintf("envoyfilter__%s__%s.yaml", envoyFilter.Namespace, envoyFilter.Name)
		registry.AddSerialized(filename, envoyFilterIstioTLSTermination.Bytes())
	}

	if values.APIServerProxy != nil || values.IstioTLSTermination {
		serializedObjects, err := registry.SerializedObjects()
		if err != nil {
			return err
		}

		if err := managedresources.CreateForSeed(ctx, s.client, s.namespace, managedResourceName, false, serializedObjects); err != nil {
			return err
		}
	} else {
		if err := managedresources.DeleteForSeed(ctx, s.client, s.namespace, managedResourceName); err != nil {
			return err
		}
	}

	destinationMutateFn := istio.DestinationRuleWithLocalityPreference(destinationRule, getLabels(), hostName)
	if values.IstioTLSTermination {
		destinationMutateFn = istio.DestinationRuleWithTLSTermination(destinationRule, getLabels(), hostName, s.namespace+istioMTLSSecretSuffix, istioapinetworkingv1beta1.ClientTLSSettings_SIMPLE)
	}

	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, s.client, destinationRule, destinationMutateFn); err != nil {
		return err
	}

	if values.IstioTLSTermination {
		destinationMTLSMutateFn := istio.DestinationRuleWithTLSTermination(mTLSDestinationRule, getLabels(), mTLSHostName, s.namespace+istioMTLSSecretSuffix, istioapinetworkingv1beta1.ClientTLSSettings_MUTUAL)
		if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, s.client, mTLSDestinationRule, destinationMTLSMutateFn); err != nil {
			return err
		}
	} else {
		if err := kubernetesutils.DeleteObject(ctx, s.client, mTLSDestinationRule); err != nil {
			return err
		}
	}

	gatewayMutateFn := istio.GatewayWithTLSPassthrough(gateway, getLabels(), s.valuesFunc().IstioIngressGateway.Labels, s.valuesFunc().Hosts, kubeapiserverconstants.Port)
	if values.IstioTLSTermination {
		gatewayMutateFn = istio.GatewayWithMutualTLS(gateway, getLabels(), s.valuesFunc().IstioIngressGateway.Labels, s.valuesFunc().Hosts, kubeapiserverconstants.Port, s.namespace+istioTLSSecretSuffix)
	}

	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, s.client, gateway, gatewayMutateFn); err != nil {
		return err
	}

	virtualServiceMutateFn := istio.VirtualServiceWithSNIMatch(virtualService, getLabels(), s.valuesFunc().Hosts, gateway.Name, kubeapiserverconstants.Port, hostName)
	if values.IstioTLSTermination {
		virtualServiceMutateFn = istio.VirtualServiceForTLSTermination(virtualService, getLabels(), s.valuesFunc().Hosts, gateway.Name, kubeapiserverconstants.Port, hostName)
	}

	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, s.client, virtualService, virtualServiceMutateFn); err != nil {
		return err
	}

	return nil
}

func (s *sni) Destroy(ctx context.Context) error {
	if err := managedresources.DeleteForSeed(ctx, s.client, s.namespace, managedResourceName); err != nil {
		return err
	}

	if err := managedresources.DeleteForSeed(ctx, s.client, s.namespace, managedResourceNameIstioTLSSecrets); err != nil {
		return err
	}

	return kubernetesutils.DeleteObjects(
		ctx,
		s.client,
		s.emptyDestinationRule(),
		s.emptyMTLSDestinationRule(),
		s.emptyGateway(),
		s.emptyVirtualService(),
	)
}

func (s *sni) Wait(_ context.Context) error        { return nil }
func (s *sni) WaitCleanup(_ context.Context) error { return nil }

func (s *sni) emptyDestinationRule() *istionetworkingv1beta1.DestinationRule {
	return &istionetworkingv1beta1.DestinationRule{ObjectMeta: metav1.ObjectMeta{Name: s.name, Namespace: s.namespace}}
}

func (s *sni) emptyMTLSDestinationRule() *istionetworkingv1beta1.DestinationRule {
	return &istionetworkingv1beta1.DestinationRule{ObjectMeta: metav1.ObjectMeta{Name: s.name + "-mtls", Namespace: s.namespace}}
}

func (s *sni) emptyEnvoyFilterAPIServerProxy() *istionetworkingv1alpha3.EnvoyFilter {
	return &istionetworkingv1alpha3.EnvoyFilter{ObjectMeta: metav1.ObjectMeta{Name: s.namespace + "-apiserver-proxy", Namespace: s.valuesFunc().IstioIngressGateway.Namespace}}
}

func (s *sni) emptyEnvoyFilterIstioTLSTermination() *istionetworkingv1alpha3.EnvoyFilter {
	return &istionetworkingv1alpha3.EnvoyFilter{ObjectMeta: metav1.ObjectMeta{Name: s.namespace + "-istio-tls-termination", Namespace: s.valuesFunc().IstioIngressGateway.Namespace}}
}

func (s *sni) emptyGateway() *istionetworkingv1beta1.Gateway {
	return &istionetworkingv1beta1.Gateway{ObjectMeta: metav1.ObjectMeta{Name: s.name, Namespace: s.namespace}}
}

func (s *sni) emptyVirtualService() *istionetworkingv1beta1.VirtualService {
	return &istionetworkingv1beta1.VirtualService{ObjectMeta: metav1.ObjectMeta{Name: s.name, Namespace: s.namespace}}
}

func (s *sni) emptyIstioMTLSSecret() *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      s.namespace + istioMTLSSecretSuffix,
			Namespace: s.valuesFunc().IstioIngressGateway.Namespace,
		},
	}
}

func (s *sni) emptyIstioTLSSecret() *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      s.namespace + istioTLSSecretSuffix,
			Namespace: s.valuesFunc().IstioIngressGateway.Namespace,
		},
	}
}

func (s *sni) reconcileIstioTLSSecrets(ctx context.Context) error {
	if !s.valuesFunc().IstioTLSTermination {
		return managedresources.DeleteForSeed(ctx, s.client, s.namespace, managedResourceNameIstioTLSSecrets)
	}

	secretCA, found := s.secretsManager.Get(v1beta1constants.SecretNameCACluster)
	if !found {
		return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameCACluster)
	}
	secretCAClient, found := s.secretsManager.Get(v1beta1constants.SecretNameCAClient)
	if !found {
		return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameCAClient)
	}
	secretServer, found := s.secretsManager.Get(apiserver.SecretNameServerCert, secretsmanager.Current)
	if !found {
		return fmt.Errorf("secret %q not found", apiserver.SecretNameServerCert)
	}

	secretIstioClientCertificate, err := s.secretsManager.Generate(ctx, &secretsutils.CertificateSecretConfig{
		Name:                        secretNameIstioClientCertificate,
		CommonName:                  "system:istio-gateway",
		CertType:                    secretsutils.ClientCert,
		SkipPublishingCACertificate: true,
		Validity:                    ptr.To(time.Hour * 24 * 30),
	}, secretsmanager.SignedByCA(v1beta1constants.SecretNameCAFrontProxy), secretsmanager.Rotate(secretsmanager.InPlace))
	if err != nil {
		return fmt.Errorf("failed to generate kube-apiserver client certificate for istio: %w", err)
	}

	registry := managedresources.NewRegistry(kubernetes.SeedScheme, kubernetes.SeedCodec, kubernetes.SeedSerializer)

	istioTLSSecret := s.emptyIstioTLSSecret()
	istioTLSSecret.Data = map[string][]byte{
		"cacert": secretCAClient.Data[secretsutils.DataKeyCertificateBundle],
		"key":    secretServer.Data[secretsutils.DataKeyPrivateKey],
		"cert":   secretServer.Data[secretsutils.DataKeyCertificate],
	}

	istioMTLSSecret := s.emptyIstioMTLSSecret()
	istioMTLSSecret.Data = map[string][]byte{
		"cacert": secretCA.Data[secretsutils.DataKeyCertificateBundle],
		"key":    secretIstioClientCertificate.Data[secretsutils.DataKeyPrivateKey],
		"cert":   secretIstioClientCertificate.Data[secretsutils.DataKeyCertificate],
	}

	serializedObjects, err := registry.AddAllAndSerialize(istioTLSSecret, istioMTLSSecret)
	if err != nil {
		return fmt.Errorf("failed to serialize Istio TLS secrets: %w", err)
	}

	if err := managedresources.CreateForSeed(ctx, s.client, s.namespace, managedResourceNameIstioTLSSecrets, false, serializedObjects); err != nil {
		return fmt.Errorf("failed to create managed resource %s: %w", managedResourceNameIstioTLSSecrets, err)
	}

	return nil
}

// GetAPIServerProxyTargetClusterName returns the name of the target cluster for apiserver-proxy for the given control-plane namespace.
// This cluster is only available if Istio TLS termination is enabled for the shoot.
func GetAPIServerProxyTargetClusterName(controlPlaneNamespace string) string {
	return fmt.Sprintf("%s--kube-apiserver-socket", controlPlaneNamespace)
}
