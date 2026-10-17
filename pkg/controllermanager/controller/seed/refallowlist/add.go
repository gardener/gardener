// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package refallowlist

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
	"github.com/gardener/gardener/pkg/apis/seedmanagement/encoding"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils"
)

// ControllerName is the name of this controller.
const ControllerName = "seed-reference-allowlist"

// Request identifies a referenced resource by its GVK, namespace, and name.
type Request struct {
	// APIVersion of the referenced resource.
	APIVersion string
	// Kind of the referenced resource.
	Kind string
	// Namespace of the referenced resource.
	Namespace string
	// Name of the referenced resource.
	Name string
}

// AddToManager adds the Reconciler to the given manager.
func (r *Reconciler) AddToManager(mgr manager.Manager) error {
	if r.Client == nil {
		r.Client = mgr.GetClient()
	}

	secretMeta := &metav1.PartialObjectMetadata{}
	secretMeta.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("Secret"))

	configMapMeta := &metav1.PartialObjectMetadata{}
	configMapMeta.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("ConfigMap"))

	workloadIdentityMeta := &metav1.PartialObjectMetadata{}
	workloadIdentityMeta.SetGroupVersionKind(securityv1alpha1.SchemeGroupVersion.WithKind("WorkloadIdentity"))

	return builder.
		TypedControllerManagedBy[Request](mgr).
		Named(ControllerName).
		WatchesMetadata(
			secretMeta,
			metadataEventHandler(corev1.SchemeGroupVersion.WithKind("Secret")),
		).
		WatchesMetadata(
			configMapMeta,
			metadataEventHandler(corev1.SchemeGroupVersion.WithKind("ConfigMap")),
		).
		WatchesMetadata(
			workloadIdentityMeta,
			metadataEventHandler(securityv1alpha1.SchemeGroupVersion.WithKind("WorkloadIdentity")),
		).
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
			MaxConcurrentReconciles: ptr.Deref(r.Config.ConcurrentSyncs, 0),
			ReconciliationTimeout:   controllerutils.DefaultReconciliationTimeout,
		}).
		Complete(r)
}

// metadataEventHandler returns a typed event handler that enqueues Secret/ConfigMap/WorkloadIdentity
// objects as Requests when they change. Deletes are skipped — there is nothing to annotate on a
// deleted object.
func metadataEventHandler(gvk schema.GroupVersionKind) handler.TypedEventHandler[client.Object, Request] {
	toRequest := func(obj client.Object) Request {
		return Request{
			APIVersion: gvk.GroupVersion().String(),
			Kind:       gvk.Kind,
			Namespace:  obj.GetNamespace(),
			Name:       obj.GetName(),
		}
	}

	return handler.TypedFuncs[client.Object, Request]{
		CreateFunc: func(_ context.Context, e event.TypedCreateEvent[client.Object], q workqueue.TypedRateLimitingInterface[Request]) {
			q.Add(toRequest(e.Object))
		},
		UpdateFunc: func(_ context.Context, e event.TypedUpdateEvent[client.Object], q workqueue.TypedRateLimitingInterface[Request]) {
			q.Add(toRequest(e.ObjectNew))
		},
	}
}

// SeedEventHandler returns an event handler that maps Gardenlet/ManagedSeed events to Requests
// for every credential/resource object referenced in their embedded SeedSpec. On update events the
// union of old and new refs is enqueued so that both newly added and removed refs are reconciled.
func SeedEventHandler[PT client.Object](getConfig func(PT) *runtime.RawExtension) handler.TypedEventHandler[client.Object, Request] {
	refsFor := func(obj client.Object) []Request {
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

	enqueueUnique := func(q workqueue.TypedRateLimitingInterface[Request], refs []Request) {
		seen := sets.New[Request]()
		for _, r := range refs {
			if !seen.Has(r) {
				seen.Insert(r)
				q.Add(r)
			}
		}
	}

	return handler.TypedFuncs[client.Object, Request]{
		CreateFunc: func(_ context.Context, e event.TypedCreateEvent[client.Object], q workqueue.TypedRateLimitingInterface[Request]) {
			enqueueUnique(q, refsFor(e.Object))
		},
		UpdateFunc: func(_ context.Context, e event.TypedUpdateEvent[client.Object], q workqueue.TypedRateLimitingInterface[Request]) {
			enqueueUnique(q, append(refsFor(e.ObjectOld), refsFor(e.ObjectNew)...))
		},
		DeleteFunc: func(_ context.Context, e event.TypedDeleteEvent[client.Object], q workqueue.TypedRateLimitingInterface[Request]) {
			enqueueUnique(q, refsFor(e.Object))
		},
	}
}
