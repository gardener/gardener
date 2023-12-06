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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/gardener/gardener/pkg/nodeagent/dbus"
)

// ControllerName is the name of this controller.
const ControllerName = "node"

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(mgr manager.Manager) error {
	if r.Client == nil {
		r.Client = mgr.GetClient()
	}
	if r.Recorder == nil {
		r.Recorder = mgr.GetEventRecorderFor(ControllerName)
	}
	if r.DBus == nil {
		r.DBus = dbus.New()
	}

	node := &metav1.PartialObjectMetadata{}
	node.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("Node"))

	return builder.
		ControllerManagedBy(mgr).
		Named(ControllerName).
		For(node, builder.WithPredicates(r.NodePredicate())).
		WithOptions(controller.Options{MaxConcurrentReconciles: 1}).
		Complete(r)
}

// NodePredicate returns 'true' when the annotation describing which systemd services should be restarted gets set or
// changed. When it's removed, 'false' is returned.
func (r *Reconciler) NodePredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return e.Object.GetAnnotations()[annotationRestartSystemdServices] != ""
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return e.ObjectOld.GetAnnotations()[annotationRestartSystemdServices] != e.ObjectNew.GetAnnotations()[annotationRestartSystemdServices] &&
				e.ObjectNew.GetAnnotations()[annotationRestartSystemdServices] != ""
		},
		DeleteFunc:  func(_ event.DeleteEvent) bool { return false },
		GenericFunc: func(_ event.GenericEvent) bool { return false },
	}
}
