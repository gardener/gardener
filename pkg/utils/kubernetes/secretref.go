// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package kubernetes

import (
	"context"
	"fmt"

	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
)

// GetSecretByReference returns the secret referenced by the given secret reference.
func GetSecretByReference(ctx context.Context, c client.Reader, ref *corev1.SecretReference) (*corev1.Secret, error) {
	secret := &corev1.Secret{}
	if err := c.Get(ctx, client.ObjectKey{Namespace: ref.Namespace, Name: ref.Name}, secret); err != nil {
		return nil, err
	}
	return secret, nil
}

// GetSecretByObjectReference returns the secret referenced by the given object reference.
func GetSecretByObjectReference(ctx context.Context, c client.Reader, ref *corev1.ObjectReference) (*corev1.Secret, error) {
	if ref == nil {
		return nil, fmt.Errorf("ref is nil")
	}
	if ref.APIVersion != "v1" || ref.Kind != "Secret" {
		return nil, fmt.Errorf("objectRef does not refer to secret")
	}
	secret := &corev1.Secret{}
	if err := c.Get(ctx, client.ObjectKey{Namespace: ref.Namespace, Name: ref.Name}, secret); err != nil {
		return nil, err
	}
	return secret, nil
}

// GetSecretMetadataByReference returns the secret referenced by the given secret reference.
func GetSecretMetadataByReference(ctx context.Context, c client.Reader, ref *corev1.SecretReference) (*metav1.PartialObjectMetadata, error) {
	metadata := &metav1.PartialObjectMetadata{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		}}
	if err := c.Get(ctx, client.ObjectKey{Namespace: ref.Namespace, Name: ref.Name}, metadata); err != nil {
		return nil, err
	}
	return metadata, nil
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

// DeleteSecretByObjectReference deletes the secret referenced by the given object reference.
func DeleteSecretByObjectReference(ctx context.Context, c client.Client, ref *corev1.ObjectReference) error {
	if ref == nil {
		return fmt.Errorf("ref is nil")
	}
	if ref.APIVersion != "v1" || ref.Kind != "Secret" {
		return fmt.Errorf("objectRef does not refer to secret")
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ref.Name,
			Namespace: ref.Namespace,
		},
	}
	return client.IgnoreNotFound(c.Delete(ctx, secret))
}

// GetCredentialsByObjectReference returns the credentials, being Secret or WorkloadIdentity, referenced by the given object reference.
func GetCredentialsByObjectReference(ctx context.Context, c client.Reader, ref corev1.ObjectReference) (client.Object, error) {
	var obj client.Object
	switch ref.GroupVersionKind() {
	case corev1.SchemeGroupVersion.WithKind("Secret"):
		obj = &corev1.Secret{}
	case securityv1alpha1.SchemeGroupVersion.WithKind("WorkloadIdentity"):
		obj = &securityv1alpha1.WorkloadIdentity{}
	default:
		return nil, fmt.Errorf("unsupported credentials reference: %s, %s", ref.Namespace+"/"+ref.Name, ref.GroupVersionKind().String())
	}

	if err := c.Get(ctx, client.ObjectKey{Namespace: ref.Namespace, Name: ref.Name}, obj); err != nil {
		return nil, err
	}
	return obj, nil
}

// GetCredentialsByCrossVersionObjectReference returns the credentials, being Secret or WorkloadIdentity, referenced by the [autoscalingv1.CrossVersionObjectReference] parameter.
func GetCredentialsByCrossVersionObjectReference(ctx context.Context, c client.Reader, ref autoscalingv1.CrossVersionObjectReference, namespace string) (client.Object, error) {
	return GetCredentialsByObjectReference(ctx, c, corev1.ObjectReference{
		APIVersion: ref.APIVersion,
		Kind:       ref.Kind,
		Name:       ref.Name,
		Namespace:  namespace,
	})
}
