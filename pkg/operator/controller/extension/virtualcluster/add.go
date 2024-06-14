// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package virtualcluster

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/clock"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	clientmapbuilder "github.com/gardener/gardener/pkg/client/kubernetes/clientmap/builder"
	"github.com/gardener/gardener/pkg/controllerutils/mapper"
	"github.com/gardener/gardener/pkg/operator/controller/garden/care"
)

// ControllerName is the name of this controller.
const ControllerName = "extension-virtual-cluster"

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(ctx context.Context, mgr manager.Manager) error {
	if r.RuntimeClient == nil {
		runtimeClientSet, err := kubernetes.NewWithConfig(
			kubernetes.WithRESTConfig(mgr.GetConfig()),
			kubernetes.WithRuntimeAPIReader(mgr.GetAPIReader()),
			kubernetes.WithRuntimeClient(mgr.GetClient()),
			kubernetes.WithRuntimeCache(mgr.GetCache()),
		)
		if err != nil {
			return fmt.Errorf("failed creating runtime clientset: %w", err)
		}
		r.RuntimeClient = runtimeClientSet.Client()
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
	if r.GardenClientMap == nil {
		var err error
		r.GardenClientMap, err = clientmapbuilder.
			NewGardenClientMapBuilder().
			WithRuntimeClient(r.RuntimeClient).
			WithClientConnectionConfig(&r.Config.VirtualClientConnection).
			WithGardenNamespace(r.GardenNamespace).
			Build(mgr.GetLogger())
		if err != nil {
			return fmt.Errorf("failed to build garden ClientMap: %w", err)
		}
		if err := mgr.Add(r.GardenClientMap); err != nil {
			return err
		}
	}

	return builder.
		ControllerManagedBy(mgr).
		Named(ControllerName).
		For(&operatorv1alpha1.Extension{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: ptr.Deref(r.Config.Controllers.ExtensionVirtualClusterConfig.ConcurrentSyncs, 0),
		}).
		Watches(
			&operatorv1alpha1.Garden{},
			mapper.EnqueueRequestsFrom(ctx, mgr.GetCache(), mapper.MapFunc(r.MapToAllExtensions), mapper.UpdateWithNew, mgr.GetLogger()),
			builder.WithPredicates(predicate.Or(care.GardenPredicate(), predicate.GenerationChangedPredicate{})),
		).
		Complete(r)
}

// MapToAllExtensions returns reconcile.Request objects for all existing gardens in the system.
func (r *Reconciler) MapToAllExtensions(ctx context.Context, log logr.Logger, reader client.Reader, _ client.Object) []reconcile.Request {
	extensionList := &metav1.PartialObjectMetadataList{}
	extensionList.SetGroupVersionKind(operatorv1alpha1.SchemeGroupVersion.WithKind("ExtensionList"))
	if err := reader.List(ctx, extensionList); err != nil {
		log.Error(err, "Failed to list extensions")
		return nil
	}

	log.Info("AAAAAAAAAAAAAAAAAAA", "foo", extensionList)
	return mapper.ObjectListToRequests(extensionList)
}

// // MapToAllGardens returns reconcile.Request objects for all existing gardens in the system.
// func (r *Reconciler) MapToAllExtensions(ctx context.Context, log logr.Logger, reader client.Reader, _ client.Object) []reconcile.Request {
// 	gardenList := &metav1.PartialObjectMetadataList{}
// 	gardenList.SetGroupVersionKind(operatorv1alpha1.SchemeGroupVersion.WithKind("GardenList"))
// 	if err := reader.List(ctx, gardenList); err != nil {
// 		log.Error(err, "Failed to list gardens")
// 		return nil
// 	}
//
// 	return mapper.ObjectListToRequests(gardenList)
// }