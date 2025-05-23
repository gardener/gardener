// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package criticalcomponents

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	predicateutils "github.com/gardener/gardener/pkg/controllerutils/predicate"
)

// ControllerName is the name of the controller.
const ControllerName = "node-critical-components"

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(mgr manager.Manager, targetCluster cluster.Cluster) error {
	if r.TargetClient == nil {
		r.TargetClient = targetCluster.GetClient()
	}
	if r.Recorder == nil {
		r.Recorder = targetCluster.GetEventRecorderFor(ControllerName + "-controller")
	}

	return builder.
		ControllerManagedBy(mgr).
		Named(ControllerName).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: ptr.Deref(r.Config.ConcurrentSyncs, 0),
		}).
		WatchesRawSource(
			source.Kind[client.Object](targetCluster.GetCache(),
				&corev1.Node{},
				&handler.EnqueueRequestForObject{},
				r.NodePredicate()),
		).
		Complete(r)
}

// NodePredicate returns a predicate that filters for Node objects that are created with the taint.
func (r *Reconciler) NodePredicate() predicate.Predicate {
	return predicate.And(
		predicateutils.ForEventTypes(predicateutils.Create, predicateutils.Update),
		predicate.NewPredicateFuncs(NodeHasCriticalComponentsNotReadyTaint),
	)
}

// NodeHasCriticalComponentsNotReadyTaint returns true if the given Node has the taint that this controller manages.
func NodeHasCriticalComponentsNotReadyTaint(obj client.Object) bool {
	node, ok := obj.(*corev1.Node)
	if !ok {
		return false
	}

	for _, taint := range node.Spec.Taints {
		if taint.Key == v1beta1constants.TaintNodeCriticalComponentsNotReady {
			return true
		}
	}
	return false
}
