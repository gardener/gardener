// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package dashboard

import (
	"context"
	"fmt"

	istionetworkingv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	kubeapiserverconstants "github.com/gardener/gardener/pkg/component/kubernetes/apiserver/constants"
	"github.com/gardener/gardener/pkg/utils/istio"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

func (g *gardenerDashboard) ingressHosts() []string {
	hosts := make([]string, 0, len(g.values.Ingress.Domains))
	for _, domain := range g.values.Ingress.Domains {
		hosts = append(hosts, "dashboard."+domain)
	}
	return hosts
}

func (g *gardenerDashboard) istioResources(ctx context.Context) ([]client.Object, error) {
	var (
		gatewayName   = deploymentName
		tlsSecretName = ptr.Deref(g.values.Ingress.WildcardCertSecretName, "")
		tlsSecret     *corev1.Secret
	)

	if tlsSecretName == "" {
		ingressTLSSecret, err := g.secretsManager.Generate(ctx, &secretsutils.CertificateSecretConfig{
			Name:                        deploymentName + "-tls",
			CommonName:                  deploymentName,
			DNSNames:                    g.ingressHosts(),
			CertType:                    secretsutils.ServerCert,
			Validity:                    ptr.To(v1beta1constants.IngressTLSCertificateValidity),
			SkipPublishingCACertificate: true,
		}, secretsmanager.SignedByCA(operatorv1alpha1.SecretNameCAGardener))
		if err != nil {
			return nil, err
		}
		tlsSecret = ingressTLSSecret
	} else {
		tlsSecret = &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: tlsSecretName, Namespace: g.namespace}}
		if err := g.client.Get(ctx, client.ObjectKeyFromObject(tlsSecret), tlsSecret); err != nil {
			return nil, fmt.Errorf("failed to get TLS secret %q: %w", tlsSecretName, err)
		}
	}

	// Istio expects the secret in the istio ingress gateway namespace => copy certificate to istio namespace
	tlsSecretInIstioNamespace := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s-%s", g.namespace, deploymentName, tlsSecret.Name),
			Namespace: operatorv1alpha1.VirtualGardenNamePrefix + v1beta1constants.DefaultSNIIngressNamespace,
			Labels:    GetLabels(),
		},
		Data: tlsSecret.Data,
	}

	gateway := &istionetworkingv1beta1.Gateway{ObjectMeta: metav1.ObjectMeta{Name: gatewayName, Namespace: g.namespace}}
	if err := istio.GatewayWithTLSTermination(
		gateway,
		GetLabels(),
		g.values.Ingress.IstioIngressGatewayLabels,
		g.ingressHosts(),
		kubeapiserverconstants.Port,
		tlsSecretInIstioNamespace.Name,
	)(); err != nil {
		return nil, fmt.Errorf("failed to create gateway resource: %w", err)
	}

	destinationHost := kubernetesutils.FQDNForService(serviceName, g.namespace)
	virtualService := &istionetworkingv1beta1.VirtualService{ObjectMeta: metav1.ObjectMeta{Name: gatewayName, Namespace: g.namespace}}
	if err := istio.VirtualServiceForTLSTermination(
		virtualService,
		GetLabels(),
		g.ingressHosts(),
		gatewayName,
		portServer,
		destinationHost,
		"",
		"",
	)(); err != nil {
		return nil, fmt.Errorf("failed to create virtual service resource: %w", err)
	}

	destinationRule := &istionetworkingv1beta1.DestinationRule{ObjectMeta: metav1.ObjectMeta{Name: gatewayName, Namespace: g.namespace}}
	if err := istio.DestinationRuleWithLocalityPreference(destinationRule, GetLabels(), destinationHost)(); err != nil {
		return nil, fmt.Errorf("failed to create destination rule resource: %w", err)
	}

	return []client.Object{tlsSecretInIstioNamespace, gateway, virtualService, destinationRule}, nil
}
