// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package apiserver

import (
	"context"
	"fmt"
	"strconv"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apiserver/pkg/apis/apiserver"
	apiserverv1beta1 "k8s.io/apiserver/pkg/apis/apiserver/v1beta1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

const (
	volumeNameStructuredAuthenticationConfig = "authentication-config"

	volumeMountPathStructuredAuthenticationConfig = "/etc/kubernetes/structured/authentication"

	// DataKeyConfigMapAuthenticationConfig is the key of the ConfigMap containing the authentication configuration.
	DataKeyConfigMapAuthenticationConfig = "config.yaml"
)

// reconcileConfigMapAuthenticationConfig reconciles the ConfigMap containing the authentication configuration.
func (k *kubeAPIServer) reconcileConfigMapAuthenticationConfig(ctx context.Context, configMap *corev1.ConfigMap) error {
	if !k.structuredAuthenticationFeatureGateEnabled() ||
		(k.values.AuthenticationConfiguration == nil && !k.anonymousAuthConfigurableEndpointsFeatureGateEnabled()) {
		return nil
	}

	authenticationConfig := ptr.Deref(k.values.AuthenticationConfiguration, "")

	authenticationConfigurationV1Beta1 := &apiserverv1beta1.AuthenticationConfiguration{
		TypeMeta: metav1.TypeMeta{
			APIVersion: apiserverv1beta1.ConfigSchemeGroupVersion.String(),
			Kind:       "AuthenticationConfiguration",
		},
	}

	if len(authenticationConfig) > 0 {
		authenticationConfiguration := &apiserver.AuthenticationConfiguration{}
		if err := runtime.DecodeInto(ConfigCodec, []byte(authenticationConfig), authenticationConfiguration); err != nil {
			return err
		}
		if err := apiserverv1beta1.Convert_apiserver_AuthenticationConfiguration_To_v1beta1_AuthenticationConfiguration(authenticationConfiguration, authenticationConfigurationV1Beta1, nil); err != nil {
			return err
		}
	}

	if k.anonymousAuthConfigurableEndpointsFeatureGateEnabled() && authenticationConfigurationV1Beta1.Anonymous == nil {
		authenticationConfigurationV1Beta1.Anonymous = &apiserverv1beta1.AnonymousAuthConfig{
			Enabled: ptr.Deref(k.values.AnonymousAuthenticationEnabled, false),
		}
	}

	data, err := runtime.Encode(ConfigCodec, authenticationConfigurationV1Beta1)
	if err != nil {
		return fmt.Errorf("unable to encode authentication configuration: %w", err)
	}

	configMap.Data = map[string]string{DataKeyConfigMapAuthenticationConfig: string(data)}
	utilruntime.Must(kubernetesutils.MakeUnique(configMap))
	return client.IgnoreAlreadyExists(k.client.Client().Create(ctx, configMap))
}

func (k *kubeAPIServer) handleAuthenticationSettings(deployment *appsv1.Deployment, configMapAuthenticationConfig *corev1.ConfigMap) error {
	var structAuthConfiguresAnonymousAuthentication bool

	if config, ok := configMapAuthenticationConfig.Data[DataKeyConfigMapAuthenticationConfig]; ok {
		authenticationConfiguration := &apiserverv1beta1.AuthenticationConfiguration{
			TypeMeta: metav1.TypeMeta{
				APIVersion: apiserverv1beta1.ConfigSchemeGroupVersion.String(),
				Kind:       "AuthenticationConfiguration",
			},
		}

		if err := runtime.DecodeInto(ConfigCodec, []byte(config), authenticationConfiguration); err != nil {
			return err
		}

		structAuthConfiguresAnonymousAuthentication = authenticationConfiguration.Anonymous != nil
	}

	if !k.structuredAuthenticationFeatureGateEnabled() || !structAuthConfiguresAnonymousAuthentication {
		deployment.Spec.Template.Spec.Containers[0].Args = append(deployment.Spec.Template.Spec.Containers[0].Args, "--anonymous-auth="+strconv.FormatBool(ptr.Deref(k.values.AnonymousAuthenticationEnabled, false)))
	}

	if _, ok := configMapAuthenticationConfig.Data[DataKeyConfigMapAuthenticationConfig]; !ok {
		return nil
	}

	deployment.Spec.Template.Spec.Containers[0].Args = append(deployment.Spec.Template.Spec.Containers[0].Args, fmt.Sprintf("--authentication-config=%s/%s", volumeMountPathStructuredAuthenticationConfig, DataKeyConfigMapAuthenticationConfig))
	deployment.Spec.Template.Spec.Containers[0].VolumeMounts = append(deployment.Spec.Template.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
		Name:      volumeNameStructuredAuthenticationConfig,
		MountPath: volumeMountPathStructuredAuthenticationConfig,
	})
	deployment.Spec.Template.Spec.Volumes = append(deployment.Spec.Template.Spec.Volumes, corev1.Volume{
		Name: volumeNameStructuredAuthenticationConfig,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: configMapAuthenticationConfig.Name,
				},
			},
		},
	})

	return nil
}

func (k *kubeAPIServer) structuredAuthenticationFeatureGateEnabled() bool {
	featureGateEnabled, featureGateSet := k.values.FeatureGates["StructuredAuthenticationConfiguration"]
	if featureGateSet {
		return featureGateEnabled
	}

	return true
}

func (k *kubeAPIServer) anonymousAuthConfigurableEndpointsFeatureGateEnabled() bool {
	featureGateEnabled, featureGateSet := k.values.FeatureGates["AnonymousAuthConfigurableEndpoints"]
	if featureGateSet {
		return featureGateEnabled
	}

	return true
}
