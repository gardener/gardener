// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package prometheus

import (
	"context"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

func (p *prometheus) ingress(ctx context.Context) (*networkingv1.Ingress, error) {
	if p.values.Ingress == nil {
		return nil, nil
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

	obj := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      p.name(),
			Namespace: p.namespace,
			Labels:    p.getLabels(),
			Annotations: map[string]string{
				"nginx.ingress.kubernetes.io/auth-type":   "basic",
				"nginx.ingress.kubernetes.io/auth-realm":  "Authentication Required",
				"nginx.ingress.kubernetes.io/auth-secret": p.values.Ingress.AuthSecretName,
			},
		},
		Spec: networkingv1.IngressSpec{
			IngressClassName: ptr.To(v1beta1constants.SeedNginxIngressClass),
			TLS: []networkingv1.IngressTLS{{
				SecretName: tlsSecretName,
				Hosts:      []string{p.values.Ingress.Host},
			}},
			Rules: []networkingv1.IngressRule{{
				Host: p.values.Ingress.Host,
				IngressRuleValue: networkingv1.IngressRuleValue{
					HTTP: &networkingv1.HTTPIngressRuleValue{
						Paths: []networkingv1.HTTPIngressPath{{
							Backend: networkingv1.IngressBackend{
								Service: &networkingv1.IngressServiceBackend{
									Name: p.name(),
									Port: networkingv1.ServiceBackendPort{Number: servicePort},
								},
							},
							Path:     "/",
							PathType: ptr.To(networkingv1.PathTypePrefix),
						}},
					},
				},
			}},
		},
	}

	if p.values.Ingress.BlockManagementAndTargetAPIAccess {
		metav1.SetMetaDataAnnotation(&obj.ObjectMeta, "nginx.ingress.kubernetes.io/server-snippet", `location /-/reload {
  return 403;
}
location /-/quit {
  return 403;
}
location /api/v1/targets {
  return 403;
}`)
	}

	return obj, nil
}
