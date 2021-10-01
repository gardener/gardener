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

package mapper

import (
	"context"

	extensionshandler "github.com/gardener/gardener/extensions/pkg/handler"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	grmpredicate "github.com/gardener/gardener/pkg/resourcemanager/predicate"
	contextutils "github.com/gardener/gardener/pkg/utils/context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type secretToManagedResourceMapper struct {
	client     client.Client
	ctx        context.Context
	predicates []predicate.Predicate
}

func (m *secretToManagedResourceMapper) InjectClient(client client.Client) error {
	m.client = client
	return nil
}

func (m *secretToManagedResourceMapper) InjectStopChannel(stopCh <-chan struct{}) error {
	m.ctx = contextutils.FromStopChannel(stopCh)
	return nil
}

func (m *secretToManagedResourceMapper) Map(obj client.Object) []reconcile.Request {
	if obj == nil {
		return nil
	}

	secret, ok := obj.(*corev1.Secret)
	if !ok {
		return nil
	}

	managedResourceList := &resourcesv1alpha1.ManagedResourceList{}
	if err := m.client.List(m.ctx, managedResourceList, client.InNamespace(secret.Namespace)); err != nil {
		return nil
	}

	var requests []reconcile.Request
	for _, mr := range managedResourceList.Items {
		if !grmpredicate.EvalGenericPredicate(&mr, m.predicates...) {
			continue
		}

		for _, secretRef := range mr.Spec.SecretRefs {
			if secretRef.Name == secret.Name {
				requests = append(requests, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Namespace: mr.Namespace,
						Name:      mr.Name,
					},
				})
			}
		}
	}
	return requests
}

// SecretToManagedResourceMapper returns a mapper that returns requests for ManagedResources whose
// referenced secrets have been modified.
func SecretToManagedResourceMapper(predicates ...predicate.Predicate) extensionshandler.Mapper {
	return &secretToManagedResourceMapper{predicates: predicates}
}
