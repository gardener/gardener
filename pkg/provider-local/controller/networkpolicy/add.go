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

package networkpolicy

import (
	"context"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/provider-local/local"
)

// ControllerName is the name of the controller.
const ControllerName = "networkpolicy"

// AddToManager adds a controller to the given manager.
func AddToManager(_ context.Context, mgr manager.Manager) error {
	return (&Reconciler{}).AddToManager(mgr)
}

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(mgr manager.Manager) error {
	if r.Client == nil {
		r.Client = mgr.GetClient()
	}

	return builder.
		ControllerManagedBy(mgr).
		Named(ControllerName).
		For(&corev1.Namespace{}, builder.WithPredicates(IsShootNamespace(), IsShootProviderLocal())).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 5,
		}).
		Complete(r)
}

// IsShootNamespace returns a predicate that returns true if the namespace is a shoot namespace.
func IsShootNamespace() predicate.Predicate {
	return predicate.NewPredicateFuncs(func(obj client.Object) bool {
		return strings.HasPrefix(obj.GetName(), v1beta1constants.TechnicalIDPrefix)
	})
}

// IsShootProviderLocal returns a predicate that returns true if the provider of the shoot is of type "local".
func IsShootProviderLocal() predicate.Predicate {
	return predicate.NewPredicateFuncs(func(obj client.Object) bool {
		value, hasLabel := obj.GetLabels()[v1beta1constants.LabelShootProvider]

		return hasLabel && value == local.Type
	})
}
