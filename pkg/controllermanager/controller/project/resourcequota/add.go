// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package resourcequota

import (
	"context"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/controllerutils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

// ControllerName is the name of this controller.
const ControllerName = "project-resourcequota"

var (
	createOnlyPredicate = predicate.Funcs{
		CreateFunc:  func(e event.CreateEvent) bool { return true },
		DeleteFunc:  func(e event.DeleteEvent) bool { return false },
		UpdateFunc:  func(e event.UpdateEvent) bool { return false },
		GenericFunc: func(e event.GenericEvent) bool { return false },
	}
)

func (r *Reconciler) AddToManager(mgr manager.Manager) error {
	if r.Client == nil {
		r.Client = mgr.GetClient()
	}

	return builder.
		ControllerManagedBy(mgr).
		Named(ControllerName).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: ptr.Deref(r.Config.ConcurrentSyncs, 0),
			ReconciliationTimeout:   controllerutils.DefaultReconciliationTimeout,
		}).
		For(&corev1.ResourceQuota{}, builder.WithPredicates(r.ObjectInProjectNamespace())).
		Watches(
			&v1beta1.Shoot{},
			handler.EnqueueRequestsFromMapFunc(r.MapShootToResourceQuotasInProject(mgr.GetLogger().WithValues("controller", ControllerName))),
			builder.WithPredicates(createOnlyPredicate),
		).
		Complete(r)
}

// ObjectInProjectNamespace returns a predicate that filters objects that are in Project namespaces.
func (r *Reconciler) ObjectInProjectNamespace() predicate.Predicate {
	objectInNamespaceFunc := func(namespace string) bool {
		ctx := context.Background()
		project, err := gardenerutils.ProjectForNamespaceFromReader(ctx, r.Client, namespace)
		return err == nil && project != nil
	}

	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return objectInNamespaceFunc(e.Object.GetNamespace())
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return objectInNamespaceFunc(e.Object.GetNamespace())
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return objectInNamespaceFunc(e.ObjectNew.GetNamespace())
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return objectInNamespaceFunc(e.Object.GetNamespace())
		},
	}
}

// MapShootToResourceQuotasInProject maps Shoots to ResourceQuotas in the corresponding Project namespace.
func (r *Reconciler) MapShootToResourceQuotasInProject(log logr.Logger) handler.MapFunc {
	return func(ctx context.Context, obj client.Object) []reconcile.Request {
		resourceQuotaList := &corev1.ResourceQuotaList{}
		if err := r.Client.List(ctx, resourceQuotaList, client.InNamespace(obj.GetNamespace())); err != nil {
			log.Error(err, "unable to list resource quotas")
		}

		requests := make([]reconcile.Request, 0, len(resourceQuotaList.Items))
		for _, rq := range resourceQuotaList.Items {
			requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{
				Name:      rq.Name,
				Namespace: rq.Namespace,
			}})
		}
		return requests
	}
}
