// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package alertmanager

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
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/istio"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

func (a *alertManager) istioResources(ctx context.Context) ([]client.Object, error) {
	if a.values.Ingress == nil {
		return nil, nil
	}

	var (
		// Currently, all observability components are exposed via the same istio ingress gateway.
		// When zonal gateways or exposure classes should be considered, the namespace needs to be dynamic.
		// See https://github.com/gardener/gardener/issues/11860 for details.
		ingressNamespace = v1beta1constants.DefaultSNIIngressNamespace
		gatewayName      = a.name()
	)

	if a.values.Ingress.IsGardenCluster {
		ingressNamespace = operatorv1alpha1.VirtualGardenNamePrefix + v1beta1constants.DefaultSNIIngressNamespace
		gatewayName = operatorv1alpha1.VirtualGardenNamePrefix + gatewayName
	}

	tlsSecretName := ptr.Deref(a.values.Ingress.WildcardCertSecretName, "")
	if tlsSecretName == "" && a.values.Ingress.SecretsManager != nil {
		ingressTLSSecret, err := a.values.Ingress.SecretsManager.Generate(ctx, &secretsutils.CertificateSecretConfig{
			Name:                        a.name() + "-tls",
			CommonName:                  a.name(),
			Organization:                []string{"gardener.cloud:monitoring:ingress"},
			DNSNames:                    []string{a.values.Ingress.Host},
			CertType:                    secretsutils.ServerCert,
			Validity:                    ptr.To(v1beta1constants.IngressTLSCertificateValidity),
			SkipPublishingCACertificate: true,
		}, secretsmanager.SignedByCA(a.values.Ingress.SigningCA))
		if err != nil {
			return nil, err
		}
		tlsSecretName = ingressTLSSecret.Name
	}

	// Istio expects the secret in the istio ingress gateway namespace => copy certificate to istio namespace
	tlsSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      tlsSecretName,
			Namespace: a.namespace,
		},
	}
	if err := a.client.Get(ctx, client.ObjectKeyFromObject(tlsSecret), tlsSecret); err != nil {
		return nil, fmt.Errorf("failed to get TLS secret %q: %w", tlsSecretName, err)
	}

	tlsSecretInIstioNamespace := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s-%s", a.namespace, a.name(), tlsSecretName),
			Namespace: ingressNamespace,
			Labels:    a.getLabels(),
		},
		Data: tlsSecret.Data,
	}

	gateway := &istionetworkingv1beta1.Gateway{ObjectMeta: metav1.ObjectMeta{Name: gatewayName, Namespace: a.namespace}}
	if err := istio.GatewayWithTLSTermination(
		gateway,
		a.getLabels(),
		a.values.Ingress.IstioIngressGatewayLabels,
		[]string{a.values.Ingress.Host},
		tlsSecretInIstioNamespace.Name,
	)(); err != nil {
		return nil, fmt.Errorf("failed to create gateway resource: %w", err)
	}

	destinationHost := kubernetesutils.FQDNForService(a.name(), a.namespace)
	virtualService := &istionetworkingv1beta1.VirtualService{ObjectMeta: metav1.ObjectMeta{Name: gatewayName, Namespace: a.namespace}}
	if err := istio.VirtualServiceForTLSTermination(
		virtualService,
		utils.MergeStringMaps(a.getLabels(), map[string]string{v1beta1constants.LabelBasicAuthSecretName: a.values.Ingress.AuthSecretName}),
		[]string{a.values.Ingress.Host},
		gatewayName,
		port,
		destinationHost,
		"",
		"",
	)(); err != nil {
		return nil, fmt.Errorf("failed to create virtual service resource: %w", err)
	}
	virtualService.Spec.Http = append([]*istioapinetworkingv1beta1.HTTPRoute{{
		Name: "reload-endpoint",
		Match: []*istioapinetworkingv1beta1.HTTPMatchRequest{{
			Uri: &istioapinetworkingv1beta1.StringMatch{
				MatchType: &istioapinetworkingv1beta1.StringMatch_Prefix{
					Prefix: "/-/reload",
				},
			},
		}},
		DirectResponse: &istioapinetworkingv1beta1.HTTPDirectResponse{
			Status: 403,
		},
	}}, virtualService.Spec.Http...)

	destinationRule := &istionetworkingv1beta1.DestinationRule{ObjectMeta: metav1.ObjectMeta{Name: gatewayName, Namespace: a.namespace}}
	if err := istio.DestinationRuleWithLocalityPreference(destinationRule, a.getLabels(), destinationHost)(); err != nil {
		return nil, fmt.Errorf("failed to create destination rule resource: %w", err)
	}

	return []client.Object{tlsSecretInIstioNamespace, gateway, virtualService, destinationRule}, nil
}
