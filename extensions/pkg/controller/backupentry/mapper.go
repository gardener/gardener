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

package backupentry

import (
	"context"
	"time"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils/mapper"
	predicateutils "github.com/gardener/gardener/pkg/controllerutils/predicate"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type secretToBackupEntryMapper struct {
	predicates []predicate.Predicate
}

func (m *secretToBackupEntryMapper) Map(ctx context.Context, _ logr.Logger, reader client.Reader, obj client.Object) []reconcile.Request {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	secret, ok := obj.(*corev1.Secret)
	if !ok {
		return nil
	}

	backupEntryList := &extensionsv1alpha1.BackupEntryList{}
	if err := reader.List(ctx, backupEntryList); err != nil {
		return nil
	}

	var requests []reconcile.Request
	for _, backupEntry := range backupEntryList.Items {
		if backupEntry.Spec.SecretRef.Name == secret.Name && backupEntry.Spec.SecretRef.Namespace == secret.Namespace {
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
