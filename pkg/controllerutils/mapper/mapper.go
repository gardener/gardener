// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package mapper

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	predicateutils "github.com/gardener/gardener/pkg/controllerutils/predicate"
)

// ClusterToObjectMapper returns a mapper that returns requests for objects whose referenced clusters have been
// modified.
func ClusterToObjectMapper(reader client.Reader, newObjListFunc func() client.ObjectList, predicates []predicate.Predicate) handler.MapFunc {
	return func(ctx context.Context, obj client.Object) []reconcile.Request {
		cluster, ok := obj.(*extensionsv1alpha1.Cluster)
		if !ok {
			return nil
		}

		objList := newObjListFunc()
		if err := reader.List(ctx, objList, client.InNamespace(cluster.Name)); err != nil {
			return nil
		}

		return ObjectListToRequests(objList, func(o client.Object) bool {
			return predicateutils.EvalGeneric(o, predicates...)
		})
	}
}
