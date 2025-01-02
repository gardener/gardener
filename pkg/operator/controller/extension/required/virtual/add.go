// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package virtual

import (
	"context"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/utils/clock"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	predicateutils "github.com/gardener/gardener/pkg/controllerutils/predicate"
)

// ControllerName is the name of this controller.
const ControllerName = "extension-required-virtual"

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(mgr manager.Manager, virtualCluster cluster.Cluster) error {
	if r.RuntimeClient == nil {
		r.RuntimeClient = mgr.GetClient()
	}
	if r.VirtualClient == nil {
		r.VirtualClient = virtualCluster.GetClient()
	}
	r.clock = clock.RealClock{}

	return builder.
		ControllerManagedBy(mgr).
		Named(ControllerName).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: ptr.Deref(r.Config.ConcurrentSyncs, 0),
		}).
		For(&operatorv1alpha1.Extension{}, builder.WithPredicates(predicateutils.ForEventTypes(predicateutils.Create))).
		WatchesRawSource(
			source.Kind[client.Object](virtualCluster.GetCache(), &gardencorev1beta1.ControllerInstallation{},
				handler.EnqueueRequestsFromMapFunc(r.MapControllerInstallationToExtension(mgr.GetLogger().WithValues("controller", ControllerName))),
				predicateutils.ForEventTypes(predicateutils.Create, predicateutils.Update, predicateutils.Delete),
				r.RequiredConditionChangedPredicate(),
			),
		).
		Complete(r)
}

// MapControllerInstallationToExtension returns a mapper that maps the ControllerInstallation to the Extension object.
func (r *Reconciler) MapControllerInstallationToExtension(log logr.Logger) handler.MapFunc {
	return func(ctx context.Context, obj client.Object) []reconcile.Request {
		controllerInstallation, ok := obj.(*gardencorev1beta1.ControllerInstallation)
		if !ok {
			return nil
		}

		var (
			extensionName = controllerInstallation.Spec.RegistrationRef.Name
			extension     = &operatorv1alpha1.Extension{}
		)

		if err := r.RuntimeClient.Get(ctx, client.ObjectKey{Name: controllerInstallation.Spec.RegistrationRef.Name}, extension); err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			log.Error(err, "Unable to get extension", "extension", extensionName)
		}

		return []reconcile.Request{{NamespacedName: client.ObjectKeyFromObject(extension)}}
	}
}

// RequiredConditionChangedPredicate is a predicate that returns true if the ControllerInstallationRequired changed for ControllerInstallations.
func (r *Reconciler) RequiredConditionChangedPredicate() predicate.Predicate {
	return predicate.Funcs{
		UpdateFunc: func(e event.TypedUpdateEvent[client.Object]) bool {
			controllerInstallationOld, ok := e.ObjectOld.(*gardencorev1beta1.ControllerInstallation)
			if !ok {
				return false
			}
			controllerInstallationNew, ok := e.ObjectNew.(*gardencorev1beta1.ControllerInstallation)
			if !ok {
				return false
			}

			condOld := v1beta1helper.GetCondition(controllerInstallationOld.Status.Conditions, gardencorev1beta1.ControllerInstallationRequired)
			requiredOld := condOld != nil && condOld.Status == gardencorev1beta1.ConditionTrue

			condNew := v1beta1helper.GetCondition(controllerInstallationNew.Status.Conditions, gardencorev1beta1.ControllerInstallationRequired)
			requiredNew := condNew != nil && condNew.Status == gardencorev1beta1.ConditionTrue

			return requiredNew != requiredOld
		},
	}
}
