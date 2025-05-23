// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package mapper

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
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

// ObjectListToRequests adds a reconcile.Request for each object in the provided list.
func ObjectListToRequests(list client.ObjectList, predicates ...func(client.Object) bool) []reconcile.Request {
	var requests []reconcile.Request

	utilruntime.HandleError(meta.EachListItem(list, func(object runtime.Object) error {
		obj, ok := object.(client.Object)
		if !ok {
			return fmt.Errorf("cannot convert object of type %T to client.Object", object)
		}

		for _, predicate := range predicates {
			if !predicate(obj) {
				return nil
			}
		}

		requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{
			Name:      obj.GetName(),
			Namespace: obj.GetNamespace(),
		}})

		return nil
	}))

	return requests
}
