// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package secrets

import (
	"context"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
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
	"github.com/gardener/gardener/pkg/controllerutils/mapper"
	predicateutils "github.com/gardener/gardener/pkg/controllerutils/predicate"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

// ControllerName is the name of this controller.
const ControllerName = "seed-secrets"

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(mgr manager.Manager) error {
	if r.Client == nil {
		r.Client = mgr.GetClient()
	}
	if r.GardenNamespace == "" {
		r.GardenNamespace = v1beta1constants.GardenNamespace
	}

	return builder.
		ControllerManagedBy(mgr).
		Named(ControllerName).
		For(&gardencorev1beta1.Seed{}, builder.WithPredicates(predicateutils.ForEventTypes(predicateutils.Create))).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 5,
		}).
		Watches(
			&corev1.Secret{},
			handler.EnqueueRequestsFromMapFunc(r.MapToAllSeeds(mgr.GetLogger().WithValues("controller", ControllerName))),
			builder.WithPredicates(r.GardenSecretPredicate(), r.SecretPredicate()),
		).
		Complete(r)
}

var (
	gardenRoleReq      = utils.MustNewRequirement(v1beta1constants.GardenRole, selection.Exists)
	gardenRoleSelector = labels.NewSelector().Add(gardenRoleReq).Add(gardenerutils.NoControlPlaneSecretsReq)
)

// GardenSecretPredicate returns true for all events when the respective secret is in the garden namespace and has a
// gardener.cloud/role label.
func (r *Reconciler) GardenSecretPredicate() predicate.Predicate {
	return predicate.NewPredicateFuncs(func(obj client.Object) bool {
		secret, ok := obj.(*corev1.Secret)
		if !ok {
			return false
		}

		return secret.Namespace == r.GardenNamespace &&
			gardenRoleSelector.Matches(labels.Set(secret.Labels))
	})
}

// SecretPredicate returns true for all events. For 'UPDATE' events, it only returns true when the secret has changed.
func (r *Reconciler) SecretPredicate() predicate.Predicate {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			secret, ok := e.ObjectNew.(*corev1.Secret)
			if !ok {
				return false
			}

			oldSecret, ok := e.ObjectOld.(*corev1.Secret)
			if !ok {
				return false
			}

			return !apiequality.Semantic.DeepEqual(oldSecret, secret)
		},
	}
}

// MapToAllSeeds returns reconcile.Request objects for all existing seeds in the system.
func (r *Reconciler) MapToAllSeeds(log logr.Logger) handler.MapFunc {
	return func(ctx context.Context, _ client.Object) []reconcile.Request {
		seedList := &metav1.PartialObjectMetadataList{}
		seedList.SetGroupVersionKind(gardencorev1beta1.SchemeGroupVersion.WithKind("SeedList"))
		if err := r.Client.List(ctx, seedList); err != nil {
			log.Error(err, "Failed to list seeds")
			return nil
		}

		return mapper.ObjectListToRequests(seedList)
	}
}
