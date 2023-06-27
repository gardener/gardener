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

package operatingsystemconfig

import (
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/gardener/gardener/pkg/nodeagent/apis/config/v1alpha1"
)

// controllerName is the name of this controller.
const controllerName = "operatingsystemconfig"

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(mgr manager.Manager) error {
	if r.Client == nil {
		r.Client = mgr.GetClient()
	}
	if r.Recorder == nil {
		r.Recorder = mgr.GetEventRecorderFor(controllerName)
	}

	return builder.
		ControllerManagedBy(mgr).
		Named(controllerName).
		For(&corev1.Secret{}, builder.WithPredicates(r.SecretPredicate())).

		// Watches(
		// 	&source.Kind{Type: &corev1.Secret{}},
		// 	r.EventHandler(),
		// 	builder.WithPredicates(r.SecretPredicate()),
		// ).
		Complete(r)
}

// SecretPredicate returns the predicate for Shoot events.
func (r *Reconciler) SecretPredicate() predicate.Predicate {
	return predicate.And(
		predicate.NewPredicateFuncs(func(obj client.Object) bool {
			return obj.GetNamespace() == metav1.NamespaceSystem && obj.GetName() == r.Config.OSCSecretName
		}),
		predicate.Funcs{
			CreateFunc:  func(_ event.CreateEvent) bool { return true },
			DeleteFunc:  func(_ event.DeleteEvent) bool { return false },
			GenericFunc: func(_ event.GenericEvent) bool { return false },
			UpdateFunc: func(e event.UpdateEvent) bool {
				oldSecret, ok := e.ObjectOld.(*corev1.Secret)
				if !ok {
					return false
				}
				newSecret, ok := e.ObjectNew.(*corev1.Secret)
				if !ok {
					return false
				}

				return !apiequality.Semantic.DeepEqual(oldSecret.Data[v1alpha1.NodeAgentOSCSecretKey], newSecret.Data[v1alpha1.NodeAgentOSCSecretKey])
			},
		},
	)
}

// // RandomDurationWithMetaDuration is an alias for utils.RandomDurationWithMetaDuration.
// var RandomDurationWithMetaDuration = utils.RandomDurationWithMetaDuration

// // EventHandler returns a handler for Shoot events.
// func (r *Reconciler) EventHandler() handler.EventHandler {
// 	return &handler.Funcs{
// 		CreateFunc: func(e event.CreateEvent, q workqueue.RateLimitingInterface) {
// 			req := reconcile.Request{NamespacedName: types.NamespacedName{
// 				Name:      e.Object.GetName(),
// 				Namespace: e.Object.GetNamespace(),
// 			}}

// 			q.Add(req)
// 		},
// 		UpdateFunc: func(e event.UpdateEvent, q workqueue.RateLimitingInterface) {
// 			newSecret, ok := e.ObjectNew.(*corev1.Secret)
// 			if !ok {
// 				return
// 			}
// 			oldSecret, ok := e.ObjectOld.(*corev1.Secret)
// 			if !ok {
// 				return
// 			}
// 			_, newCheckSum, err := r.extractOSCFromSecret(newSecret)
// 			if err != nil {
// 				return
// 			}
// 			_, oldCheckSum, err := r.extractOSCFromSecret(oldSecret)
// 			if err != nil {
// 				return
// 			}

// 			req := reconcile.Request{NamespacedName: types.NamespacedName{
// 				Name:      e.ObjectNew.GetName(),
// 				Namespace: e.ObjectNew.GetNamespace(),
// 			}}

// 			if bytes.Equal(newCheckSum, oldCheckSum) {
// 				// no reconcile necessary when checksum is equal
// 				return
// 			}

// 			// spread to avoid restarting all units get restarted on all nodes
// 			q.AddAfter(req, RandomDurationWithMetaDuration(&metav1.Duration{Duration: 1 * time.Minute}))
// 		},
// 	}
// }
