// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package inplaceupdate

import (
	"context"

	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/clock"
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

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/controllerutils"
)

// ControllerName is the name of this controller.
const ControllerName = "shoot-inplace-update"

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(mgr manager.Manager, seedCluster cluster.Cluster) error {
	if r.SeedClient == nil {
		r.SeedClient = seedCluster.GetClient()
	}
	if r.Clock == nil {
		r.Clock = clock.RealClock{}
	}

	return builder.
		ControllerManagedBy(mgr).
		Named(ControllerName).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 1,
			ReconciliationTimeout:   controllerutils.DefaultReconciliationTimeout,
		}).
		WatchesRawSource(source.Kind[client.Object](
			seedCluster.GetCache(),
			&corev1.Node{},
			handler.EnqueueRequestsFromMapFunc(MapNodeToPool),
			NodeInPlaceUpdateStatePredicate(),
		)).
		Complete(r)
}

// MapNodeToPool maps a node event to a reconcile request for its worker pool osc secret.
func MapNodeToPool(_ context.Context, obj client.Object) []reconcile.Request {
	poolSecretName := obj.GetLabels()[v1beta1constants.LabelWorkerPoolGardenerNodeAgentSecretName]
	if poolSecretName == "" {
		return nil
	}
	return []reconcile.Request{{NamespacedName: client.ObjectKey{Name: poolSecretName}}}
}

// NodeInPlaceUpdateStatePredicate returns a predicate that triggers pool reconciliation when a node's in-place-update state changes.
func NodeInPlaceUpdateStatePredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return nodeNeedsDrain(e.Object) || nodeUpdateResult(e.Object) != "" || nodeHasInPlaceUpdateConditionFromObject(e.Object)
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return (!nodeNeedsDrain(e.ObjectOld) && nodeNeedsDrain(e.ObjectNew)) ||
				nodeUpdateResult(e.ObjectOld) != nodeUpdateResult(e.ObjectNew)
		},
		DeleteFunc:  func(_ event.DeleteEvent) bool { return false },
		GenericFunc: func(_ event.GenericEvent) bool { return false },
	}
}

func nodeNeedsDrain(obj client.Object) bool {
	return obj.GetAnnotations()[v1beta1constants.AnnotationNodeAgentInPlaceUpdateNeedsDrain] == "true"
}

func nodeUpdateResult(obj client.Object) string {
	return obj.GetLabels()[machinev1alpha1.LabelKeyNodeUpdateResult]
}

func nodeHasInPlaceUpdateConditionFromObject(obj client.Object) bool {
	node, ok := obj.(*corev1.Node)
	if !ok {
		return false
	}
	return nodeHasInPlaceUpdateCondition(node)
}
