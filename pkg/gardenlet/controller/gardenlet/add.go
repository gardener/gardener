// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardenlet

import (
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controller/gardenletdeployer"
	predicateutils "github.com/gardener/gardener/pkg/controllerutils/predicate"
	"github.com/gardener/gardener/pkg/utils/oci"
)

// ControllerName is the name of this controller.
const ControllerName = "gardenlet"

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(
	mgr manager.Manager,
	gardenCluster cluster.Cluster,
	seedClientSet kubernetes.Interface,
) error {
	if r.GardenClient == nil {
		r.GardenClient = gardenCluster.GetClient()
	}
	if r.GardenRESTConfig == nil {
		r.GardenRESTConfig = gardenCluster.GetConfig()
	}
	if r.SeedClientSet == nil {
		r.SeedClientSet = seedClientSet
	}
	if r.Recorder == nil {
		r.Recorder = gardenCluster.GetEventRecorderFor(ControllerName + "-controller")
	}
	if r.Clock == nil {
		r.Clock = clock.RealClock{}
	}
	if r.GardenNamespace == "" {
		r.GardenNamespace = v1beta1constants.GardenNamespace
	}
	if r.HelmRegistry == nil {
		r.HelmRegistry = oci.NewHelmRegistry(r.GardenClient)
	}
	if r.ValuesHelper == nil {
		r.ValuesHelper = gardenletdeployer.NewValuesHelper(&r.Config)
	}

	return builder.
		ControllerManagedBy(mgr).
		Named(ControllerName).
		WithOptions(controller.Options{
			// There can only be one Gardenlet object relevant for an instance of gardenlet, so it's enough to have one
			// worker only.
			MaxConcurrentReconciles: 1,
		}).
		WatchesRawSource(
			source.Kind[client.Object](gardenCluster.GetCache(),
				&seedmanagementv1alpha1.Gardenlet{},
				&handler.EnqueueRequestForObject{},
				predicate.GenerationChangedPredicate{},
				predicateutils.ForEventTypes(predicateutils.Create, predicateutils.Update)),
		).
		Complete(r)
}
