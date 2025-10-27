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
	"github.com/gardener/gardener/pkg/gardenlet/controller/lease"
	"github.com/gardener/gardener/pkg/healthz"
	"github.com/gardener/gardener/pkg/utils/gardener/gardenlet"
)

// AddToManager adds the shoot-lease controller to the given manager.
func AddToManager(mgr manager.Manager, gardenCluster cluster.Cluster, shootRESTClient rest.Interface, healthManager healthz.Manager, clock clock.Clock) error {
	return (&lease.Reconciler{
		RuntimeRESTClient: shootRESTClient,
		HealthManager:     healthManager,
		Clock:             clock,

		NewObjectFunc: func() client.Object { return &gardencorev1beta1.Shoot{} },
		GetObjectConditions: func(obj client.Object) []gardencorev1beta1.Condition {
			return obj.(*gardencorev1beta1.Shoot).Status.Conditions
		},
		SetObjectConditions: func(obj client.Object, conditions []gardencorev1beta1.Condition) {
			obj.(*gardencorev1beta1.Shoot).Status.Conditions = conditions
		},

		LeaseNamePrefix:    gardenlet.ResourcePrefixSelfHostedShoot,
		LeaseResyncSeconds: 2,
	}).AddToManager(mgr, gardenCluster, "shoot")
}
