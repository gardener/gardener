// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package kubeapiserver

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	webhookadmissionv1 "k8s.io/apiserver/pkg/admission/plugin/webhook/config/apis/webhookadmission/v1"
	webhookadmissionv1alpha1 "k8s.io/apiserver/pkg/admission/plugin/webhook/config/apis/webhookadmission/v1alpha1"
	apiserverv1alpha1 "k8s.io/apiserver/pkg/apis/apiserver/v1alpha1"
	auditv1 "k8s.io/apiserver/pkg/apis/audit/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"github.com/gardener/gardener/pkg/operation/botanist/component/vpnseedserver"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
)

const (
	configMapAdmissionNamePrefix = "kube-apiserver-admission-config"
	configMapAdmissionDataKey    = "admission-configuration.yaml"

	configMapAuditPolicyNamePrefix = "audit-policy-config"
	configMapAuditPolicyDataKey    = "audit-policy.yaml"

	configMapEgressSelectorNamePrefix = "kube-apiserver-egress-selector-config"
	configMapEgressSelectorDataKey    = "egress-selector-configuration.yaml"
)

func (k *kubeAPIServer) emptyConfigMap(name string) *corev1.ConfigMap {
	return &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: k.namespace}}
}

func (k *kubeAPIServer) reconcileConfigMapAdmission(ctx context.Context, configMap *corev1.ConfigMap) error {
	configMap.Data = map[string]string{}

	admissionConfig := &apiserverv1alpha1.AdmissionConfiguration{}
	for _, plugin := range k.values.EnabledAdmissionPlugins {
		rawConfig, err := computeRelevantAdmissionPluginRawConfig(plugin)
		if err != nil {
			return err
		}

		if rawConfig != nil {
			admissionConfig.Plugins = append(admissionConfig.Plugins, apiserverv1alpha1.AdmissionPluginConfiguration{
				Name: plugin.Name,
				Path: volumeMountPathAdmissionConfiguration + "/" + admissionPluginsConfigFilename(plugin.Name),
			})

			configMap.Data[admissionPluginsConfigFilename(plugin.Name)] = string(rawConfig)
		}
	}

	data, err := runtime.Encode(codec, admissionConfig)
	if err != nil {
		return err
	}

	configMap.Data[configMapAdmissionDataKey] = string(data)
	utilruntime.Must(kubernetesutils.MakeUnique(configMap))

	return client.IgnoreAlreadyExists(k.client.Client().Create(ctx, configMap))
}

func admissionPluginsConfigFilename(name string) string {
	return strings.ToLower(name) + ".yaml"
}

func computeRelevantAdmissionPluginRawConfig(plugin AdmissionPluginConfig) ([]byte, error) {
	var (
		nothingToMutate    = (plugin.Config == nil || plugin.Config.Raw == nil) && len(plugin.Kubeconfig) == 0
		mustDefaultConfig  = (plugin.Config == nil || plugin.Config.Raw == nil) && len(plugin.Kubeconfig) > 0
		kubeconfigFilePath = volumeMountPathAdmissionKubeconfigSecrets + "/" + admissionPluginsKubeconfigFilename(plugin.Name)
	)

	if len(plugin.Kubeconfig) == 0 {
		// This makes sure that the path to the kubeconfig is overwritten if specified in case no kubeconfig was
		// provided. It prevents that users can access arbitrary files in the kube-apiserver pods and disguise them as
		// kubeconfigs for their admission plugin configs.
		kubeconfigFilePath = ""
	}

	switch plugin.Name {
	case "ValidatingAdmissionWebhook", "MutatingAdmissionWebhook":
		if nothingToMutate {
			return nil, nil
		}

		if mustDefaultConfig {
			if plugin.Config == nil {
				plugin.Config = &runtime.RawExtension{}
			}
			if len(plugin.Config.Raw) == 0 {
				plugin.Config.Raw = []byte(fmt.Sprintf(`apiVersion: %s
kind: WebhookAdmissionConfiguration`, webhookadmissionv1.SchemeGroupVersion.String()))
			}
		}

		configObj, err := runtime.Decode(codec, plugin.Config.Raw)
		if err != nil {
			return nil, fmt.Errorf("cannot decode config for admission plugin %s: %w", plugin.Name, err)
		}

		switch config := configObj.(type) {
		case *webhookadmissionv1.WebhookAdmission:
			config.KubeConfigFile = kubeconfigFilePath
			return runtime.Encode(codec, config)
		case *webhookadmissionv1alpha1.WebhookAdmission:
			config.KubeConfigFile = kubeconfigFilePath
			return runtime.Encode(codec, config)
		default:
			return nil, fmt.Errorf("expected apiserver.config.k8s.io/{v1alpha1.WebhookAdmission,v1.WebhookAdmissionConfiguration} in %s plugin configuration but got %T", plugin.Name, config)
		}

	case "ImagePolicyWebhook":
		// The configuration for this admission plugin is not backed by the API machinery, hence we have to use
		// regular marshalling.
		if nothingToMutate {
			return nil, nil
		}

		if mustDefaultConfig {
			if plugin.Config == nil {
				plugin.Config = &runtime.RawExtension{}
			}
			if len(plugin.Config.Raw) == 0 {
				plugin.Config.Raw = []byte("imagePolicy: {}")
			}
		}

		config := map[string]interface{}{}
		if err := yaml.Unmarshal(plugin.Config.Raw, &config); err != nil {
			return nil, fmt.Errorf("failed to unmarshal plugin configuration for %s: %w", plugin.Name, err)
		}
		if config["imagePolicy"] == nil {
			return nil, fmt.Errorf(`expected "imagePolicy" key in configuration but it does not exist`)
		}

		config["imagePolicy"].(map[string]interface{})["kubeConfigFile"] = kubeconfigFilePath
		return yaml.Marshal(config)

	default:
		// For all other plugins, we do not need to mutate anything, hence we only return the provided config if set.
		if plugin.Config != nil && plugin.Config.Raw != nil {
			return plugin.Config.Raw, nil
		}
	}

	return nil, nil
}

func (k *kubeAPIServer) reconcileConfigMapAuditPolicy(ctx context.Context, configMap *corev1.ConfigMap) error {
	defaultPolicy := &auditv1.Policy{
		Rules: []auditv1.PolicyRule{
			{Level: auditv1.LevelNone},
		},
	}

	data, err := runtime.Encode(codec, defaultPolicy)
	if err != nil {
		return err
	}
	policy := string(data)

	if k.values.Audit != nil && k.values.Audit.Policy != nil {
		policy = *k.values.Audit.Policy
	}

	configMap.Data = map[string]string{configMapAuditPolicyDataKey: policy}
	utilruntime.Must(kubernetesutils.MakeUnique(configMap))
	return client.IgnoreAlreadyExists(k.client.Client().Create(ctx, configMap))
}

func (k *kubeAPIServer) reconcileConfigMapEgressSelector(ctx context.Context, configMap *corev1.ConfigMap) error {
	if !k.values.VPN.Enabled || k.values.VPN.HighAvailabilityEnabled {
		// We don't delete the configmap here as we don't know its name (as it's unique). Instead, we rely on the usual
		// garbage collection for unique secrets/configmaps.
		return nil
	}

	egressSelectorConfig := &apiserverv1alpha1.EgressSelectorConfiguration{
		EgressSelections: []apiserverv1alpha1.EgressSelection{
			{
				Name: "cluster",
				Connection: apiserverv1alpha1.Connection{
					ProxyProtocol: apiserverv1alpha1.ProtocolHTTPConnect,
					Transport: &apiserverv1alpha1.Transport{
						TCP: &apiserverv1alpha1.TCPTransport{
							URL: fmt.Sprintf("https://%s:%d", vpnseedserver.ServiceName, vpnseedserver.EnvoyPort),
							TLSConfig: &apiserverv1alpha1.TLSConfig{
								CABundle:   fmt.Sprintf("%s/%s", volumeMountPathCAVPN, secretsutils.DataKeyCertificateBundle),
								ClientCert: fmt.Sprintf("%s/%s", volumeMountPathHTTPProxy, secretsutils.DataKeyCertificate),
								ClientKey:  fmt.Sprintf("%s/%s", volumeMountPathHTTPProxy, secretsutils.DataKeyPrivateKey),
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

	data, err := runtime.Encode(codec, egressSelectorConfig)
	if err != nil {
		return err
	}

	configMap.Data = map[string]string{configMapEgressSelectorDataKey: string(data)}
	utilruntime.Must(kubernetesutils.MakeUnique(configMap))

	return client.IgnoreAlreadyExists(k.client.Client().Create(ctx, configMap))
}
