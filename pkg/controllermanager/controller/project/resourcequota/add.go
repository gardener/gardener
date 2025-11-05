// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package resourcequota

import (
	"context"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/controllerutils/mapper"
	predicateutils "github.com/gardener/gardener/pkg/controllerutils/predicate"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

// ControllerName is the name of this controller.
const ControllerName = "project-resourcequota"

// AddToManager adds a controller with the given Options to the given manager.
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
		For(&corev1.ResourceQuota{}, builder.WithPredicates(r.ObjectInProjectNamespace(
			context.Background(),
			mgr.GetLogger().WithValues("controller", ControllerName)))).
		Watches(
			&gardencorev1beta1.Shoot{},
			handler.EnqueueRequestsFromMapFunc(r.MapShootToResourceQuotasInProject(mgr.GetLogger().WithValues("controller", ControllerName))),
			builder.WithPredicates(predicateutils.ForEventTypes(predicateutils.Create)),
		).
		Complete(r)
}

// ObjectInProjectNamespace returns a predicate that filters objects that are in Project namespaces.
func (r *Reconciler) ObjectInProjectNamespace(ctx context.Context, log logr.Logger) predicate.Predicate {
	return predicate.NewPredicateFuncs(func(object client.Object) bool {
		namespace := object.GetNamespace()
		project, err := gardenerutils.ProjectForNamespaceFromReader(ctx, r.Client, namespace)
		if err != nil {
			log.Error(err, "Unable to find gardener project", "namespace", namespace)
		}
		return err == nil && project != nil
	})
}

// MapShootToResourceQuotasInProject maps Shoots to ResourceQuotas in the corresponding Project namespace.
func (r *Reconciler) MapShootToResourceQuotasInProject(log logr.Logger) handler.MapFunc {
	return func(ctx context.Context, obj client.Object) []reconcile.Request {
		resourceQuotaList := &corev1.ResourceQuotaList{}
		if err := r.Client.List(ctx, resourceQuotaList, client.InNamespace(obj.GetNamespace())); err != nil {
			log.Error(err, "Unable to list resource quotas")
		}

		return mapper.ObjectListToRequests(resourceQuotaList)
	}
}
