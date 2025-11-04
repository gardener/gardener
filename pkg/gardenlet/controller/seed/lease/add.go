// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package lease

import (
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/gardenlet/controller/lease"
	"github.com/gardener/gardener/pkg/healthz"
)

// AddToManager adds the seed-lease controller to the given manager.
func AddToManager(mgr manager.Manager, gardenCluster cluster.Cluster, seedRESTClient rest.Interface, config gardenletconfigv1alpha1.SeedControllerConfiguration, healthManager healthz.Manager, seedName string) error {
	return (&lease.Reconciler{
		SeedRESTClient: seedRESTClient,
		Config:         config,
		HealthManager:  healthManager,
		SeedName:       seedName,
	}).AddToManager(mgr, gardenCluster, "seed")
}
