// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package seed

import (
	"fmt"

	"github.com/Masterminds/semver/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/clock"
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
	predicateutils "github.com/gardener/gardener/pkg/controllerutils/predicate"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// ControllerName is the name of this controller.
const ControllerName = "seed"

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(mgr manager.Manager, gardenCluster cluster.Cluster) error {
	if r.GardenClient == nil {
		r.GardenClient = gardenCluster.GetClient()
	}
	if r.SeedVersion == nil {
		var err error
		r.SeedVersion, err = semver.NewVersion(r.SeedClientSet.Version())
		if err != nil {
			return err
		}
	}
	if r.Clock == nil {
		r.Clock = clock.RealClock{}
	}
	if r.Recorder == nil {
		r.Recorder = gardenCluster.GetEventRecorderFor(ControllerName + "-controller")
	}
	if r.GardenNamespace == "" {
		r.GardenNamespace = v1beta1constants.GardenNamespace
	}

	if r.ClientCertificateExpirationTimestamp == nil {
		gardenletClientCertificate, err := kubernetesutils.ClientCertificateFromRESTConfig(gardenCluster.GetConfig())
		if err != nil {
			return fmt.Errorf("failed to get gardenlet client certificate: %w", err)
		}
		r.ClientCertificateExpirationTimestamp = &metav1.Time{Time: gardenletClientCertificate.Leaf.NotAfter}
	}
	mgr.GetLogger().WithValues("controller", ControllerName).Info("The client certificate used to communicate with the garden cluster has expiration date", "expirationDate", r.ClientCertificateExpirationTimestamp)

	return builder.
		ControllerManagedBy(mgr).
		Named(ControllerName).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 1,
		}).
		WatchesRawSource(source.Kind[client.Object](
			gardenCluster.GetCache(),
			&gardencorev1beta1.Seed{},
			&handler.EnqueueRequestForObject{},
			predicateutils.HasName(r.Config.SeedConfig.Name),
			predicate.GenerationChangedPredicate{},
		)).
		Complete(r)
}
