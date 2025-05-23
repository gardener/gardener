// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package builder

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	"github.com/gardener/gardener/pkg/utils"
)

// ManagedResource is a structure managing a ManagedResource.
type ManagedResource struct {
	client            client.Client
	createIfNotExists bool
	resource          *resourcesv1alpha1.ManagedResource

	labels, annotations map[string]string
}

// NewManagedResource creates a new builder for a ManagedResource.
func NewManagedResource(client client.Client) *ManagedResource {
	return &ManagedResource{
		client:            client,
		createIfNotExists: true,
		resource:          &resourcesv1alpha1.ManagedResource{},
	}
}

// CreateIfNotExists determines if the managed resources should be created if it does not exist.
func (m *ManagedResource) CreateIfNotExists(createIfNotExists bool) *ManagedResource {
	m.createIfNotExists = createIfNotExists
	return m
}

// WithNamespacedName sets the namespace and name.
func (m *ManagedResource) WithNamespacedName(namespace, name string) *ManagedResource {
	m.resource.Namespace = namespace
	m.resource.Name = name
	return m
}

// WithLabels sets the labels.
func (m *ManagedResource) WithLabels(labels map[string]string) *ManagedResource {
	m.labels = utils.MergeStringMaps(m.labels, labels)
	return m
}

// WithAnnotations sets the annotations.
func (m *ManagedResource) WithAnnotations(annotations map[string]string) *ManagedResource {
	m.annotations = annotations
	return m
}

// WithClass sets the Class field.
func (m *ManagedResource) WithClass(name string) *ManagedResource {
	if name == "" {
		m.resource.Spec.Class = nil
	} else {
		m.resource.Spec.Class = &name
	}
	return m
}

// WithSecretRef adds a reference with the given name to the SecretRefs field.
func (m *ManagedResource) WithSecretRef(secretRefName string) *ManagedResource {
	m.resource.Spec.SecretRefs = append(m.resource.Spec.SecretRefs, corev1.LocalObjectReference{Name: secretRefName})
	return m
}

// WithSecretRefs sets the SecretRefs field.
func (m *ManagedResource) WithSecretRefs(secretRefs []corev1.LocalObjectReference) *ManagedResource {
	m.resource.Spec.SecretRefs = append(m.resource.Spec.SecretRefs, secretRefs...)
	return m
}

// WithInjectedLabels sets the InjectLabels field.
func (m *ManagedResource) WithInjectedLabels(labelsToInject map[string]string) *ManagedResource {
	m.resource.Spec.InjectLabels = labelsToInject
	return m
}

// ForceOverwriteAnnotations sets the ForceOverwriteAnnotations field.
func (m *ManagedResource) ForceOverwriteAnnotations(v bool) *ManagedResource {
	m.resource.Spec.ForceOverwriteAnnotations = &v
	return m
}

// ForceOverwriteLabels sets the ForceOverwriteLabels field.
func (m *ManagedResource) ForceOverwriteLabels(v bool) *ManagedResource {
	m.resource.Spec.ForceOverwriteLabels = &v
	return m
}

// KeepObjects sets the KeepObjects field.
func (m *ManagedResource) KeepObjects(v bool) *ManagedResource {
	m.resource.Spec.KeepObjects = &v
	return m
}

// DeletePersistentVolumeClaims sets the DeletePersistentVolumeClaims field.
func (m *ManagedResource) DeletePersistentVolumeClaims(v bool) *ManagedResource {
	m.resource.Spec.DeletePersistentVolumeClaims = &v
	return m
}

// Reconcile creates or updates the ManagedResource as well as marks all referenced secrets as garbage collectable.
func (m *ManagedResource) Reconcile(ctx context.Context) error {
	resource := &resourcesv1alpha1.ManagedResource{
		ObjectMeta: metav1.ObjectMeta{Name: m.resource.Name, Namespace: m.resource.Namespace},
	}

	mutateFn := func(obj *resourcesv1alpha1.ManagedResource) {
		for k, v := range m.labels {
			metav1.SetMetaDataLabel(&obj.ObjectMeta, k, v)
		}

		for k, v := range m.annotations {
			metav1.SetMetaDataAnnotation(&obj.ObjectMeta, k, v)
		}

		obj.Spec = m.resource.Spec

		// the annotations should be injected after the spec is updated!
		utilruntime.Must(references.InjectAnnotations(obj))
	}

	if err := m.client.Get(ctx, client.ObjectKeyFromObject(resource), resource); err != nil {
		if apierrors.IsNotFound(err) && m.createIfNotExists {
			// if the mr is not found just create it
			mutateFn(resource)

			return m.client.Create(ctx, resource)
		}
		return err
	}

	// Always mark all old secrets as garbage collectable.
	// This is done in order to guarantee backwards compatibility with previous versions of this library
	// when the underlying mananaged resource secrets were not immutable and not garbage collectable.
	// This guarantees that "old" secrets are always taken care of.
	// If an old secret is already deleted then we do not care about it and continue the flow.
	// For more details, please see https://github.com/gardener/gardener/pull/8116
	oldSecrets := secretsFromRefs(resource)
	if err := markSecretsAsGarbageCollectable(ctx, m.client, oldSecrets); err != nil {
		return fmt.Errorf("marking old secrets as garbage collectable: %w", err)
	}

	// Update the managed resource if necessary
	existing := resource.DeepCopyObject()
	mutateFn(resource)
	if equality.Semantic.DeepEqual(existing, resource) {
		return nil
	}

	return m.client.Update(ctx, resource)
}

// Delete deletes the ManagedResource.
func (m *ManagedResource) Delete(ctx context.Context) error {
	return client.IgnoreNotFound(m.client.Delete(ctx, m.resource))
}

func markSecretsAsGarbageCollectable(ctx context.Context, c client.Client, secrets []*corev1.Secret) error {
	for _, secret := range secrets {
		if err := c.Get(ctx, client.ObjectKeyFromObject(secret), secret); err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return err
		}

		// if the GC label is already set then skip sending an empty patch
		if secret.Labels[references.LabelKeyGarbageCollectable] == references.LabelValueGarbageCollectable {
			continue
		}

		patch := client.StrategicMergeFrom(secret.DeepCopy())
		metav1.SetMetaDataLabel(&secret.ObjectMeta, references.LabelKeyGarbageCollectable, references.LabelValueGarbageCollectable)
		if err := c.Patch(ctx, secret, patch); err != nil {
			return err
		}
	}
	return nil
}

func secretsFromRefs(obj *resourcesv1alpha1.ManagedResource) []*corev1.Secret {
	secrets := make([]*corev1.Secret, 0, len(obj.Spec.SecretRefs))
	for _, secretRef := range obj.Spec.SecretRefs {
		secrets = append(secrets, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretRef.Name,
				Namespace: obj.Namespace,
			},
		})
	}
	return secrets
}
