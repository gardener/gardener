// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package perses

import (
	"context"
	"fmt"

	istionetworkingv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/component/networking/istiobasicauthserver"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/istio"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

const persesContainerPort uint32 = 8080

func (p *perses) istioResources(ctx context.Context) ([]client.Object, error) {
	if p.values.ExternalExposure == nil {
		return nil, nil
	}

	var (
		ingressNamespace = p.values.ExternalExposure.IstioIngressGatewayNamespace
		gatewayName      = p.persesName()
	)

	tlsSecretName := ptr.Deref(p.values.ExternalExposure.WildcardCertSecretName, "")
	if tlsSecretName == "" && p.values.ExternalExposure.SecretsManager != nil {
		ingressTLSSecret, err := p.values.ExternalExposure.SecretsManager.Generate(ctx, &secretsutils.CertificateSecretConfig{
			Name:                        p.persesName() + "-tls",
			CommonName:                  p.persesName(),
			Organization:                []string{"gardener.cloud:monitoring:ingress"},
			DNSNames:                    []string{p.values.ExternalExposure.Host},
			CertType:                    secretsutils.ServerCert,
			Validity:                    new(v1beta1constants.IngressTLSCertificateValidity),
			SkipPublishingCACertificate: true,
		}, secretsmanager.SignedByCA(p.values.ExternalExposure.SigningCA))
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
			Name:      fmt.Sprintf("%s-%s-%s", p.namespace, p.persesName(), tlsSecretName),
			Namespace: ingressNamespace,
			Labels:    p.getLabels(),
		},
		Data: tlsSecret.Data,
	}

	gateway := &istionetworkingv1beta1.Gateway{ObjectMeta: metav1.ObjectMeta{Name: gatewayName, Namespace: p.namespace}}
	if err := istio.GatewayWithTLSTermination(
		gateway,
		p.getLabels(),
		p.values.ExternalExposure.IstioIngressGatewayLabels,
		[]string{p.values.ExternalExposure.Host},
		tlsSecretInIstioNamespace.Name,
	)(); err != nil {
		return nil, fmt.Errorf("failed to create gateway resource: %w", err)
	}

	destinationHost := kubernetesutils.FQDNForService(p.persesName(), p.namespace)
	virtualService := &istionetworkingv1beta1.VirtualService{ObjectMeta: metav1.ObjectMeta{Name: gatewayName, Namespace: p.namespace}}
	if err := istio.VirtualServiceForTLSTermination(
		virtualService,
		utils.MergeStringMaps(p.getLabels(), istiobasicauthserver.BasicAuthLabels(p.values.ExternalExposure.IsGardenCluster, p.values.ExternalExposure.AuthSecretName, p.values.ExternalExposure.AuthSecretManaged)),
		[]string{ingressNamespace},
		[]string{p.values.ExternalExposure.Host},
		gatewayName,
		persesContainerPort,
		destinationHost,
		"",
		"",
	)(); err != nil {
		return nil, fmt.Errorf("failed to create virtual service resource: %w", err)
	}

	destinationRule := &istionetworkingv1beta1.DestinationRule{ObjectMeta: metav1.ObjectMeta{Name: gatewayName, Namespace: p.namespace}}
	if err := istio.DestinationRuleWithLocalityPreference(destinationRule, p.getLabels(), []string{ingressNamespace}, destinationHost)(); err != nil {
		return nil, fmt.Errorf("failed to create destination rule resource: %w", err)
	}

	return []client.Object{tlsSecretInIstioNamespace, gateway, virtualService, destinationRule}, nil
}
