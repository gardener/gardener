// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"context"

	"github.com/gardener/gardener-resource-manager/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// ManagedResourceLabelKeyOrigin is a key for a label on a managed resource with the value 'origin'.
	ManagedResourceLabelKeyOrigin = "origin"
	// ManagedResourceLabelValueGardener is a value for a label on a managed resource with the value 'gardener'.
	ManagedResourceLabelValueGardener = "gardener"

	// ManagedResourceSecretPrefix is the prefix that is used for secrets referenced by managed resources.
	ManagedResourceSecretPrefix = "managedresource-"
)

// DeployManagedResource deploys a ManagedResource CR for the gardener-resource-manager.
func DeployManagedResource(ctx context.Context, c client.Client, name, namespace string, keepObjects bool, data map[string][]byte) error {
	var (
		secretName, secret = NewManagedResourceSecret(c, name, namespace, data)
		managedResource    = NewManagedResource(c, name, namespace, keepObjects)
	)

	if err := secret.Reconcile(ctx); err != nil {
		return err
	}
	return managedResource.WithSecretRef(secretName).Reconcile(ctx)
}

// NewManagedResourceSecret constructs a new Secret object containing manifests managed by the Gardener-Resource-Manager
// which can be reconciled.
func NewManagedResourceSecret(c client.Client, name, namespace string, data map[string][]byte) (string, *manager.Secret) {
	secretName := ManagedResourceSecretName(name)
	return secretName, manager.NewSecret(c).
		WithNamespacedName(namespace, secretName).
		WithKeyValues(data)
}

// NewManagedResource constructs a new ManagedResource object for the Gardener-Resource-Manager.
func NewManagedResource(c client.Client, name, namespace string, keepObjects bool) *manager.ManagedResource {
	var (
		injectedLabels = map[string]string{ShootNoCleanup: "true"}
		labels         = map[string]string{ManagedResourceLabelKeyOrigin: ManagedResourceLabelValueGardener}
	)

	return manager.NewManagedResource(c).
		WithNamespacedName(namespace, name).
		WithLabels(labels).
		WithInjectedLabels(injectedLabels).
		KeepObjects(keepObjects)
}

// ManagedResourceSecretName returns the name of a corev1.Scret for the given name of a
// resourcesv1alpha1.ManagedResource.
func ManagedResourceSecretName(managedResourceName string) string {
	return ManagedResourceSecretPrefix + managedResourceName
}
