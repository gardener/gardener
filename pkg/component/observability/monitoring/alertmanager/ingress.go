// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package alertmanager

import (
	"context"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

func (a *alertManager) ingress(ctx context.Context) (*networkingv1.Ingress, error) {
	if a.values.Ingress == nil {
		return nil, nil
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

	return &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      a.name(),
			Namespace: a.namespace,
			Labels:    a.getLabels(),
			Annotations: map[string]string{
				"nginx.ingress.kubernetes.io/auth-type":   "basic",
				"nginx.ingress.kubernetes.io/auth-realm":  "Authentication Required",
				"nginx.ingress.kubernetes.io/auth-secret": a.values.Ingress.AuthSecretName,
				"nginx.ingress.kubernetes.io/server-snippet": `location /-/reload {
  return 403;
}`,
			},
		},
		Spec: networkingv1.IngressSpec{
			IngressClassName: ptr.To(v1beta1constants.SeedNginxIngressClass),
			TLS: []networkingv1.IngressTLS{{
				SecretName: tlsSecretName,
				Hosts:      []string{a.values.Ingress.Host},
			}},
			Rules: []networkingv1.IngressRule{{
				Host: a.values.Ingress.Host,
				IngressRuleValue: networkingv1.IngressRuleValue{
					HTTP: &networkingv1.HTTPIngressRuleValue{
						Paths: []networkingv1.HTTPIngressPath{{
							Backend: networkingv1.IngressBackend{
								Service: &networkingv1.IngressServiceBackend{
									Name: a.name(),
									Port: networkingv1.ServiceBackendPort{Number: port},
								},
							},
							Path:     "/",
							PathType: ptr.To(networkingv1.PathTypePrefix),
						}},
					},
				},
			}},
		},
	}, nil
}
