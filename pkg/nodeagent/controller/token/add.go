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

package token

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
	"github.com/gardener/gardener/pkg/nodeagent/controller/common"
)

// ControllerName is the name of this controller.
const ControllerName = "token"

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(mgr manager.Manager) error {
	if r.Client == nil {
		r.Client = mgr.GetClient()
	}

	return builder.
		ControllerManagedBy(mgr).
		Named(ControllerName).
		For(&corev1.Secret{}, builder.WithPredicates(r.SecretPredicate())).
		Complete(r)
}

// SecretPredicate returns the predicate for Secret events.
func (r *Reconciler) SecretPredicate() predicate.Predicate {
	return predicate.And(
		predicate.NewPredicateFuncs(func(obj client.Object) bool {
			config, err := common.ReadNodeAgentConfiguration(r.Fs)
			if err != nil {
				return false
			}

			r.Config = config

			return obj.GetNamespace() == metav1.NamespaceSystem && obj.GetName() == r.Config.TokenSecretName
		}),
		predicate.Funcs{
			CreateFunc: func(_ event.CreateEvent) bool {
				config, err := common.ReadNodeAgentConfiguration(r.Fs)
				if err != nil {
					return false
				}

				r.Config = config

				return true
			},
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

				return !apiequality.Semantic.DeepEqual(oldSecret.Data[v1alpha1.NodeAgentTokenSecretKey], newSecret.Data[v1alpha1.NodeAgentTokenSecretKey])
			},
		},
	)
}
