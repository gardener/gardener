// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package apiserver

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	apiserverv1alpha1 "k8s.io/apiserver/pkg/apis/apiserver/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	vpnseedserver "github.com/gardener/gardener/pkg/component/networking/vpn/seedserver"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
)

const (
	configMapAdmissionNamePrefix            = "kube-apiserver-admission-config"
	configMapAuditPolicyNamePrefix          = "audit-policy-config"
	configMapAuthenticationConfigNamePrefix = "kube-apiserver-authentication-config"
	configMapAuthorizationConfigNamePrefix  = "kube-apiserver-authorization-config"
	configMapEgressSelectorNamePrefix       = "kube-apiserver-egress-selector-config"
	configMapEgressSelectorDataKey          = "egress-selector-configuration.yaml"
	configMapEnvoyConfigPrefix              = "kube-apiserver-envoy-config"
	configMapEnvoyConfigDataKey             = "envoy.yaml"
)

func (k *kubeAPIServer) emptyConfigMap(name string) *corev1.ConfigMap {
	return &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: k.namespace}}
}

func (k *kubeAPIServer) reconcileConfigMapEgressSelector(ctx context.Context, configMap *corev1.ConfigMap) error {
	if !k.values.VPN.Enabled {
		// We don't delete the configmap here as we don't know its name (as it's unique). Instead, we rely on the usual
		// garbage collection for unique secrets/configmaps.
		return nil
	}

	proxyHost := vpnseedserver.ServiceName
	proxyPort := vpnseedserver.EnvoyPort
	if k.values.VPN.HighAvailabilityEnabled {
		proxyHost = EnvoyHostHAVPN
		proxyPort = EnvoyPortHAVPN
	}

	egressSelectorConfig := &apiserverv1alpha1.EgressSelectorConfiguration{
		EgressSelections: []apiserverv1alpha1.EgressSelection{
			{
				Name: "cluster",
				Connection: apiserverv1alpha1.Connection{
					ProxyProtocol: apiserverv1alpha1.ProtocolHTTPConnect,
					Transport: &apiserverv1alpha1.Transport{
						TCP: &apiserverv1alpha1.TCPTransport{
							URL: fmt.Sprintf("https://%s:%d", proxyHost, proxyPort),
							TLSConfig: &apiserverv1alpha1.TLSConfig{
								CABundle:   fmt.Sprintf("%s/%s", volumeMountPathCAVPN, secretsutils.DataKeyCertificateBundle),
								ClientCert: fmt.Sprintf("%s/%s", volumeMountPathHTTPProxyClient, secretsutils.DataKeyCertificate),
								ClientKey:  fmt.Sprintf("%s/%s", volumeMountPathHTTPProxyClient, secretsutils.DataKeyPrivateKey),
							},
						},
					},
				},
			},
			{
				Name:       "controlplane",
				Connection: apiserverv1alpha1.Connection{ProxyProtocol: apiserverv1alpha1.ProtocolDirect},
			},
			{
				Name:       "etcd",
				Connection: apiserverv1alpha1.Connection{ProxyProtocol: apiserverv1alpha1.ProtocolDirect},
			},
		},
	}

	data, err := runtime.Encode(ConfigCodec, egressSelectorConfig)
	if err != nil {
		return err
	}

	configMap.Data = map[string]string{configMapEgressSelectorDataKey: string(data)}
	utilruntime.Must(kubernetesutils.MakeUnique(configMap))

	return client.IgnoreAlreadyExists(k.client.Client().Create(ctx, configMap))
}

func (k *kubeAPIServer) reconcileConfigMapEnvoyConfig(ctx context.Context, configMap *corev1.ConfigMap) error {
	if !k.values.VPN.Enabled && !k.values.VPN.HighAvailabilityEnabled {
		return nil
	}

	envoyConfig, err := k.getEnvoyConfig()
	if err != nil {
		return err
	}

	configMap.Data = map[string]string{configMapEnvoyConfigDataKey: envoyConfig}

	utilruntime.Must(kubernetesutils.MakeUnique(configMap))
	return client.IgnoreAlreadyExists(k.client.Client().Create(ctx, configMap))
}
