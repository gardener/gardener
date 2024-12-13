// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package apiserverexposure

import (
	"context"
	"fmt"

	istioapinetworkingv1beta1 "istio.io/api/networking/v1beta1"
	istionetworkingv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/component"
	kubeapiserverconstants "github.com/gardener/gardener/pkg/component/kubernetes/apiserver/constants"
	"github.com/gardener/gardener/pkg/controllerutils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/istio"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// IngressValues configure the kube-apiserver ingress.
type IngressValues struct {
	// Host is the host where the kube-apiserver should be exposed.
	Host string
	// IstioIngressGatewayLabelsFunc is a function returingin the labels identifying the corresponding istio ingress gateway.
	IstioIngressGatewayLabelsFunc func() map[string]string
	// IstioIngressGatewayNamespaceFunc is a function returning the namespace of the corresponding istio ingress gateway.
	IstioIngressGatewayNamespaceFunc func() string
	// ServiceName is the name of the service the ingress is using.
	ServiceName string
	// ServiceNamespace is the namespace of the service the ingress is using.
	ServiceNamespace string
	// TLSSecretName is the name of the TLS secret.
	// If no secret is provided TLS is not terminated by nginx.
	TLSSecretName *string
}

// NewIngress creates a new instance of Deployer for the ingress used to expose the kube-apiserver.
func NewIngress(c client.Client, namespace string, values IngressValues) component.Deployer {
	return &ingress{client: c, namespace: namespace, values: values}
}

type ingress struct {
	client    client.Client
	namespace string
	values    IngressValues
}

func (i *ingress) Deploy(ctx context.Context) error {
	var (
		destinationRule = i.emptyDestinationRule()
		gateway         = i.emptyGateway()
		virtualService  = i.emptyVirtualService()
	)

	tlsMode := istioapinetworkingv1beta1.ClientTLSSettings{Mode: istioapinetworkingv1beta1.ClientTLSSettings_DISABLE}
	if i.values.TLSSecretName != nil {
		tlsMode = istioapinetworkingv1beta1.ClientTLSSettings{Mode: istioapinetworkingv1beta1.ClientTLSSettings_SIMPLE}
	}

	serviceNamespace := i.namespace
	if i.values.ServiceNamespace != "" {
		serviceNamespace = i.values.ServiceNamespace
	}

	destinationHost := kubernetesutils.FQDNForService(i.values.ServiceName, serviceNamespace)

	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, i.client, destinationRule, istio.DestinationRuleWithLocalityPreferenceAndTLS(destinationRule, getLabels(), destinationHost, &tlsMode)); err != nil {
		return err
	}

	if i.values.TLSSecretName != nil {
		// Istio expects the secret in the istio ingress gateway namespace => copy certificate to istio namespace
		wildcardCert, err := gardenerutils.GetWildcardCertificate(ctx, i.client)
		if err != nil {
			return err
		}
		if wildcardCert == nil {
			return fmt.Errorf("wildcard secret '%s' not found in garden namespace", *i.values.TLSSecretName)
		}

		wildcardSecret := i.emptyWildcardSecret()
		if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, i.client, wildcardSecret, func() error {
			wildcardSecret.Data = wildcardCert.Data
			return nil
		}); err != nil {
			return err
		}

		if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, i.client, gateway, istio.GatewayWithTLSTermination(gateway, getLabels(), i.values.IstioIngressGatewayLabelsFunc(), []string{i.values.Host}, kubeapiserverconstants.Port, ptr.Deref(i.values.TLSSecretName, ""))); err != nil {
			return err
		}

		if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, i.client, virtualService, func() error {
			if err := istio.VirtualServiceWithSNIMatch(virtualService, getLabels(), []string{i.values.Host}, gateway.Name, kubeapiserverconstants.Port, destinationHost)(); err != nil {
				return err
			}
			virtualService.Spec.Http = []*istioapinetworkingv1beta1.HTTPRoute{{
				Match: []*istioapinetworkingv1beta1.HTTPMatchRequest{{
					Uri: &istioapinetworkingv1beta1.StringMatch{
						MatchType: &istioapinetworkingv1beta1.StringMatch_Prefix{Prefix: "/"},
					},
				}},
				Route: []*istioapinetworkingv1beta1.HTTPRouteDestination{{
					Destination: &istioapinetworkingv1beta1.Destination{
						Host: destinationHost,
						Port: &istioapinetworkingv1beta1.PortSelector{
							Number: kubeapiserverconstants.Port,
						},
					},
				}},
			}}
			return nil
		}); err != nil {
			return err
		}
	} else {
		if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, i.client, gateway, istio.GatewayWithTLSPassthrough(gateway, getLabels(), i.values.IstioIngressGatewayLabelsFunc(), []string{i.values.Host}, kubeapiserverconstants.Port)); err != nil {
			return err
		}

		if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, i.client, virtualService, istio.VirtualServiceWithSNIMatch(virtualService, getLabels(), []string{i.values.Host}, gateway.Name, kubeapiserverconstants.Port, destinationHost)); err != nil {
			return err
		}
	}

	return nil
}

func (i *ingress) Destroy(ctx context.Context) error {
	objects := []client.Object{
		i.emptyDestinationRule(),
		i.emptyGateway(),
		i.emptyVirtualService(),
	}
	if i.values.TLSSecretName != nil && i.values.IstioIngressGatewayNamespaceFunc != nil {
		objects = append(objects, i.emptyWildcardSecret())
	}
	return kubernetesutils.DeleteObjects(ctx, i.client, objects...)
}

func (i *ingress) emptyDestinationRule() *istionetworkingv1beta1.DestinationRule {
	serviceNamespace := i.namespace
	if i.values.ServiceNamespace != "" {
		serviceNamespace = i.values.ServiceNamespace
	}
	return &istionetworkingv1beta1.DestinationRule{ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.DeploymentNameKubeAPIServer + "-ingress", Namespace: serviceNamespace}}
}

func (i *ingress) emptyGateway() *istionetworkingv1beta1.Gateway {
	return &istionetworkingv1beta1.Gateway{ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.DeploymentNameKubeAPIServer + "-ingress", Namespace: i.namespace}}
}

func (i *ingress) emptyVirtualService() *istionetworkingv1beta1.VirtualService {
	return &istionetworkingv1beta1.VirtualService{ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.DeploymentNameKubeAPIServer + "-ingress", Namespace: i.namespace}}
}

func (i *ingress) emptyWildcardSecret() *corev1.Secret {
	return &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: *i.values.TLSSecretName, Namespace: i.values.IstioIngressGatewayNamespaceFunc()}}
}
