// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package node

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"
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
const ControllerName = "node"

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(mgr manager.Manager, targetCluster cluster.Cluster) error {
	if r.TargetClient == nil {
		r.TargetClient = targetCluster.GetClient()
	}
	if r.Recorder == nil {
		r.Recorder = targetCluster.GetEventRecorderFor("gardener-" + ControllerName + "-controller") // node-controller is ambiguous
	}

	return builder.
		ControllerManagedBy(mgr).
		Named(ControllerName).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: pointer.IntDeref(r.Config.ConcurrentSyncs, 0),
		}).
		Watches(
			source.Kind(targetCluster.GetCache(), &corev1.Node{}),
			&handler.EnqueueRequestForObject{},
			builder.WithPredicates(r.NodePredicate()),
		).
		Complete(r)
}

// NodePredicate returns a predicate that filters for Node objects that are created with the taint.
func (r *Reconciler) NodePredicate() predicate.Predicate {
	return predicate.And(
		predicateutils.ForEventTypes(predicateutils.Create),
		predicate.NewPredicateFuncs(func(obj client.Object) bool {
			return NodeHasCriticalComponentsNotReadyTaint(obj)
		}),
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
