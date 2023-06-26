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
	SecretWebhookKubeconfigDataKey = "kubeconfig.yaml"
	// ConfigMapAuditPolicyDataKey is a constant for a key in the data of the ConfigMap containing an audit policy.
	ConfigMapAuditPolicyDataKey = "audit-policy.yaml"
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

	configMap.Data = map[string]string{ConfigMapAuditPolicyDataKey: policy}
	utilruntime.Must(kubernetesutils.MakeUnique(configMap))
	return client.IgnoreAlreadyExists(c.Create(ctx, configMap))
}
