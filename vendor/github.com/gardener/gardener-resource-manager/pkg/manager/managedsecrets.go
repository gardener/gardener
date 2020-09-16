// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package manager

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func (s *Secret) WithLabels(labels map[string]string) *Secret {
	s.secret.Labels = labels
	return s
}

func (s *Secret) WithAnnotations(annotations map[string]string) *Secret {
	s.secret.Annotations = annotations
	return s
}

func (s *Secret) WithKeyValues(keyValues map[string][]byte) *Secret {
	s.secret.Data = keyValues
	return s
}

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
