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

	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
)

const (
	secretOIDCCABundleNamePrefix   = "kube-apiserver-oidc-cabundle"
	secretOIDCCABundleDataKeyCaCrt = "ca.crt"

	secretServiceAccountSigningKeyNamePrefix = "kube-apiserver-sa-signing-key"
	// SecretServiceAccountSigningKeyDataKeySigningKey is a constant for a key in the data map that contains the key
	// which is used to sign service accounts.
	SecretServiceAccountSigningKeyDataKeySigningKey = "signing-key"

	// SecretEtcdEncryptionConfigurationDataKey is a constant for a key in the data map that contains the config
	// which is used to encrypt etcd data.
	SecretEtcdEncryptionConfigurationDataKey = "encryption-configuration.yaml"
)

func (k *kubeAPIServer) emptySecret(name string) *corev1.Secret {
	return &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: k.namespace}}
}

func (k *kubeAPIServer) reconcileSecretOIDCCABundle(ctx context.Context, secret *corev1.Secret) error {
	if k.values.OIDC == nil || k.values.OIDC.CABundle == nil {
		// We don't delete the secret here as we don't know its name (as it's unique). Instead, we rely on the usual
		// garbage collection for unique secrets/configmaps.
		return nil
	}

	secret.Data = map[string][]byte{secretOIDCCABundleDataKeyCaCrt: []byte(*k.values.OIDC.CABundle)}
	utilruntime.Must(kutil.MakeUnique(secret))

	return kutil.IgnoreAlreadyExists(k.client.Client().Create(ctx, secret))
}

func (k *kubeAPIServer) reconcileSecretServiceAccountSigningKey(ctx context.Context, secret *corev1.Secret) error {
	if k.values.ServiceAccount.SigningKey == nil {
		// We don't delete the secret here as we don't know its name (as it's unique). Instead, we rely on the usual
		// garbage collection for unique secrets/configmaps.
		return nil
	}

	secret.Data = map[string][]byte{SecretServiceAccountSigningKeyDataKeySigningKey: k.values.ServiceAccount.SigningKey}
	utilruntime.Must(kutil.MakeUnique(secret))

	return kutil.IgnoreAlreadyExists(k.client.Client().Create(ctx, secret))
}
