// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardenlet

import (
	"context"
	"fmt"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/utils/clock"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils"
	predicateutils "github.com/gardener/gardener/pkg/controllerutils/predicate"
	gardenletutils "github.com/gardener/gardener/pkg/utils/gardener/gardenlet"
	"github.com/gardener/gardener/pkg/utils/oci"
)

// ControllerName is the name of this controller.
const ControllerName = "gardenlet"

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(ctx context.Context, mgr manager.Manager, virtualCluster cluster.Cluster) error {
	if r.RuntimeCluster == nil {
		r.RuntimeCluster = mgr
	}
	if r.VirtualConfig == nil {
		r.VirtualConfig = virtualCluster.GetConfig()
	}
	if r.VirtualClient == nil {
		r.VirtualClient = virtualCluster.GetClient()
	}
	if r.Clock == nil {
		r.Clock = clock.RealClock{}
	}
	if r.GardenNamespace == "" {
		r.GardenNamespace = v1beta1constants.GardenNamespace
	}
	if r.DefaultGardenClusterAddress == "" {
		return fmt.Errorf("DefaultGardenClusterAddress address is not set")
	}
	if r.GardenNamespaceTarget == "" {
		r.GardenNamespaceTarget = v1beta1constants.GardenNamespace
	}
	if r.Recorder == nil {
		r.Recorder = mgr.GetEventRecorderFor(ControllerName + "-controller")
	}
	if r.HelmRegistry == nil {
		r.HelmRegistry = oci.NewHelmRegistry(r.RuntimeCluster.GetClient())
	}

	return builder.
		ControllerManagedBy(mgr).
		Named(ControllerName).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: pointer.IntDeref(r.Config.ConcurrentSyncs, 0),
			ReconciliationTimeout:   controllerutils.DefaultReconciliationTimeout,
		}).
		WatchesRawSource(source.Kind[client.Object](virtualCluster.GetCache(), &seedmanagementv1alpha1.Gardenlet{},
			&handler.EnqueueRequestForObject{},
			predicateutils.ForEventTypes(predicateutils.Create, predicateutils.Update),
			&predicate.GenerationChangedPredicate{},
			r.OperatorResponsiblePredicate(ctx),
		)).
		Complete(r)
}

// OperatorResponsiblePredicate is a predicate for checking whether the Seed object has already been created for the
// Gardenlet resource, and whether the kubeconfig secret ref has been removed. It also returns 'true' if the
// 'force-redeploy' operation annotation is set, even though the Seed object already exists.
func (r *Reconciler) OperatorResponsiblePredicate(ctx context.Context) predicate.Predicate {
	return predicate.NewPredicateFuncs(func(obj client.Object) bool {
		gardenlet, ok := obj.(*seedmanagementv1alpha1.Gardenlet)
		if !ok {
			return false
		}
		return !strings.HasPrefix(gardenlet.Name, gardenletutils.ResourcePrefixSelfHostedShoot) &&
			(hasForceRedeployOperationAnnotation(gardenlet) ||
				r.seedDoesNotExist(ctx, gardenlet) ||
				gardenlet.Spec.KubeconfigSecretRef != nil)
	})
}

func (r *Reconciler) seedDoesNotExist(ctx context.Context, gardenlet *seedmanagementv1alpha1.Gardenlet) bool {
	return apierrors.IsNotFound(r.VirtualClient.Get(ctx, client.ObjectKey{Name: gardenlet.Name}, &gardencorev1beta1.Seed{}))
}

func hasForceRedeployOperationAnnotation(gardenlet *seedmanagementv1alpha1.Gardenlet) bool {
	return gardenlet.Annotations[v1beta1constants.GardenerOperation] == v1beta1constants.OperationForceRedeploy
}
