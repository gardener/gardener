// Copyright 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package backupbucket

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils/mapper"
	predicateutils "github.com/gardener/gardener/pkg/controllerutils/predicate"
)

type secretToBackupBucketMapper struct {
	predicates []predicate.Predicate
}

func (m *secretToBackupBucketMapper) Map(ctx context.Context, _ logr.Logger, reader client.Reader, obj client.Object) []reconcile.Request {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	secret, ok := obj.(*corev1.Secret)
	if !ok {
		return nil
	}

	backupBucketList := &extensionsv1alpha1.BackupBucketList{}
	if err := reader.List(ctx, backupBucketList); err != nil {
		return nil
	}

	var requests []reconcile.Request
	for _, backupBucket := range backupBucketList.Items {
		if backupBucket.Spec.SecretRef.Name == secret.Name && backupBucket.Spec.SecretRef.Namespace == secret.Namespace {
			if predicateutils.EvalGeneric(&backupBucket, m.predicates...) {
				requests = append(requests, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name: backupBucket.Name,
					},
				})
			}
		}
	}
	return requests
}

// SecretToBackupBucketMapper returns a mapper that returns requests for BackupBucket whose
// referenced secrets have been modified.
func SecretToBackupBucketMapper(predicates []predicate.Predicate) mapper.Mapper {
	return &secretToBackupBucketMapper{predicates: predicates}
}
