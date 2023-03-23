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

package garden

import (
	"fmt"

	"github.com/Masterminds/semver"
	"k8s.io/utils/clock"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
)

// ControllerName is the name of this controller.
const ControllerName = "garden"

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(mgr manager.Manager) error {
	var err error

	if r.RuntimeClientSet == nil {
		r.RuntimeClientSet, err = kubernetes.NewWithConfig(
			kubernetes.WithRESTConfig(mgr.GetConfig()),
			kubernetes.WithRuntimeAPIReader(mgr.GetAPIReader()),
			kubernetes.WithRuntimeClient(mgr.GetClient()),
			kubernetes.WithRuntimeCache(mgr.GetCache()),
		)
		if err != nil {
			return fmt.Errorf("failed creating seed clientset: %w", err)
		}
	}
	if r.RuntimeVersion == nil {
		serverVersion, err := r.RuntimeClientSet.DiscoverVersion()
		if err != nil {
			return fmt.Errorf("failed getting server version for runtime cluster: %w", err)
		}
		r.RuntimeVersion, err = semver.NewVersion(serverVersion.GitVersion)
		if err != nil {
			return fmt.Errorf("failed parsing version %q for runtime cluster: %w", serverVersion.GitVersion, err)
		}
	}
	if r.Clock == nil {
		r.Clock = clock.RealClock{}
	}
	if r.Recorder == nil {
		r.Recorder = mgr.GetEventRecorderFor(ControllerName + "-controller")
	}
	if r.GardenNamespace == "" {
		r.GardenNamespace = v1beta1constants.GardenNamespace
	}

	return builder.
		ControllerManagedBy(mgr).
		Named(ControllerName).
		For(&operatorv1alpha1.Garden{}, builder.WithPredicates(predicate.Or(predicate.GenerationChangedPredicate{}, r.HasOperationAnnotation()))).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: pointer.IntDeref(r.Config.Controllers.Garden.ConcurrentSyncs, 0),
		}).
		Complete(r)
}

// HasOperationAnnotation returns a predicate which returns true when the object has an operation annotation.
func (r *Reconciler) HasOperationAnnotation() predicate.Predicate {
	hasOperationAnnotation := func(annotations map[string]string) bool {
		return operatorv1alpha1.AvailableOperationAnnotations.Has(annotations[v1beta1constants.GardenerOperation])
	}

	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return hasOperationAnnotation(e.Object.GetAnnotations())
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return !hasOperationAnnotation(e.ObjectOld.GetAnnotations()) && hasOperationAnnotation(e.ObjectNew.GetAnnotations())
		},
		DeleteFunc:  func(e event.DeleteEvent) bool { return false },
		GenericFunc: func(e event.GenericEvent) bool { return false },
	}
}
