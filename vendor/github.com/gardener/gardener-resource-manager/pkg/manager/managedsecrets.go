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

package manager

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type Secret struct {
	client client.Client

	keyValues map[string]string
	secret    *corev1.Secret
}

func NewSecret(client client.Client) *Secret {
	return &Secret{
		client:    client,
		keyValues: make(map[string]string),
		secret:    &corev1.Secret{},
	}
}

func (s *Secret) WithNamespacedName(namespace, name string) *Secret {
	s.secret.Namespace = namespace
	s.secret.Name = name
	return s
}

func (s *Secret) WithKeyValues(keyValues map[string][]byte) *Secret {
	s.secret.Data = keyValues
	return s
}

func (s *Secret) Reconcile(ctx context.Context) error {
	secret := &corev1.Secret{ObjectMeta: s.secret.ObjectMeta}

	_, err := controllerutil.CreateOrUpdate(ctx, s.client, secret, func() error {
		secret.Type = corev1.SecretTypeOpaque
		secret.Data = s.secret.Data
		return nil
	})
	return err
}

func (s *Secret) Delete(ctx context.Context) error {
	if err := s.client.Delete(ctx, s.secret); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	return nil
}

type Secrets struct {
	client client.Client

	secrets []Secret
}

func NewSecrets(client client.Client) *Secrets {
	return &Secrets{
		client:  client,
		secrets: []Secret{},
	}
}

func (s *Secrets) WithSecretList(secrets []Secret) *Secrets {
	s.secrets = append(s.secrets, secrets...)
	return s
}

func (s *Secrets) WithSecret(secrets Secret) *Secrets {
	s.secrets = append(s.secrets, secrets)
	return s
}

func (s *Secrets) Reconcile(ctx context.Context) error {
	for _, secret := range s.secrets {
		if err := secret.Reconcile(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (s *Secrets) Delete(ctx context.Context) error {
	for _, secret := range s.secrets {
		if err := secret.Delete(ctx); err != nil {
			return err
		}
	}
	return nil
}
