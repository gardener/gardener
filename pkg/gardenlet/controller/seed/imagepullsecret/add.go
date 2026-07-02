// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package imagepullsecret

import (
	"context"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/controllerutils/mapper"
	predicateutils "github.com/gardener/gardener/pkg/controllerutils/predicate"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

// ControllerName is the name of this controller.
const ControllerName = "seed-image-pull-secret"

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(mgr manager.Manager, gardenCluster cluster.Cluster, seedCluster cluster.Cluster) error {
	if r.GardenClient == nil {
		r.GardenClient = gardenCluster.GetClient()
	}
	if r.SeedClient == nil {
		r.SeedClient = seedCluster.GetClient()
	}

	seedNamespace := gardenerutils.ComputeGardenNamespace(r.SeedName)

	return builder.
		ControllerManagedBy(mgr).
		Named(ControllerName).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 1,
		}).
		// Watch image pull secrets in this seed's scoped namespace (seed-<name>) on the garden cluster.
		// gardener-controller-manager copies secrets there based on the annotation, and the gardenlet
		// is unconditionally authorized to list/watch its seed-<name> namespace.
		WatchesRawSource(source.Kind[client.Object](
			gardenCluster.GetCache(),
			&corev1.Secret{},
			&handler.EnqueueRequestForObject{},
			r.ImagePullSecretPredicate(seedNamespace),
			predicateutils.ForEventTypes(predicateutils.Create, predicateutils.Update, predicateutils.Delete),
		)).
		// Watch for Namespace creation in the seed cluster to propagate existing secrets to new namespaces.
		WatchesRawSource(source.Kind[client.Object](
			seedCluster.GetCache(),
			&corev1.Namespace{},
			handler.EnqueueRequestsFromMapFunc(r.MapToAllImagePullSecrets(mgr.GetLogger().WithValues("controller", ControllerName))),
			r.TargetNamespacePredicate(),
			predicateutils.ForEventTypes(predicateutils.Create),
		)).
		Complete(r)
}

// ImagePullSecretPredicate returns true for secrets in the seed-scoped namespace with the image-pull-secret role.
func (r *Reconciler) ImagePullSecretPredicate(seedNamespace string) predicate.Predicate {
	return predicate.NewPredicateFuncs(func(obj client.Object) bool {
		return obj.GetNamespace() == seedNamespace &&
			obj.GetLabels()[v1beta1constants.GardenRole] == v1beta1constants.GardenRoleImagePullSecret
	})
}

// TargetNamespacePredicate returns true for namespaces with the extension or shoot role.
func (r *Reconciler) TargetNamespacePredicate() predicate.Predicate {
	return predicate.NewPredicateFuncs(func(obj client.Object) bool {
		role := obj.GetLabels()[v1beta1constants.GardenRole]
		return role == v1beta1constants.GardenRoleExtension || role == v1beta1constants.GardenRoleShoot
	})
}

// MapToAllImagePullSecrets returns reconcile.Request objects for all image pull secrets in the
// seed cluster's garden namespace. Used to trigger reconciliation when a new namespace is created.
func (r *Reconciler) MapToAllImagePullSecrets(log logr.Logger) handler.MapFunc {
	return func(ctx context.Context, _ client.Object) []reconcile.Request {
		secretList := &corev1.SecretList{}
		if err := r.SeedClient.List(ctx, secretList,
			client.InNamespace(v1beta1constants.GardenNamespace),
			client.MatchingLabels{v1beta1constants.GardenRole: v1beta1constants.GardenRoleImagePullSecret},
		); err != nil {
			log.Error(err, "Failed to list image pull secrets")
			return nil
		}
		return mapper.ObjectListToRequests(secretList)
	}
}
