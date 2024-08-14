// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package serviceaccount

import (
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/utils/clock"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/controller/tokenrequestor"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

// AddToManager adds the controller to the given manager.
func AddToManager(mgr manager.Manager, seedCluster, gardenCluster cluster.Cluster, seedName string, cfg config.TokenRequestorServiceAccountControllerConfiguration) error {
	return (&tokenrequestor.Reconciler{
		ConcurrentSyncs: ptr.Deref(cfg.ConcurrentSyncs, 0),
		Clock:           clock.RealClock{},
		JitterFunc:      wait.Jitter,
		Class:           ptr.To(resourcesv1alpha1.ResourceManagerClassGarden),
		TargetNamespace: gardenerutils.ComputeGardenNamespace(seedName),
	}).AddToManager(mgr, seedCluster, gardenCluster)
}
