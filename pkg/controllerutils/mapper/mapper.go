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

	"sigs.k8s.io/controller-runtime/pkg/cache"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	predicateutils "github.com/gardener/gardener/pkg/controllerutils/predicate"
	contextutil "github.com/gardener/gardener/pkg/utils/context"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
)

type clusterToObjectMapper struct {
	ctx            context.Context
	reader         cache.Cache
	newObjListFunc func() client.ObjectList
	predicates     []predicate.Predicate
}

func (m *clusterToObjectMapper) InjectCache(c cache.Cache) error {
	m.reader = c
	return nil
}

func (m *clusterToObjectMapper) InjectStopChannel(stopCh <-chan struct{}) error {
	m.ctx = contextutil.FromStopChannel(stopCh)
	return nil
}

func (m *clusterToObjectMapper) InjectFunc(f inject.Func) error {
	for _, p := range m.predicates {
		if err := f(p); err != nil {
			return err
		}
	}
	return nil
}

func (m *clusterToObjectMapper) Map(ctx context.Context, _ logr.Logger, reader client.Reader, obj client.Object) []reconcile.Request {
	cluster, ok := obj.(*extensionsv1alpha1.Cluster)
	if !ok {
		return nil
	}

	objList := m.newObjListFunc()
	if err := reader.List(ctx, objList, client.InNamespace(cluster.Name)); err != nil {
		return nil
	}

	return ObjectListToRequests(objList, func(o client.Object) bool {
		return predicateutils.EvalGeneric(o, m.predicates...)
	})
}

// ClusterToObjectMapper returns a mapper that returns requests for objects whose
// referenced clusters have been modified.
func ClusterToObjectMapper(newObjListFunc func() client.ObjectList, predicates []predicate.Predicate) Mapper {
	return &clusterToObjectMapper{newObjListFunc: newObjListFunc, predicates: predicates}
}
