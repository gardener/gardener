// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package backupentry

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils/mapper"
	predicateutils "github.com/gardener/gardener/pkg/controllerutils/predicate"
)

type secretToBackupEntryMapper struct {
	predicates []predicate.Predicate
}

func (m *secretToBackupEntryMapper) Map(ctx context.Context, _ logr.Logger, reader client.Reader, obj client.Object) []reconcile.Request {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	backupEntryList := &extensionsv1alpha1.BackupEntryList{}
	if err := reader.List(ctx, backupEntryList); err != nil {
		return nil
	}

	var requests []reconcile.Request
	for _, backupEntry := range backupEntryList.Items {
		if backupEntry.Spec.SecretRef.Name == obj.GetName() && backupEntry.Spec.SecretRef.Namespace == obj.GetNamespace() {
			if predicateutils.EvalGeneric(&backupEntry, m.predicates...) {
				requests = append(requests, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name: backupEntry.Name,
					},
				})
			}
		}
	}

	return requests
}

// SecretToBackupEntryMapper returns a mapper that returns requests for BackupEntry whose
// referenced secrets have been modified.
func SecretToBackupEntryMapper(predicates []predicate.Predicate) mapper.Mapper {
	return &secretToBackupEntryMapper{predicates: predicates}
}

type namespaceToBackupEntryMapper struct {
	predicates []predicate.Predicate
}

func (m *namespaceToBackupEntryMapper) Map(ctx context.Context, _ logr.Logger, reader client.Reader, obj client.Object) []reconcile.Request {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	namespace, ok := obj.(*corev1.Namespace)
	if !ok {
		return nil
	}

	backupEntryList := &extensionsv1alpha1.BackupEntryList{}
	if err := reader.List(ctx, backupEntryList); err != nil {
		return nil
	}

	shootUID := namespace.Annotations[v1beta1constants.ShootUID]

	var requests []reconcile.Request
	for _, backupEntry := range backupEntryList.Items {
		if !predicateutils.EvalGeneric(&backupEntry, m.predicates...) {
			continue
		}

		expectedTechnicalID, expectedUID := ExtractShootDetailsFromBackupEntryName(backupEntry.Name)
		if namespace.Name == expectedTechnicalID && shootUID == expectedUID {
			requests = append(requests, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name: backupEntry.Name,
				},
			})
		}
	}
	return requests
}

// NamespaceToBackupEntryMapper returns a mapper that returns requests for BackupEntry whose
// associated Shoot's seed namespace have been modified.
func NamespaceToBackupEntryMapper(predicates []predicate.Predicate) mapper.Mapper {
	return &namespaceToBackupEntryMapper{predicates: predicates}
}
