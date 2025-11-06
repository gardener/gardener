// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package lease

import (
	"k8s.io/client-go/rest"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	predicateutils "github.com/gardener/gardener/pkg/controllerutils/predicate"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/gardenlet/controller/lease"
	"github.com/gardener/gardener/pkg/healthz"
)

// AddToManager adds the seed-lease controller to the given manager.
func AddToManager(
	mgr manager.Manager,
	gardenCluster cluster.Cluster,
	seedRESTClient rest.Interface,
	config gardenletconfigv1alpha1.SeedControllerConfiguration,
	healthManager healthz.Manager,
	seedName string,
	clock clock.Clock,
	leaseNamespace *string,
) error {
	return (&lease.Reconciler{
		RuntimeRESTClient: seedRESTClient,
		HealthManager:     healthManager,
		Clock:             clock,

		NewObjectFunc: func() client.Object { return &gardencorev1beta1.Seed{} },
		GetObjectConditions: func(obj client.Object) []gardencorev1beta1.Condition {
			return obj.(*gardencorev1beta1.Seed).Status.Conditions
		},
		SetObjectConditions: func(obj client.Object, conditions []gardencorev1beta1.Condition) {
			obj.(*gardencorev1beta1.Seed).Status.Conditions = conditions
		},

		LeaseNamespace:     leaseNamespace,
		LeaseResyncSeconds: *config.LeaseResyncSeconds,
	}).AddToManager(mgr, gardenCluster, "seed", predicateutils.HasName(seedName))
}
