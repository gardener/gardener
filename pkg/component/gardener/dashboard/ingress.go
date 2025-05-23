// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package dashboard

import (
	"context"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
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

func (g *gardenerDashboard) ingress(ctx context.Context) (*networkingv1.Ingress, error) {
	tlsSecretName := ptr.Deref(g.values.Ingress.WildcardCertSecretName, "")
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
		tlsSecretName = ingressTLSSecret.Name
	}

	obj := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploymentName,
			Namespace: g.namespace,
			Labels:    GetLabels(),
			Annotations: map[string]string{
				"nginx.ingress.kubernetes.io/ssl-redirect":          "true",
				"nginx.ingress.kubernetes.io/use-port-in-redirects": "true",
			},
		},
		Spec: networkingv1.IngressSpec{
			IngressClassName: ptr.To(v1beta1constants.SeedNginxIngressClass),
			TLS: []networkingv1.IngressTLS{{
				SecretName: tlsSecretName,
				Hosts:      g.ingressHosts(),
			}},
		},
	}

	for _, host := range g.ingressHosts() {
		obj.Spec.Rules = append(obj.Spec.Rules, networkingv1.IngressRule{
			Host: host,
			IngressRuleValue: networkingv1.IngressRuleValue{
				HTTP: &networkingv1.HTTPIngressRuleValue{
					Paths: []networkingv1.HTTPIngressPath{{
						Backend: networkingv1.IngressBackend{
							Service: &networkingv1.IngressServiceBackend{
								Name: serviceName,
								Port: networkingv1.ServiceBackendPort{Number: portServer},
							},
						},
						Path:     "/",
						PathType: ptr.To(networkingv1.PathTypePrefix),
					}},
				},
			},
		})
	}

	return obj, nil
}
