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

package apiserver

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	webhookadmissionv1 "k8s.io/apiserver/pkg/admission/plugin/webhook/config/apis/webhookadmission/v1"
	webhookadmissionv1alpha1 "k8s.io/apiserver/pkg/admission/plugin/webhook/config/apis/webhookadmission/v1alpha1"
	apiserverv1alpha1 "k8s.io/apiserver/pkg/apis/apiserver/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

var admissionCodec runtime.Codec

func init() {
	admissionScheme := runtime.NewScheme()
	utilruntime.Must(apiserverv1alpha1.AddToScheme(admissionScheme))
	utilruntime.Must(webhookadmissionv1.AddToScheme(admissionScheme))
	utilruntime.Must(webhookadmissionv1alpha1.AddToScheme(admissionScheme))

	var (
		ser = json.NewSerializerWithOptions(json.DefaultMetaFactory, admissionScheme, admissionScheme, json.SerializerOptions{
			Yaml:   true,
			Pretty: false,
			Strict: false,
		})
		versions = schema.GroupVersions([]schema.GroupVersion{
			apiserverv1alpha1.SchemeGroupVersion,
			webhookadmissionv1.SchemeGroupVersion,
			webhookadmissionv1alpha1.SchemeGroupVersion,
		})
	)

	admissionCodec = serializer.NewCodecFactory(admissionScheme).CodecForVersions(ser, ser, versions, versions)
}

const (
	// ConfigMapAdmissionDataKey is a constant for a key in the data of the ConfigMap containing the configuration of
	// admission plugins.
	ConfigMapAdmissionDataKey = "admission-configuration.yaml"
	// VolumeMountPathAdmissionConfiguration is a constant for the volume mount path of the admission configuration
	// files.
	VolumeMountPathAdmissionConfiguration = "/etc/kubernetes/admission"
	// VolumeMountPathAdmissionKubeconfigSecrets is a constant for the volume mount path of the admission kubeconfig
	// files.
	VolumeMountPathAdmissionKubeconfigSecrets = "/etc/kubernetes/admission-kubeconfigs"
)

// ReconcileSecretAdmissionKubeconfigs reconciles the secret containing the kubeconfig for admission plugins.
func ReconcileSecretAdmissionKubeconfigs(ctx context.Context, c client.Client, secret *corev1.Secret, values Values) error {
	secret.Data = make(map[string][]byte)

	for _, plugin := range values.EnabledAdmissionPlugins {
		if len(plugin.Kubeconfig) != 0 {
			secret.Data[admissionPluginsKubeconfigFilename(plugin.Name)] = plugin.Kubeconfig
		}
	}

	utilruntime.Must(kubernetesutils.MakeUnique(secret))
	return client.IgnoreAlreadyExists(c.Create(ctx, secret))
}

// ReconcileConfigMapAdmission reconciles the ConfigMap containing the configs for the admission plugins.
func ReconcileConfigMapAdmission(ctx context.Context, c client.Client, configMap *corev1.ConfigMap, values Values) error {
	configMap.Data = map[string]string{}

	admissionConfig := &apiserverv1alpha1.AdmissionConfiguration{}
	for _, plugin := range values.EnabledAdmissionPlugins {
		rawConfig, err := computeRelevantAdmissionPluginRawConfig(plugin)
		if err != nil {
			return err
		}

		if rawConfig != nil {
			admissionConfig.Plugins = append(admissionConfig.Plugins, apiserverv1alpha1.AdmissionPluginConfiguration{
				Name: plugin.Name,
				Path: VolumeMountPathAdmissionConfiguration + "/" + admissionPluginsConfigFilename(plugin.Name),
			})

			configMap.Data[admissionPluginsConfigFilename(plugin.Name)] = string(rawConfig)
		}
	}

	data, err := runtime.Encode(admissionCodec, admissionConfig)
	if err != nil {
		return err
	}

	configMap.Data[ConfigMapAdmissionDataKey] = string(data)
	utilruntime.Must(kubernetesutils.MakeUnique(configMap))

	return client.IgnoreAlreadyExists(c.Create(ctx, configMap))
}

func computeRelevantAdmissionPluginRawConfig(plugin AdmissionPluginConfig) ([]byte, error) {
	var (
		nothingToMutate    = (plugin.Config == nil || plugin.Config.Raw == nil) && len(plugin.Kubeconfig) == 0
		mustDefaultConfig  = (plugin.Config == nil || plugin.Config.Raw == nil) && len(plugin.Kubeconfig) > 0
		kubeconfigFilePath = VolumeMountPathAdmissionKubeconfigSecrets + "/" + admissionPluginsKubeconfigFilename(plugin.Name)
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

		configObj, err := runtime.Decode(admissionCodec, plugin.Config.Raw)
		if err != nil {
			return nil, fmt.Errorf("cannot decode config for admission plugin %s: %w", plugin.Name, err)
		}

		switch config := configObj.(type) {
		case *webhookadmissionv1.WebhookAdmission:
			config.KubeConfigFile = kubeconfigFilePath
			return runtime.Encode(admissionCodec, config)
		case *webhookadmissionv1alpha1.WebhookAdmission:
			config.KubeConfigFile = kubeconfigFilePath
			return runtime.Encode(admissionCodec, config)
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

func admissionPluginsConfigFilename(name string) string {
	return strings.ToLower(name) + ".yaml"
}

func admissionPluginsKubeconfigFilename(name string) string {
	return strings.ToLower(name) + "-kubeconfig.yaml"
}
