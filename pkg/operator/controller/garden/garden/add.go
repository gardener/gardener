// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package garden

import (
	"context"
	"fmt"

	"github.com/Masterminds/semver/v3"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/clock"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/gardener/gardener/pkg/api/operator/v1alpha1/helper"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/controllerutils/mapper"
	operatorpredicate "github.com/gardener/gardener/pkg/operator/predicate"
)

// ControllerName is the name of this controller.
const ControllerName = "garden"

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(mgr manager.Manager, gardenClientMap clientmap.ClientMap) error {
	var err error

	if r.Manager == nil {
		r.Manager = mgr
	}
	if r.RuntimeClientSet == nil {
		r.RuntimeClientSet, err = kubernetes.NewWithConfig(
			kubernetes.WithRESTConfig(mgr.GetConfig()),
			kubernetes.WithRuntimeAPIReader(mgr.GetAPIReader()),
			kubernetes.WithRuntimeClient(mgr.GetClient()),
			kubernetes.WithRuntimeCache(mgr.GetCache()),
		)
		if err != nil {
			return fmt.Errorf("failed creating runtime clientset: %w", err)
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
		r.Recorder = mgr.GetEventRecorder(ControllerName + "-controller")
	}
	if r.GardenNamespace == "" {
		r.GardenNamespace = v1beta1constants.GardenNamespace
	}
	if gardenClientMap == nil {
		return fmt.Errorf("gardenClientMap must not be nil")
	}
	r.GardenClientMap = gardenClientMap

	return builder.
		ControllerManagedBy(mgr).
		Named(ControllerName).
		For(&operatorv1alpha1.Garden{}, builder.WithPredicates(predicate.Or(predicate.GenerationChangedPredicate{}, r.HasOperationAnnotation()))).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: ptr.Deref(r.Config.Controllers.Garden.ConcurrentSyncs, 0),
		}).
		Watches(&operatorv1alpha1.Extension{},
			handler.EnqueueRequestsFromMapFunc(r.MapToGarden(mgr.GetLogger().WithValues("controller", ControllerName))),
			builder.WithPredicates(operatorpredicate.ExtensionResourcesChanged()),
		).
		Complete(r)
}

// MapToGarden returns a reconcile.Request for the Garden.
func (r *Reconciler) MapToGarden(log logr.Logger) handler.MapFunc {
	return func(ctx context.Context, _ client.Object) []reconcile.Request {
		gardenList := &operatorv1alpha1.GardenList{}
		if err := r.RuntimeClientSet.Client().List(ctx, gardenList, client.Limit(1)); err != nil {
			log.Error(err, "Could not list gardens")
			return nil
		}

		return mapper.ObjectListToRequests(gardenList)
	}
}

// HasOperationAnnotation returns a predicate which returns true when the object has an operation annotation.
func (r *Reconciler) HasOperationAnnotation() predicate.Predicate {
	hasOperationAnnotation := func(annotations map[string]string) bool {
		operations := helper.GetGardenerOperations(annotations)
		return operatorv1alpha1.AvailableOperationAnnotations.HasAny(operations...)
	}

	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return hasOperationAnnotation(e.Object.GetAnnotations())
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return (!hasOperationAnnotation(e.ObjectOld.GetAnnotations()) && hasOperationAnnotation(e.ObjectNew.GetAnnotations())) ||
				(hasOperationAnnotation(e.ObjectOld.GetAnnotations()) && hasOperationAnnotation(e.ObjectNew.GetAnnotations()) &&
					!sets.New(helper.GetGardenerOperations(e.ObjectOld.GetAnnotations())...).Equal(sets.New(helper.GetGardenerOperations(e.ObjectNew.GetAnnotations())...)))
		},
		DeleteFunc:  func(_ event.DeleteEvent) bool { return false },
		GenericFunc: func(_ event.GenericEvent) bool { return false },
	}
}
