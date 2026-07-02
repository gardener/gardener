// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package imagepullsecret

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
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/controllerutils/mapper"
	predicateutils "github.com/gardener/gardener/pkg/controllerutils/predicate"
	"github.com/gardener/gardener/pkg/utils"
)

// ControllerName is the name of this controller.
const ControllerName = "seed-image-pull-secret"

var imagePullSecretSelector = labels.NewSelector().Add(
	utils.MustNewRequirement(v1beta1constants.GardenRole, selection.Equals, v1beta1constants.GardenRoleImagePullSecret),
)

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
		For(&corev1.Secret{},
			builder.WithPredicates(r.ImagePullSecretPredicate()),
		).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 5,
			ReconciliationTimeout:   controllerutils.DefaultReconciliationTimeout,
		}).
		Watches(
			&gardencorev1beta1.Seed{},
			handler.EnqueueRequestsFromMapFunc(r.MapToAllImagePullSecrets(mgr.GetLogger().WithValues("controller", ControllerName))),
			builder.WithPredicates(predicateutils.ForEventTypes(predicateutils.Create)),
		).
		Complete(r)
}

// ImagePullSecretPredicate returns true for secrets in the garden namespace with the
// gardener.cloud/role=image-pull-secret label.
func (r *Reconciler) ImagePullSecretPredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return r.IsImagePullSecret(e.Object)
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			if !r.IsImagePullSecret(e.ObjectNew) {
				return false
			}
			oldSecret, ok1 := e.ObjectOld.(*corev1.Secret)
			newSecret, ok2 := e.ObjectNew.(*corev1.Secret)
			if !ok1 || !ok2 {
				return true
			}
			return !apiequality.Semantic.DeepEqual(oldSecret, newSecret)
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return r.IsImagePullSecret(e.Object)
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return r.IsImagePullSecret(e.Object)
		},
	}
}

// IsImagePullSecret reports whether obj is an image pull secret in the garden namespace.
func (r *Reconciler) IsImagePullSecret(obj client.Object) bool {
	return obj.GetNamespace() == r.GardenNamespace &&
		imagePullSecretSelector.Matches(labels.Set(obj.GetLabels()))
}

// MapToAllImagePullSecrets returns reconcile.Request objects for all image pull secrets in the
// garden namespace. Used to trigger reconciliation when a new Seed is created.
func (r *Reconciler) MapToAllImagePullSecrets(log logr.Logger) handler.MapFunc {
	return func(ctx context.Context, _ client.Object) []reconcile.Request {
		secretList := &metav1.PartialObjectMetadataList{}
		secretList.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("SecretList"))
		if err := r.Client.List(ctx, secretList,
			client.InNamespace(r.GardenNamespace),
			client.MatchingLabelsSelector{Selector: imagePullSecretSelector},
		); err != nil {
			log.Error(err, "Failed to list image pull secrets")
			return nil
		}
		return mapper.ObjectListToRequests(secretList)
	}
}
