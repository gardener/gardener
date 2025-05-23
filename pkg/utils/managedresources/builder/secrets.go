// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package builder

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	"github.com/gardener/gardener/pkg/utils"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// Secret is a structure for managing a secret.
type Secret struct {
	client            client.Client
	createIfNotExists bool

	keyValues map[string]string
	secret    *corev1.Secret
}

// NewSecret creates a new builder for a secret.
func NewSecret(client client.Client) *Secret {
	return &Secret{
		client:            client,
		createIfNotExists: true,
		keyValues:         make(map[string]string),
		secret:            &corev1.Secret{},
	}
}

// CreateIfNotExists determines if the secret should be created if it does not exist.
// Immutable secrets are always created, regardless of this configuration.
func (s *Secret) CreateIfNotExists(createIfNotExists bool) *Secret {
	s.createIfNotExists = createIfNotExists
	return s
}

// WithNamespacedName sets the namespace and name.
func (s *Secret) WithNamespacedName(namespace, name string) *Secret {
	s.secret.Namespace = namespace
	s.secret.Name = name
	return s
}

// WithLabels sets the labels. The label "resources.gardener.cloud/garbage-collectable-reference" is retained
// if it already exists in the current labels.
func (s *Secret) WithLabels(labels map[string]string) *Secret {
	if s.secret.Labels == nil {
		s.secret.Labels = utils.MergeStringMaps(labels)
		return s
	}
	_, ok := s.secret.Labels[references.LabelKeyGarbageCollectable]
	if ok && ptr.Deref(s.secret.Immutable, false) {
		s.secret.Labels = map[string]string{
			references.LabelKeyGarbageCollectable: references.LabelValueGarbageCollectable,
		}
	}
	s.secret.Labels = utils.MergeStringMaps(labels, s.secret.Labels)
	return s
}

// AddLabels adds the labels to the existing secret labels.
func (s *Secret) AddLabels(labels map[string]string) *Secret {
	if s.secret.Labels == nil {
		s.secret.Labels = make(map[string]string, len(labels))
	}
	for k, v := range labels {
		s.secret.Labels[k] = v
	}
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

// Unique makes the secret unique and immutable. Returns the new and unique name of the secret and the builder object.
// This function should be called after the name and data of the secret were set.
func (s *Secret) Unique() (string, *Secret) {
	utilruntime.Must(kubernetesutils.MakeUnique(s.secret))
	return s.secret.Name, s
}

// Reconcile creates or updates the secret.
func (s *Secret) Reconcile(ctx context.Context) error {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: s.secret.Name, Namespace: s.secret.Namespace},
	}

	mutate := func() error {
		secret.Labels = s.secret.Labels
		secret.Annotations = s.secret.Annotations
		secret.Type = corev1.SecretTypeOpaque
		secret.Data = s.secret.Data
		secret.Immutable = s.secret.Immutable
		return nil
	}

	if s.createIfNotExists || ptr.Deref(s.secret.Immutable, false) {
		_, err := controllerutil.CreateOrUpdate(ctx, s.client, secret, mutate)
		return err
	}
	return updateSecret(ctx, s.client, secret, mutate)
}

func updateSecret(ctx context.Context, cl client.Client, secret *corev1.Secret, mutate func() error) error {
	if err := cl.Get(ctx, client.ObjectKeyFromObject(secret), secret); err != nil {
		return err
	}

	existingSecret := secret.DeepCopy()
	if err := mutate(); err != nil {
		return err
	}

	if equality.Semantic.DeepEqual(existingSecret, secret) {
		return nil
	}

	return cl.Update(ctx, secret)
}
