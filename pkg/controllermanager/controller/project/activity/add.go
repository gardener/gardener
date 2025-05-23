// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package activity

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/types"
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

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

// ControllerName is the name of this controller.
const ControllerName = "project-activity"

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(mgr manager.Manager) error {
	if r.Client == nil {
		r.Client = mgr.GetClient()
	}
	if r.Clock == nil {
		r.Clock = clock.RealClock{}
	}

	return builder.
		ControllerManagedBy(mgr).
		Named(ControllerName).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: ptr.Deref(r.Config.ConcurrentSyncs, 0),
		}).
		Watches(
			&gardencorev1beta1.Shoot{},
			handler.EnqueueRequestsFromMapFunc(r.MapObjectToProject(mgr.GetLogger().WithValues("controller", ControllerName))),
			builder.WithPredicates(r.OnlyNewlyCreatedObjects(), predicate.GenerationChangedPredicate{}),
		).
		Watches(
			&gardencorev1beta1.BackupEntry{},
			handler.EnqueueRequestsFromMapFunc(r.MapObjectToProject(mgr.GetLogger().WithValues("controller", ControllerName))),
			builder.WithPredicates(r.OnlyNewlyCreatedObjects(), predicate.GenerationChangedPredicate{}),
		).
		Watches(
			&gardencorev1beta1.Quota{},
			handler.EnqueueRequestsFromMapFunc(r.MapObjectToProject(mgr.GetLogger().WithValues("controller", ControllerName))),
			builder.WithPredicates(r.OnlyNewlyCreatedObjects(), r.NeedsSecretOrCredentialsBindingReferenceLabelPredicate()),
		).
		Watches(
			&corev1.Secret{},
			handler.EnqueueRequestsFromMapFunc(r.MapObjectToProject(mgr.GetLogger().WithValues("controller", ControllerName))),
			builder.WithPredicates(r.OnlyNewlyCreatedObjects(), r.NeedsSecretOrCredentialsBindingReferenceLabelPredicate()),
		).
		Complete(r)
}

// OnlyNewlyCreatedObjects filters for objects which are created less than an hour ago for create events. This can be
// used to prevent unnecessary reconciliations in case of controller restarts.
func (r *Reconciler) OnlyNewlyCreatedObjects() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			objMeta, err := meta.Accessor(e.Object)
			if err != nil {
				return false
			}

			return r.Clock.Now().UTC().Sub(objMeta.GetCreationTimestamp().UTC()) <= time.Hour
		},
	}
}

// NeedsSecretOrCredentialsBindingReferenceLabelPredicate returns a predicate which only returns true when the objects have the
// reference.gardener.cloud/secretbinding or reference.gardener.cloud/credentialsbinding label.
func (r *Reconciler) NeedsSecretOrCredentialsBindingReferenceLabelPredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			objMeta, err := meta.Accessor(e.Object)
			if err != nil {
				return false
			}

			_, hasSecretBindingLabel := objMeta.GetLabels()[v1beta1constants.LabelSecretBindingReference]
			_, hasCredentialsBindingLabel := objMeta.GetLabels()[v1beta1constants.LabelCredentialsBindingReference]
			return hasSecretBindingLabel || hasCredentialsBindingLabel
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldObjMeta, err := meta.Accessor(e.ObjectOld)
			if err != nil {
				return false
			}

			objMeta, err := meta.Accessor(e.ObjectNew)
			if err != nil {
				return false
			}

			_, oldObjHasSecretBindingLabel := oldObjMeta.GetLabels()[v1beta1constants.LabelSecretBindingReference]
			_, newObjHasSecretBindingLabel := objMeta.GetLabels()[v1beta1constants.LabelSecretBindingReference]
			_, oldObjHasCredentialsBindingLabel := oldObjMeta.GetLabels()[v1beta1constants.LabelCredentialsBindingReference]
			_, newObjHasCredentialsBindingLabel := objMeta.GetLabels()[v1beta1constants.LabelCredentialsBindingReference]

			return oldObjHasSecretBindingLabel ||
				newObjHasSecretBindingLabel ||
				oldObjHasCredentialsBindingLabel ||
				newObjHasCredentialsBindingLabel
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			objMeta, err := meta.Accessor(e.Object)
			if err != nil {
				return false
			}

			_, hasSecretBindingLabel := objMeta.GetLabels()[v1beta1constants.LabelSecretBindingReference]
			_, hasCredentialsBindingLabel := objMeta.GetLabels()[v1beta1constants.LabelCredentialsBindingReference]
			return hasSecretBindingLabel || hasCredentialsBindingLabel
		},
	}
}

// MapObjectToProject is a handler.MapFunc for mapping an object to the Project it belongs to.
func (r *Reconciler) MapObjectToProject(log logr.Logger) handler.MapFunc {
	return func(ctx context.Context, obj client.Object) []reconcile.Request {
		project, err := gardenerutils.ProjectForNamespaceFromReader(ctx, r.Client, obj.GetNamespace())
		if err != nil {
			if !apierrors.IsNotFound(err) {
				log.Error(err, "Failed to get project for namespace", "namespace", obj.GetNamespace())
			}
			return nil
		}

		return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: project.Name}}}
	}
}
