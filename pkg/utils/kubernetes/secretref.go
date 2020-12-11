// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// GetSecretByRef returns the secret referenced by the given secret reference.
func GetSecretByRef(ctx context.Context, c client.Client, secretRef corev1.SecretReference) (*corev1.Secret, error) {
	secret := &corev1.Secret{}
	if err := c.Get(ctx, Key(secretRef.Namespace, secretRef.Name), secret); err != nil {
		return nil, err
	}
	return secret, nil
}

// CreateOrUpdateSecretByRef creates or updates the secret referenced by the given secret reference
// with the given type, data, and owner references.
func CreateOrUpdateSecretByRef(ctx context.Context, c client.Client, secretRef corev1.SecretReference, typ corev1.SecretType, data map[string][]byte, ownerRefs []metav1.OwnerReference) error {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretRef.Name,
			Namespace: secretRef.Namespace,
		},
	}
	if _, err := controllerutil.CreateOrUpdate(ctx, c, secret, func() error {
		secret.ObjectMeta.OwnerReferences = ownerRefs
		secret.Type = typ
		secret.Data = data
		return nil
	}); err != nil {
		return err
	}
	return nil
}

// DeleteSecretByRef deletes the secret referenced by the given secret reference.
func DeleteSecretByRef(ctx context.Context, c client.Client, secretRef corev1.SecretReference) error {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretRef.Name,
			Namespace: secretRef.Namespace,
		},
	}
	if err := c.Delete(ctx, secret); client.IgnoreNotFound(err) != nil {
		return err
	}
	return nil
}
