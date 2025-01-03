// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package tokeninvalidator

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
)

// ControllerName is the name of the controller.
const ControllerName = "token-invalidator"

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(mgr manager.Manager, targetCluster cluster.Cluster) error {
	if r.TargetReader == nil {
		r.TargetReader = targetCluster.GetAPIReader()
	}
	if r.TargetClient == nil {
		r.TargetClient = targetCluster.GetClient()
	}

	secret := &metav1.PartialObjectMetadata{}
	secret.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("Secret"))

	return builder.
		ControllerManagedBy(mgr).
		Named(ControllerName).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 1,
			RateLimiter:             r.RateLimiter,
		}).
		WatchesRawSource(source.Kind[client.Object](
			targetCluster.GetCache(),
			secret,
			&handler.EnqueueRequestForObject{},
			r.SecretPredicate(),
		)).
		WatchesRawSource(source.Kind[client.Object](
			targetCluster.GetCache(),
			&corev1.ServiceAccount{},
			handler.EnqueueRequestsFromMapFunc(r.MapServiceAccountToSecrets),
			r.ServiceAccountPredicate(),
		)).
		Complete(r)
}

// SecretPredicate returns the predicate for secrets.
func (r *Reconciler) SecretPredicate() predicate.Predicate {
	isRelevantSecret := func(obj client.Object) bool {
		return obj.GetAnnotations()[corev1.ServiceAccountNameKey] != ""
	}

	return predicate.Funcs{
		CreateFunc:  func(e event.CreateEvent) bool { return isRelevantSecret(e.Object) },
		UpdateFunc:  func(e event.UpdateEvent) bool { return isRelevantSecret(e.ObjectNew) },
		DeleteFunc:  func(_ event.DeleteEvent) bool { return false },
		GenericFunc: func(_ event.GenericEvent) bool { return false },
	}
}

// ServiceAccountPredicate returns the predicate for service accounts.
func (r *Reconciler) ServiceAccountPredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(_ event.CreateEvent) bool { return false },
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldSA, ok := e.ObjectOld.(*corev1.ServiceAccount)
			if !ok {
				return false
			}

			newSA, ok := e.ObjectNew.(*corev1.ServiceAccount)
			if !ok {
				return false
			}

			return !apiequality.Semantic.DeepEqual(oldSA.AutomountServiceAccountToken, newSA.AutomountServiceAccountToken) ||
				oldSA.Labels[resourcesv1alpha1.StaticTokenSkip] != newSA.Labels[resourcesv1alpha1.StaticTokenSkip]
		},
		DeleteFunc:  func(_ event.DeleteEvent) bool { return false },
		GenericFunc: func(_ event.GenericEvent) bool { return false },
	}
}

// MapServiceAccountToSecrets maps the ServiceAccount to all referenced secrets.
func (r *Reconciler) MapServiceAccountToSecrets(_ context.Context, obj client.Object) []reconcile.Request {
	sa, ok := obj.(*corev1.ServiceAccount)
	if !ok {
		return nil
	}

	out := make([]reconcile.Request, 0, len(sa.Secrets))

	for _, secretRef := range sa.Secrets {
		out = append(out, reconcile.Request{NamespacedName: types.NamespacedName{
			Name:      secretRef.Name,
			Namespace: sa.Namespace,
		}})
	}

	return out
}
