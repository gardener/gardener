// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package extensions

import (
	"fmt"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

// ControllerName is the name of this controller.
const ControllerName = "shootstate-extensions"

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(mgr manager.Manager, gardenCluster, seedCluster cluster.Cluster) error {
	if r.GardenClient == nil {
		r.GardenClient = gardenCluster.GetClient()
	}
	if r.SeedClient == nil {
		r.SeedClient = seedCluster.GetClient()
	}

	return builder.
		ControllerManagedBy(mgr).
		Named(fmt.Sprintf("%s-%s", ControllerName, r.ObjectKind)).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: pointer.IntDeref(r.Config.ConcurrentSyncs, 0),
		}).
		Watches(
			source.NewKindWithCache(r.NewObjectFunc(), seedCluster.GetCache()),
			&handler.EnqueueRequestForObject{},
			builder.WithPredicates(
				r.ObjectPredicate(),
				r.InvalidOperationAnnotationPredicate(),
			),
		).
		Complete(r)
}

// ObjectPredicate returns true for 'create' and 'update' events. For updates, it only returns true when the extension
// state or the extension resources in the status have changed, or when the operation annotation has changed.
func (r *Reconciler) ObjectPredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(_ event.CreateEvent) bool {
			return true
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			// enqueue on periodic cache resyncs
			if e.ObjectOld.GetResourceVersion() == e.ObjectNew.GetResourceVersion() {
				return true
			}

			extensionObj, ok := e.ObjectNew.(extensionsv1alpha1.Object)
			if !ok {
				return false
			}

			oldExtensionObj, ok := e.ObjectOld.(extensionsv1alpha1.Object)
			if !ok {
				return false
			}

			return !apiequality.Semantic.DeepEqual(extensionObj.GetExtensionStatus().GetState(), oldExtensionObj.GetExtensionStatus().GetState()) ||
				!apiequality.Semantic.DeepEqual(extensionObj.GetExtensionStatus().GetResources(), oldExtensionObj.GetExtensionStatus().GetResources()) ||
				(invalidOperationAnnotations.Has(oldExtensionObj.GetAnnotations()[v1beta1constants.GardenerOperation]) && !invalidOperationAnnotations.Has(extensionObj.GetAnnotations()[v1beta1constants.GardenerOperation]))
		},
		DeleteFunc:  func(_ event.DeleteEvent) bool { return true },
		GenericFunc: func(_ event.GenericEvent) bool { return false },
	}
}

var invalidOperationAnnotations = sets.New(
	v1beta1constants.GardenerOperationWaitForState,
	v1beta1constants.GardenerOperationRestore,
	v1beta1constants.GardenerOperationMigrate,
)

// InvalidOperationAnnotationPredicate returns a predicate which evaluates to false if the object has one of the
// following operation annotations: 'wait-for-state', 'restore', 'migrate'.
func (r *Reconciler) InvalidOperationAnnotationPredicate() predicate.Predicate {
	return predicate.NewPredicateFuncs(func(obj client.Object) bool {
		return !invalidOperationAnnotations.Has(obj.GetAnnotations()[v1beta1constants.GardenerOperation])
	})
}
