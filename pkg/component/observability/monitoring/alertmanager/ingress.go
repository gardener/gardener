// Copyright 2024 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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

	tlsSecretName := ptr.Deref(a.values.Ingress.WildcardCertName, "")
	if tlsSecretName == "" && a.values.Ingress.SecretsManager != nil {
		ingressTLSSecret, err := a.values.Ingress.SecretsManager.Generate(ctx, &secretsutils.CertificateSecretConfig{
			Name:                        a.name() + "-tls",
			CommonName:                  a.name(),
			Organization:                []string{"gardener.cloud:monitoring:ingress"},
			DNSNames:                    []string{a.values.Ingress.Host},
			CertType:                    secretsutils.ServerCert,
			Validity:                    ptr.To(v1beta1constants.IngressTLSCertificateValidity),
			SkipPublishingCACertificate: true,
		}, secretsmanager.SignedByCA(v1beta1constants.SecretNameCACluster))
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
