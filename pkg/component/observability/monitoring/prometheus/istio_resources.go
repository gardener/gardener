// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package prometheus

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
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	kubeapiserverconstants "github.com/gardener/gardener/pkg/component/kubernetes/apiserver/constants"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/istio"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

func (p *prometheus) istioResources(ctx context.Context) ([]client.Object, error) {
	if p.values.Ingress == nil {
		return nil, nil
	}

	var (
		// Currently, all observability components are exposed via the same istio ingress gateway.
		// When zonal gateways or exposure classes should be considered, the namespace needs to be dynamic.
		// See https://github.com/gardener/gardener/issues/11860 for details.
		ingressNamespace = v1beta1constants.DefaultSNIIngressNamespace
		gatewayName      = p.name()
	)

	if p.values.Ingress.IsGardenCluster {
		ingressNamespace = operatorv1alpha1.VirtualGardenNamePrefix + v1beta1constants.DefaultSNIIngressNamespace
		gatewayName = operatorv1alpha1.VirtualGardenNamePrefix + gatewayName
	}

	tlsSecretName := ptr.Deref(p.values.Ingress.WildcardCertSecretName, "")
	if tlsSecretName == "" && p.values.Ingress.SecretsManager != nil {
		ingressTLSSecret, err := p.values.Ingress.SecretsManager.Generate(ctx, &secretsutils.CertificateSecretConfig{
			Name:                        p.name() + "-tls",
			CommonName:                  p.name(),
			Organization:                []string{"gardener.cloud:monitoring:ingress"},
			DNSNames:                    []string{p.values.Ingress.Host},
			CertType:                    secretsutils.ServerCert,
			Validity:                    ptr.To(v1beta1constants.IngressTLSCertificateValidity),
			SkipPublishingCACertificate: true,
		}, secretsmanager.SignedByCA(p.values.Ingress.SigningCA))
		if err != nil {
			return nil, err
		}
		tlsSecretName = ingressTLSSecret.Name
	}

	// Istio expects the secret in the istio ingress gateway namespace => copy certificate to istio namespace
	tlsSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      tlsSecretName,
			Namespace: p.namespace,
		},
	}
	if err := p.client.Get(ctx, client.ObjectKeyFromObject(tlsSecret), tlsSecret); err != nil {
		return nil, fmt.Errorf("failed to get TLS secret %q: %w", tlsSecretName, err)
	}

	tlsSecretInIstioNamespace := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s-%s", p.namespace, p.name(), tlsSecretName),
			Namespace: ingressNamespace,
			Labels:    p.getLabels(),
		},
		Data: tlsSecret.Data,
	}

	backendPort := servicePorts.Web.Port
	if p.values.Cortex != nil {
		backendPort = servicePorts.Cortex.Port
	}

	gateway := &istionetworkingv1beta1.Gateway{ObjectMeta: metav1.ObjectMeta{Name: gatewayName, Namespace: p.namespace}}
	if err := istio.GatewayWithTLSTermination(
		gateway,
		p.getLabels(),
		p.values.Ingress.IstioIngressGatewayLabels,
		[]string{p.values.Ingress.Host},
		kubeapiserverconstants.Port,
		tlsSecretInIstioNamespace.Name,
	)(); err != nil {
		return nil, fmt.Errorf("failed to create gateway resource: %w", err)
	}

	destinationHost := kubernetesutils.FQDNForService(p.name(), p.namespace)
	virtualService := &istionetworkingv1beta1.VirtualService{ObjectMeta: metav1.ObjectMeta{Name: gatewayName, Namespace: p.namespace}}
	if err := istio.VirtualServiceForTLSTermination(
		virtualService,
		utils.MergeStringMaps(p.getLabels(), map[string]string{v1beta1constants.LabelBasicAuthSecretName: p.values.Ingress.AuthSecretName}),
		[]string{p.values.Ingress.Host},
		gatewayName,
		uint32(backendPort), // #nosec: G115 -- only constants 80 and 81 are used, whose conversion is safe
		destinationHost,
		"",
		"",
	)(); err != nil {
		return nil, fmt.Errorf("failed to create virtual service resource: %w", err)
	}
	if p.values.Ingress.BlockManagementAndTargetAPIAccess {
		virtualService.Spec.Http = append([]*istioapinetworkingv1beta1.HTTPRoute{{
			Name: "disabled-endpoints",
			Match: []*istioapinetworkingv1beta1.HTTPMatchRequest{
				{
					Uri: &istioapinetworkingv1beta1.StringMatch{
						MatchType: &istioapinetworkingv1beta1.StringMatch_Prefix{
							Prefix: "/-/reload",
						},
					},
				},
				{
					Uri: &istioapinetworkingv1beta1.StringMatch{
						MatchType: &istioapinetworkingv1beta1.StringMatch_Prefix{
							Prefix: "/-/quit",
						},
					},
				},
				{
					Uri: &istioapinetworkingv1beta1.StringMatch{
						MatchType: &istioapinetworkingv1beta1.StringMatch_Prefix{
							Prefix: "/api/v1/targets",
						},
					},
				},
			},
			DirectResponse: &istioapinetworkingv1beta1.HTTPDirectResponse{
				Status: 403,
			},
		}}, virtualService.Spec.Http...)
	}

	destinationRule := &istionetworkingv1beta1.DestinationRule{ObjectMeta: metav1.ObjectMeta{Name: gatewayName, Namespace: p.namespace}}
	if err := istio.DestinationRuleWithLocalityPreference(destinationRule, p.getLabels(), destinationHost)(); err != nil {
		return nil, fmt.Errorf("failed to create destination rule resource: %w", err)
	}

	return []client.Object{tlsSecretInIstioNamespace, gateway, virtualService, destinationRule}, nil
}
