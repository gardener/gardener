// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package backupentry

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	predicateutils "github.com/gardener/gardener/pkg/controllerutils/predicate"
)

// SecretToBackupEntryMapper returns a mapper that returns requests for BackupEntry whose
// referenced secrets have been modified.
func SecretToBackupEntryMapper(reader client.Reader, predicates []predicate.Predicate) handler.MapFunc {
	return func(ctx context.Context, obj client.Object) []reconcile.Request {
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		backupEntryList := &extensionsv1alpha1.BackupEntryList{}
		if err := reader.List(ctx, backupEntryList); err != nil {
			return nil
		}

		var requests []reconcile.Request
		for _, backupEntry := range backupEntryList.Items {
			if backupEntry.Spec.SecretRef.Name == obj.GetName() && backupEntry.Spec.SecretRef.Namespace == obj.GetNamespace() {
				if predicateutils.EvalGeneric(&backupEntry, predicates...) {
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
}

// NamespaceToBackupEntryMapper returns a mapper that returns requests for BackupEntry whose
// associated Shoot's seed namespace have been modified.
func NamespaceToBackupEntryMapper(reader client.Reader, predicates []predicate.Predicate) handler.MapFunc {
	return func(ctx context.Context, obj client.Object) []reconcile.Request {
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
			if !predicateutils.EvalGeneric(&backupEntry, predicates...) {
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
}
