// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package builder

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// Secret is a structure for managing a secret.
type Secret struct {
	client client.Client

	keyValues map[string]string
	secret    *corev1.Secret
}

// NewSecret creates a new builder for a secret.
func NewSecret(client client.Client) *Secret {
	return &Secret{
		client:    client,
		keyValues: make(map[string]string),
		secret:    &corev1.Secret{},
	}
}

// WithNamespacedName sets the namespace and name.
func (s *Secret) WithNamespacedName(namespace, name string) *Secret {
	s.secret.Namespace = namespace
	s.secret.Name = name
	return s
}

// WithLabels sets the labels.
func (s *Secret) WithLabels(labels map[string]string) *Secret {
	s.secret.Labels = labels
	return s
}

// WithAnnotations sets the annotations.
func (s *Secret) WithAnnotations(annotations map[string]string) *Secret {
	s.secret.Annotations = annotations
	return s
}

// WithKeyValues sets the data map.
func (s *Secret) WithKeyValues(keyValues map[string][]byte) *Secret {
	s.secret.Data = keyValues
	return s
}

// Reconcile creates or updates the secret.
func (s *Secret) Reconcile(ctx context.Context) error {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: s.secret.Name, Namespace: s.secret.Namespace},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, s.client, secret, func() error {
		secret.Labels = s.secret.Labels
		secret.Annotations = s.secret.Annotations
		secret.Type = corev1.SecretTypeOpaque
		secret.Data = s.secret.Data
		return nil
	})
	return err
}

// Delete deletes the secret.
func (s *Secret) Delete(ctx context.Context) error {
	return client.IgnoreNotFound(s.client.Delete(ctx, s.secret))
}

// Secrets is a structure for managing multiple secrets.
type Secrets struct {
	client client.Client

	secrets []Secret
}

// NewSecrets creates a Manager for multiple secrets.
func NewSecrets(client client.Client) *Secrets {
	return &Secrets{
		client:  client,
		secrets: []Secret{},
	}
}

// WithSecretList sets the secrets list.
func (s *Secrets) WithSecretList(secrets []Secret) *Secrets {
	s.secrets = append(s.secrets, secrets...)
	return s
}

// WithSecret adds the given secret to the secrets list.
func (s *Secrets) WithSecret(secrets Secret) *Secrets {
	s.secrets = append(s.secrets, secrets)
	return s
}

// Reconcile reconciles all secrets.
func (s *Secrets) Reconcile(ctx context.Context) error {
	for _, secret := range s.secrets {
		if err := secret.Reconcile(ctx); err != nil {
			return err
		}
	}
	return nil
}

// Delete deletes all secrets.
func (s *Secrets) Delete(ctx context.Context) error {
	for _, secret := range s.secrets {
		if err := secret.Delete(ctx); err != nil {
			return err
		}
	}
	return nil
}
