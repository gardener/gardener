// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package kubeapiserverexposure

import (
	"context"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/component"
	kubeapiserverconstants "github.com/gardener/gardener/pkg/component/kubeapiserver/constants"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/utils"
)

const (
	nginxIngressSSLPassthrough       = "nginx.ingress.kubernetes.io/ssl-passthrough"
	nginxIngressBackendProtocol      = "nginx.ingress.kubernetes.io/backend-protocol"
	nginxIngressBackendProtocolHTTPS = "HTTPS"
)

// IngressValues configure the kube-apiserver ingress.
type IngressValues struct {
	// Host is the host where the kube-apiserver should be exposed.
	Host string
	// IngressClassName is the name of the ingress class.
	IngressClassName *string
	// ServiceName is the name of the service the ingress is using.
	ServiceName string
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
	ingress := i.emptyIngress()

	_, err := controllerutils.GetAndCreateOrMergePatch(ctx, i.client, ingress, func() error {
		if i.values.TLSSecretName == nil {
			metav1.SetMetaDataAnnotation(&ingress.ObjectMeta, nginxIngressSSLPassthrough, "true")
		} else {
			metav1.SetMetaDataAnnotation(&ingress.ObjectMeta, nginxIngressBackendProtocol, nginxIngressBackendProtocolHTTPS)
		}
		ingress.Labels = utils.MergeStringMaps(ingress.Labels, getLabels())
		pathType := networkingv1.PathTypePrefix
		ingress.Spec = networkingv1.IngressSpec{
			IngressClassName: i.values.IngressClassName,
			Rules: []networkingv1.IngressRule{
				{
					Host: i.values.Host,
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: i.values.ServiceName,
											Port: networkingv1.ServiceBackendPort{
												Number: kubeapiserverconstants.Port,
											},
										},
									},
									Path:     "/",
									PathType: &pathType,
								},
							},
						},
					},
				},
			},
			TLS: []networkingv1.IngressTLS{{Hosts: []string{i.values.Host}, SecretName: pointer.StringDeref(i.values.TLSSecretName, "")}},
		}
		return nil
	})
	return err
}

func (i *ingress) Destroy(ctx context.Context) error {
	return client.IgnoreNotFound(i.client.Delete(ctx, i.emptyIngress()))
}

func (i *ingress) emptyIngress() *networkingv1.Ingress {
	return &networkingv1.Ingress{ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.DeploymentNameKubeAPIServer, Namespace: i.namespace}}
}
