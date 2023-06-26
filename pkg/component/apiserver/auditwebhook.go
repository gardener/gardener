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
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

const (
	// SecretWebhookKubeconfigDataKey is a constant for a key in the data of the secret containing a kubeconfig.
	SecretWebhookKubeconfigDataKey = "kubeconfig.yaml"
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
