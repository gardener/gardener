// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package refallowlist

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/gardener/gardener/pkg/apis/seedmanagement/encoding"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils"
)

// ControllerName is the name of this controller.
const ControllerName = "seed-reference-allowlist"

// Request is the reconcile request type for this controller. It identifies a referenced resource
// and the seed name to add to or remove from its `seed.gardener.cloud/names` annotation.
type Request struct {
	// APIVersion of the referenced resource.
	APIVersion string
	// Kind of the referenced resource.
	Kind string
	// Namespace of the referenced resource.
	Namespace string
	// Name of the referenced resource.
	Name string
	// SeedName is the seed that references (or no longer references) this resource.
	SeedName string
	// Remove indicates that SeedName should be removed from the annotation.
	Remove bool
}

// AddToManager adds the Reconciler to the given manager.
func (r *Reconciler) AddToManager(mgr manager.Manager) error {
	if r.Client == nil {
		r.Client = mgr.GetClient()
	}

	return builder.
		TypedControllerManagedBy[Request](mgr).
		Named(ControllerName).
		Watches(
			&seedmanagementv1alpha1.Gardenlet{},
			SeedEventHandler(func(obj *seedmanagementv1alpha1.Gardenlet) *runtime.RawExtension {
				return &obj.Spec.Config
			}),
			builder.WithPredicates(predicate.GenerationChangedPredicate{}),
		).
		Watches(
			&seedmanagementv1alpha1.ManagedSeed{},
			SeedEventHandler(func(obj *seedmanagementv1alpha1.ManagedSeed) *runtime.RawExtension {
				return &obj.Spec.Gardenlet.Config
			}),
			builder.WithPredicates(predicate.GenerationChangedPredicate{}),
		).
		WithOptions(controller.TypedOptions[Request]{
			MaxConcurrentReconciles: 5,
			ReconciliationTimeout:   controllerutils.DefaultReconciliationTimeout,
		}).
		Complete(r)
}

// SeedEventHandler returns an event handler that maps `Gardenlet` or `ManagedSeed` events to
// reconcile requests for every credential/resource object referenced in their embedded `SeedSpec`.
// On update events only refs that actually changed are enqueued: dropped refs are marked for
// removal and newly added refs are marked for addition.
func SeedEventHandler[T any, PT interface {
	*T
	client.Object
}](getConfig func(PT) *runtime.RawExtension) handler.TypedEventHandler[client.Object, Request] {
	refsFor := func(obj client.Object) []ref {
		typed, ok := obj.(PT)
		if !ok {
			return nil
		}
		gardenletConfig, err := encoding.DecodeGardenletConfiguration(getConfig(typed), false)
		if err != nil || gardenletConfig.SeedConfig == nil {
			return nil
		}
		return refsFromSeedSpec(&gardenletConfig.SeedConfig.Spec)
	}

	enqueue := func(q workqueue.TypedRateLimitingInterface[Request], seedName string, refs []ref, remove bool) {
		for _, r := range refs {
			q.Add(Request{
				APIVersion: r.apiVersion,
				Kind:       r.kind,
				Namespace:  r.namespace,
				Name:       r.name,
				SeedName:   seedName,
				Remove:     remove,
			})
		}
	}

	return handler.TypedFuncs[client.Object, Request]{
		CreateFunc: func(_ context.Context, e event.TypedCreateEvent[client.Object], q workqueue.TypedRateLimitingInterface[Request]) {
			enqueue(q, e.Object.GetName(), refsFor(e.Object), false)
		},
		UpdateFunc: func(_ context.Context, e event.TypedUpdateEvent[client.Object], q workqueue.TypedRateLimitingInterface[Request]) {
			oldRefs := refsFor(e.ObjectOld)
			newRefs := refsFor(e.ObjectNew)
			newSet := sets.New(newRefs...)
			oldSet := sets.New(oldRefs...)
			for _, r := range oldRefs {
				if !newSet.Has(r) {
					enqueue(q, e.ObjectOld.GetName(), []ref{r}, true)
				}
			}
			for _, r := range newRefs {
				if !oldSet.Has(r) {
					enqueue(q, e.ObjectNew.GetName(), []ref{r}, false)
				}
			}
		},
		DeleteFunc: func(_ context.Context, e event.TypedDeleteEvent[client.Object], q workqueue.TypedRateLimitingInterface[Request]) {
			enqueue(q, e.Object.GetName(), refsFor(e.Object), true)
		},
		GenericFunc: func(_ context.Context, e event.TypedGenericEvent[client.Object], q workqueue.TypedRateLimitingInterface[Request]) {
			enqueue(q, e.Object.GetName(), refsFor(e.Object), false)
		},
	}
}
