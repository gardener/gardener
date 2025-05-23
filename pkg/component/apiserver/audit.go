// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package apiserver

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	auditv1 "k8s.io/apiserver/pkg/apis/audit/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

var auditCodec runtime.Codec

func init() {
	auditScheme := runtime.NewScheme()
	utilruntime.Must(auditv1.AddToScheme(auditScheme))

	var (
		ser = json.NewSerializerWithOptions(json.DefaultMetaFactory, auditScheme, auditScheme, json.SerializerOptions{
			Yaml:   true,
			Pretty: false,
			Strict: false,
		})
		versions = schema.GroupVersions([]schema.GroupVersion{
			auditv1.SchemeGroupVersion,
		})
	)

	auditCodec = serializer.NewCodecFactory(auditScheme).CodecForVersions(ser, ser, versions, versions)
}

const (
	// SecretWebhookKubeconfigDataKey is a constant for a key in the data of the secret containing a kubeconfig.
	SecretWebhookKubeconfigDataKey = "kubeconfig.yaml" // #nosec G101 -- No credential.

	configMapAuditPolicyDataKey = "audit-policy.yaml"

	volumeNameAuditPolicy                 = "audit-policy-config"
	volumeNameAuditWebhookKubeconfig      = "audit-webhook-kubeconfig"
	volumeMountPathAuditPolicy            = "/etc/kubernetes/audit"
	volumeMountPathAuditWebhookKubeconfig = "/etc/kubernetes/webhook/audit"
)

// ReconcileSecretAuditWebhookKubeconfig reconciles the secret containing the kubeconfig for audit webhooks.
func ReconcileSecretAuditWebhookKubeconfig(ctx context.Context, c client.Client, secret *corev1.Secret, auditConfig *AuditConfig) error {
	if auditConfig == nil || auditConfig.Webhook == nil || len(auditConfig.Webhook.Kubeconfig) == 0 {
		// We don't delete the secret here as we don't know its name (as it's unique). Instead, we rely on the usual
		// garbage collection for unique secrets/configmaps.
		return nil
	}

	return ReconcileSecretWebhookKubeconfig(ctx, c, secret, auditConfig.Webhook.Kubeconfig)
}

// ReconcileSecretWebhookKubeconfig reconciles the secret containing a kubeconfig for webhooks.
func ReconcileSecretWebhookKubeconfig(ctx context.Context, c client.Client, secret *corev1.Secret, kubeconfig []byte) error {
	secret.Data = map[string][]byte{SecretWebhookKubeconfigDataKey: kubeconfig}
	utilruntime.Must(kubernetesutils.MakeUnique(secret))
	return client.IgnoreAlreadyExists(c.Create(ctx, secret))
}

// ReconcileConfigMapAuditPolicy reconciles the ConfigMap containing the audit policy.
func ReconcileConfigMapAuditPolicy(ctx context.Context, c client.Client, configMap *corev1.ConfigMap, auditConfig *AuditConfig) error {
	defaultPolicy := &auditv1.Policy{
		Rules: []auditv1.PolicyRule{
			{Level: auditv1.LevelNone},
		},
	}

	data, err := runtime.Encode(auditCodec, defaultPolicy)
	if err != nil {
		return err
	}

	policy := string(data)
	if auditConfig != nil && auditConfig.Policy != nil {
		policy = *auditConfig.Policy
	}

	configMap.Data = map[string]string{configMapAuditPolicyDataKey: policy}
	utilruntime.Must(kubernetesutils.MakeUnique(configMap))
	return client.IgnoreAlreadyExists(c.Create(ctx, configMap))
}

// InjectAuditSettings injects the audit settings into `gardener-apiserver` and `kube-apiserver` deployments.
func InjectAuditSettings(deployment *appsv1.Deployment, configMapAuditPolicy *corev1.ConfigMap, secretWebhookKubeconfig *corev1.Secret, auditConfig *AuditConfig) {
	deployment.Spec.Template.Spec.Containers[0].Args = append(deployment.Spec.Template.Spec.Containers[0].Args, fmt.Sprintf("--audit-policy-file=%s/%s", volumeMountPathAuditPolicy, configMapAuditPolicyDataKey))

	deployment.Spec.Template.Spec.Containers[0].VolumeMounts = append(deployment.Spec.Template.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
		Name:      volumeNameAuditPolicy,
		MountPath: volumeMountPathAuditPolicy,
	})
	deployment.Spec.Template.Spec.Volumes = append(deployment.Spec.Template.Spec.Volumes, corev1.Volume{
		Name: volumeNameAuditPolicy,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: configMapAuditPolicy.Name,
				},
			},
		},
	})

	if auditConfig == nil || auditConfig.Webhook == nil {
		return
	}

	if len(auditConfig.Webhook.Kubeconfig) > 0 {
		deployment.Spec.Template.Spec.Containers[0].Args = append(deployment.Spec.Template.Spec.Containers[0].Args, fmt.Sprintf("--audit-webhook-config-file=%s/%s", volumeMountPathAuditWebhookKubeconfig, SecretWebhookKubeconfigDataKey))
		deployment.Spec.Template.Spec.Containers[0].VolumeMounts = append(deployment.Spec.Template.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
			Name:      volumeNameAuditWebhookKubeconfig,
			MountPath: volumeMountPathAuditWebhookKubeconfig,
			ReadOnly:  true,
		})
		deployment.Spec.Template.Spec.Volumes = append(deployment.Spec.Template.Spec.Volumes, corev1.Volume{
			Name: volumeNameAuditWebhookKubeconfig,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: secretWebhookKubeconfig.Name,
				},
			},
		})
	}

	if v := auditConfig.Webhook.BatchMaxSize; v != nil {
		deployment.Spec.Template.Spec.Containers[0].Args = append(deployment.Spec.Template.Spec.Containers[0].Args, fmt.Sprintf("--audit-webhook-batch-max-size=%d", *v))
	}

	if v := auditConfig.Webhook.Version; v != nil {
		deployment.Spec.Template.Spec.Containers[0].Args = append(deployment.Spec.Template.Spec.Containers[0].Args, "--audit-webhook-version="+*v)
	}
}
