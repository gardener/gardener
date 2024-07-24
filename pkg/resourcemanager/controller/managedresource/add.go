// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package managedresource

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/clock"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils/mapper"
	predicateutils "github.com/gardener/gardener/pkg/controllerutils/predicate"
	reconcilerutils "github.com/gardener/gardener/pkg/controllerutils/reconciler"
	resourcemanagerpredicate "github.com/gardener/gardener/pkg/resourcemanager/predicate"
)

// ControllerName is the name of the controller.
const ControllerName = "managedresource"

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(ctx context.Context, mgr manager.Manager, sourceCluster, targetCluster cluster.Cluster) error {
	if r.SourceClient == nil {
		r.SourceClient = sourceCluster.GetClient()
	}
	if r.TargetClient == nil {
		r.TargetClient = targetCluster.GetClient()
	}
	if r.TargetScheme == nil {
		r.TargetScheme = targetCluster.GetScheme()
	}
	if r.Clock == nil {
		r.Clock = clock.RealClock{}
	}
	if r.TargetRESTMapper == nil {
		r.TargetRESTMapper = targetCluster.GetRESTMapper()
	}
	if r.RequeueAfterOnDeletionPending == nil {
		r.RequeueAfterOnDeletionPending = ptr.To(5 * time.Second)
	}

	c, err := builder.
		ControllerManagedBy(mgr).
		Named(ControllerName).
		For(&resourcesv1alpha1.ManagedResource{}, builder.WithPredicates(
			r.ClassFilter,
			predicate.Or(
				predicate.GenerationChangedPredicate{},
				resourcemanagerpredicate.HasOperationAnnotation(),
				resourcemanagerpredicate.ConditionStatusChanged(resourcesv1alpha1.ResourcesHealthy, resourcemanagerpredicate.ConditionChangedToUnhealthy),
				resourcemanagerpredicate.NoLongerIgnored(),
				// we need to reconcile once if the ManagedResource got marked as ignored in order to update the conditions
				resourcemanagerpredicate.GotMarkedAsIgnored(),
			),
			// TODO: refactor this predicate chain into a single predicate.Funcs that can be properly tested as a whole
			predicate.Or(
				// Added again here, as otherwise NotIgnored would filter this add/update event out
				resourcemanagerpredicate.GotMarkedAsIgnored(),
				resourcemanagerpredicate.NotIgnored(),
				predicateutils.IsDeleting(),
			),
		)).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: ptr.Deref(r.Config.ConcurrentSyncs, 0),
		}).
		Build(reconcilerutils.OperationAnnotationWrapper(
			mgr,
			func() client.Object { return &resourcesv1alpha1.ManagedResource{} },
			r,
		))
	if err != nil {
		return err
	}

	return c.Watch(
		source.Kind(mgr.GetCache(), &corev1.Secret{},
			mapper.EnqueueRequestsFrom(ctx, mgr.GetCache(), r.MapSecretToManagedResources(
				r.ClassFilter,
				predicate.Or(
					resourcemanagerpredicate.NotIgnored(),
					predicateutils.IsDeleting(),
				),
			), mapper.UpdateWithOldAndNew, c.GetLogger())),
	)
}

// MapSecretToManagedResources maps secrets to relevant ManagedResources.
func (r *Reconciler) MapSecretToManagedResources(managedResourcePredicates ...predicate.Predicate) mapper.MapFunc {
	return func(ctx context.Context, _ logr.Logger, reader client.Reader, obj client.Object) []reconcile.Request {
		if obj == nil {
			return nil
		}

		secret, ok := obj.(*corev1.Secret)
		if !ok {
			return nil
		}

		managedResourceList := &resourcesv1alpha1.ManagedResourceList{}
		if err := reader.List(ctx, managedResourceList, client.InNamespace(secret.Namespace)); err != nil {
			return nil
		}

		var requests []reconcile.Request
		for _, mr := range managedResourceList.Items {
			if !predicateutils.EvalGeneric(&mr, managedResourcePredicates...) {
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
}
