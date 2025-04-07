// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package apiserver

import (
	"context"
	"fmt"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	apiserverv1beta1 "k8s.io/apiserver/pkg/apis/apiserver/v1beta1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	versionutils "github.com/gardener/gardener/pkg/utils/version"
)

const (
	volumeNameStructuredAuthorizationConfig             = "authorization-config"
	volumeNameStructuredAuthorizationWebhookKubeconfigs = "authorization-kubeconfigs"

	volumeMountPathStructuredAuthorizationConfig             = "/etc/kubernetes/structured/authorization"
	volumeMountPathStructuredAuthorizationWebhookKubeconfigs = "/etc/kubernetes/structured/authorization-kubeconfigs" // #nosec G101 -- No credential.

	// DataKeyConfigMapAuthorizationConfig is the key of the ConfigMap containing the authorization configuration.
	DataKeyConfigMapAuthorizationConfig = "config.yaml"
)

func (k *kubeAPIServer) useStructuredAuthorization() bool {
	value, ok := k.values.FeatureGates["StructuredAuthorizationConfiguration"]
	return (!ok || value) && !versionutils.ConstraintK8sLess130.Check(k.values.Version)
}

func (k *kubeAPIServer) reconcileConfigMapAuthorizationConfig(ctx context.Context, configMap *corev1.ConfigMap) error {
	if !k.useStructuredAuthorization() {
		return nil
	}

	authorizationConfiguration := &apiserverv1beta1.AuthorizationConfiguration{
		TypeMeta: metav1.TypeMeta{
			APIVersion: apiserverv1beta1.ConfigSchemeGroupVersion.String(),
			Kind:       "AuthorizationConfiguration",
		},
		Authorizers: []apiserverv1beta1.AuthorizerConfiguration{{Type: "RBAC", Name: "rbac"}},
	}

	if !k.values.IsWorkerless {
		authorizationConfiguration.Authorizers = append([]apiserverv1beta1.AuthorizerConfiguration{{Type: "Node", Name: "node"}}, authorizationConfiguration.Authorizers...)
	}

	for _, webhook := range k.values.AuthorizationWebhooks {
		config := apiserverv1beta1.AuthorizerConfiguration{
			Type:    "Webhook",
			Name:    webhook.Name,
			Webhook: &webhook.WebhookConfiguration,
		}

		config.Webhook.ConnectionInfo = apiserverv1beta1.WebhookConnectionInfo{
			Type:           apiserverv1beta1.AuthorizationWebhookConnectionInfoTypeKubeConfigFile,
			KubeConfigFile: ptr.To(volumeMountPathStructuredAuthorizationWebhookKubeconfigs + "/" + authorizationWebhookKubeconfigFilename(webhook.Name)),
		}

		authorizationConfiguration.Authorizers = append(authorizationConfiguration.Authorizers, config)
	}

	data, err := runtime.Encode(ConfigCodec, authorizationConfiguration)
	if err != nil {
		return fmt.Errorf("unable to encode authorization configuration: %w", err)
	}

	configMap.Data = map[string]string{DataKeyConfigMapAuthorizationConfig: string(data)}
	utilruntime.Must(kubernetesutils.MakeUnique(configMap))
	return client.IgnoreAlreadyExists(k.client.Client().Create(ctx, configMap))
}

func (k *kubeAPIServer) handleAuthorizationSettings(deployment *appsv1.Deployment, configMapAuthorizationConfig *corev1.ConfigMap, secretWebhooksKubeconfigs *corev1.Secret) {
	if !k.useStructuredAuthorization() {
		// TODO: Delete this branch and everything related to it once we only support a Kubernetes version which
		//  promotes the StructuredAuthorizationConfiguration feature gate to GA (1.32+ or higher).
		authModes := []string{"RBAC"}

		if !k.values.IsWorkerless {
			authModes = append([]string{"Node"}, authModes...)
		}

		if len(k.values.AuthorizationWebhooks) > 0 {
			authModes = append(authModes, "Webhook")

			webhook := k.values.AuthorizationWebhooks[0] // only one authz webhook supported in this case

			deployment.Spec.Template.Spec.Containers[0].Args = append(deployment.Spec.Template.Spec.Containers[0].Args, fmt.Sprintf("--authorization-webhook-config-file=%s/%s", volumeMountPathStructuredAuthorizationWebhookKubeconfigs, authorizationWebhookKubeconfigFilename(webhook.Name)))
			deployment.Spec.Template.Spec.Containers[0].Args = append(deployment.Spec.Template.Spec.Containers[0].Args, fmt.Sprintf("--authorization-webhook-cache-authorized-ttl=%s", webhook.AuthorizedTTL.Duration.String()))
			deployment.Spec.Template.Spec.Containers[0].Args = append(deployment.Spec.Template.Spec.Containers[0].Args, fmt.Sprintf("--authorization-webhook-cache-unauthorized-ttl=%s", webhook.UnauthorizedTTL.Duration.String()))
			if v := webhook.SubjectAccessReviewVersion; v != "" {
				deployment.Spec.Template.Spec.Containers[0].Args = append(deployment.Spec.Template.Spec.Containers[0].Args, fmt.Sprintf("--authorization-webhook-version=%s", v))
			}
		}

		deployment.Spec.Template.Spec.Containers[0].Args = append(deployment.Spec.Template.Spec.Containers[0].Args, "--authorization-mode="+strings.Join(authModes, ","))
	} else {
		deployment.Spec.Template.Spec.Containers[0].Args = append(deployment.Spec.Template.Spec.Containers[0].Args, fmt.Sprintf("--authorization-config=%s/%s", volumeMountPathStructuredAuthorizationConfig, DataKeyConfigMapAuthorizationConfig))
		deployment.Spec.Template.Spec.Containers[0].VolumeMounts = append(deployment.Spec.Template.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
			Name:      volumeNameStructuredAuthorizationConfig,
			MountPath: volumeMountPathStructuredAuthorizationConfig,
			ReadOnly:  true,
		})
		deployment.Spec.Template.Spec.Volumes = append(deployment.Spec.Template.Spec.Volumes, corev1.Volume{
			Name: volumeNameStructuredAuthorizationConfig,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{
					Name: configMapAuthorizationConfig.Name,
				}},
			},
		})
	}

	if len(k.values.AuthorizationWebhooks) > 0 {
		deployment.Spec.Template.Spec.Containers[0].VolumeMounts = append(deployment.Spec.Template.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
			Name:      volumeNameStructuredAuthorizationWebhookKubeconfigs,
			MountPath: volumeMountPathStructuredAuthorizationWebhookKubeconfigs,
			ReadOnly:  true,
		})
		deployment.Spec.Template.Spec.Volumes = append(deployment.Spec.Template.Spec.Volumes, corev1.Volume{
			Name: volumeNameStructuredAuthorizationWebhookKubeconfigs,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: secretWebhooksKubeconfigs.Name,
				},
			},
		})
	}
}
