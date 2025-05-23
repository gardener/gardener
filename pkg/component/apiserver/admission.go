// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package apiserver

import (
	"context"
	"fmt"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
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
	configMapAdmissionDataKey = "admission-configuration.yaml"

	volumeNameAdmissionConfiguration          = "admission-config"
	volumeNameAdmissionKubeconfigSecrets      = "admission-kubeconfigs"
	volumeMountPathAdmissionConfiguration     = "/etc/kubernetes/admission"
	volumeMountPathAdmissionKubeconfigSecrets = "/etc/kubernetes/admission-kubeconfigs" // #nosec G101 -- No credential.
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
				Path: volumeMountPathAdmissionConfiguration + "/" + admissionPluginsConfigFilename(plugin.Name),
			})

			configMap.Data[admissionPluginsConfigFilename(plugin.Name)] = string(rawConfig)
		}
	}

	data, err := runtime.Encode(admissionCodec, admissionConfig)
	if err != nil {
		return err
	}

	configMap.Data[configMapAdmissionDataKey] = string(data)
	utilruntime.Must(kubernetesutils.MakeUnique(configMap))

	return client.IgnoreAlreadyExists(c.Create(ctx, configMap))
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

		config := map[string]any{}
		if err := yaml.Unmarshal(plugin.Config.Raw, &config); err != nil {
			return nil, fmt.Errorf("failed to unmarshal plugin configuration for %s: %w", plugin.Name, err)
		}
		if config["imagePolicy"] == nil {
			return nil, fmt.Errorf(`expected "imagePolicy" key in configuration but it does not exist`)
		}

		config["imagePolicy"].(map[string]any)["kubeConfigFile"] = kubeconfigFilePath
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

// InjectAdmissionSettings injects the admission settings into `gardener-apiserver` and `kube-apiserver` deployments.
func InjectAdmissionSettings(deployment *appsv1.Deployment, configMapAdmissionConfigs *corev1.ConfigMap, secretAdmissionKubeconfigs *corev1.Secret, values Values) {
	if len(values.EnabledAdmissionPlugins) > 0 {
		deployment.Spec.Template.Spec.Containers[0].Args = append(deployment.Spec.Template.Spec.Containers[0].Args, "--enable-admission-plugins="+strings.Join(admissionPluginNames(values), ","))
	}
	if len(values.DisabledAdmissionPlugins) > 0 {
		deployment.Spec.Template.Spec.Containers[0].Args = append(deployment.Spec.Template.Spec.Containers[0].Args, "--disable-admission-plugins="+strings.Join(disabledAdmissionPluginNames(values), ","))
	}
	deployment.Spec.Template.Spec.Containers[0].Args = append(deployment.Spec.Template.Spec.Containers[0].Args, fmt.Sprintf("--admission-control-config-file=%s/%s", volumeMountPathAdmissionConfiguration, configMapAdmissionDataKey))

	deployment.Spec.Template.Spec.Containers[0].VolumeMounts = append(deployment.Spec.Template.Spec.Containers[0].VolumeMounts,
		corev1.VolumeMount{
			Name:      volumeNameAdmissionConfiguration,
			MountPath: volumeMountPathAdmissionConfiguration,
		},
		corev1.VolumeMount{
			Name:      volumeNameAdmissionKubeconfigSecrets,
			MountPath: volumeMountPathAdmissionKubeconfigSecrets,
		},
	)

	deployment.Spec.Template.Spec.Volumes = append(deployment.Spec.Template.Spec.Volumes,
		corev1.Volume{
			Name: volumeNameAdmissionConfiguration,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: configMapAdmissionConfigs.Name,
					},
				},
			},
		},
		corev1.Volume{
			Name: volumeNameAdmissionKubeconfigSecrets,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: secretAdmissionKubeconfigs.Name,
				},
			},
		},
	)
}

func admissionPluginNames(values Values) []string {
	var out []string

	for _, plugin := range values.EnabledAdmissionPlugins {
		out = append(out, plugin.Name)
	}

	return out
}

func disabledAdmissionPluginNames(values Values) []string {
	var out []string

	for _, plugin := range values.DisabledAdmissionPlugins {
		out = append(out, plugin.Name)
	}

	return out
}
