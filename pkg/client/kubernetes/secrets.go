// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package kubernetes

import (
	"sort"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CreateSecret creates a new Secret object.
func (c *Clientset) CreateSecret(namespace, name string, secretType corev1.SecretType, data map[string][]byte, updateIfExists bool) (*corev1.Secret, error) {
	secret, err := c.kubernetes.CoreV1().Secrets(namespace).Create(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Type: secretType,
		Data: data,
	})
	if err != nil && apierrors.IsAlreadyExists(err) && updateIfExists {
		return c.UpdateSecret(namespace, name, secretType, data)
	}
	return secret, err
}

// CreateSecretObject creates a new Secret object.
func (c *Clientset) CreateSecretObject(secret *corev1.Secret, updateIfExists bool) (*corev1.Secret, error) {
	updatedSecret, err := c.kubernetes.CoreV1().Secrets(secret.Namespace).Create(secret)
	if err != nil && apierrors.IsAlreadyExists(err) && updateIfExists {
		return c.UpdateSecretObject(secret)
	}
	return updatedSecret, err
}

// UpdateSecret updates an already existing Secret object.
func (c *Clientset) UpdateSecret(namespace, name string, secretType corev1.SecretType, data map[string][]byte) (*corev1.Secret, error) {
	return c.kubernetes.CoreV1().Secrets(namespace).Update(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Type: secretType,
		Data: data,
	})
}

// UpdateSecretObject updates an already existing Secret object.
func (c *Clientset) UpdateSecretObject(secret *corev1.Secret) (*corev1.Secret, error) {
	return c.kubernetes.CoreV1().Secrets(secret.Namespace).Update(secret)
}

// ListSecrets lists all Secrets in a given <namespace>.
func (c *Clientset) ListSecrets(namespace string, listOptions metav1.ListOptions) (*corev1.SecretList, error) {
	secrets, err := c.kubernetes.CoreV1().Secrets(namespace).List(listOptions)
	if err != nil {
		return nil, err
	}
	sort.Slice(secrets.Items, func(i, j int) bool {
		return secrets.Items[i].ObjectMeta.CreationTimestamp.Before(&secrets.Items[j].ObjectMeta.CreationTimestamp)
	})
	return secrets, nil
}

// GetSecret returns a Secret object.
func (c *Clientset) GetSecret(namespace, name string) (*corev1.Secret, error) {
	return c.kubernetes.CoreV1().Secrets(namespace).Get(name, metav1.GetOptions{})
}

// DeleteSecret deletes an already existing Secret object.
func (c *Clientset) DeleteSecret(namespace, name string) error {
	return c.kubernetes.CoreV1().Secrets(namespace).Delete(name, &defaultDeleteOptions)
}
