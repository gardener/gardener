// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package namespacedcloudprofile

import (
	"context"

	"github.com/go-logr/logr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/controllerutils/mapper"
)

// ControllerName is the name of this controller.
const ControllerName = "namespacedcloudprofile"

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(ctx context.Context, mgr manager.Manager) error {
	if r.Client == nil {
		r.Client = mgr.GetClient()
	}
	if r.Recorder == nil {
		r.Recorder = mgr.GetEventRecorderFor(ControllerName + "-controller")
	}

	c, err := builder.
		ControllerManagedBy(mgr).
		Named(ControllerName).
		For(&gardencorev1beta1.NamespacedCloudProfile{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: ptr.Deref(r.Config.ConcurrentSyncs, 0),
		}).
		Build(r)
	if err != nil {
		return err
	}

	return c.Watch(
		source.Kind(mgr.GetCache(), &gardencorev1beta1.CloudProfile{}),
		mapper.EnqueueRequestsFrom(ctx, mgr.GetCache(), mapper.MapFunc(r.MapCloudProfileToNamespacedCloudProfile), mapper.UpdateWithNew, c.GetLogger()),
	)
}

// MapCloudProfileToNamespacedCloudProfile is a mapper.MapFunc for mapping a core.gardener.cloud/v1beta1.CloudProfile to core.gardener.cloud/v1beta1.NamespacedCloudProfile.
func (r *Reconciler) MapCloudProfileToNamespacedCloudProfile(ctx context.Context, log logr.Logger, _ client.Reader, obj client.Object) []reconcile.Request {
	cloudProfile, ok := obj.(*gardencorev1beta1.CloudProfile)
	if !ok {
		return nil
	}
	namespacedCloudProfileList, err := controllerutils.GetNamespacedCloudProfilesReferencingCloudProfile(ctx, r.Client, cloudProfile.Name)
	if err != nil {
		log.Error(err, "Failed to list NamespacedCloudProfiles referencing this CloudProfile", "cloudProfileName", cloudProfile.Name)
		return nil
	}
	return mapper.ObjectListToRequests(namespacedCloudProfileList)
}
