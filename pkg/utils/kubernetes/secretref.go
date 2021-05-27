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

package kubernetes

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetSecretByReference returns the secret referenced by the given secret reference.
func GetSecretByReference(ctx context.Context, c client.Reader, ref *corev1.SecretReference) (*corev1.Secret, error) {
	secret := &corev1.Secret{}
	if err := c.Get(ctx, Key(ref.Namespace, ref.Name), secret); err != nil {
		return nil, err
	}
	return secret, nil
}

// DeleteSecretByReference deletes the secret referenced by the given secret reference.
func DeleteSecretByReference(ctx context.Context, c client.Client, ref *corev1.SecretReference) error {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ref.Name,
			Namespace: ref.Namespace,
		},
	}
	return client.IgnoreNotFound(c.Delete(ctx, secret))
}
