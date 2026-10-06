// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package inplaceupdate

import (
	"context"

	kubernetesclientset "k8s.io/client-go/kubernetes"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/apis/config/gardenlet/v1alpha1"
)

// Reconciler orchestrates in-place updates for all nodes in a worker pool.
type Reconciler struct {
	ShootClient         client.Client
	ShootClientSet      kubernetesclientset.Interface
	GardenClient        client.Client
	ShootNamespacedName client.ObjectKey
	Clock               clock.Clock
	Config              gardenletconfigv1alpha1.ShootInPlaceUpdateControllerConfiguration
}

// Reconcile processes all in-place-update state for a single worker pool.
func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	return reconcile.Result{}, nil
}
