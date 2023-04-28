// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package networkpolicy

import (
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/controller/networkpolicy"
	"github.com/gardener/gardener/pkg/controller/networkpolicy/hostnameresolver"
	"github.com/gardener/gardener/pkg/controllerutils/mapper"
	"github.com/gardener/gardener/pkg/extensions"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
)

// AddToManager adds all Seed controllers to the given manager.
func AddToManager(
	mgr manager.Manager,
	seedCluster cluster.Cluster,
	cfg config.GardenletConfiguration,
	resolver hostnameresolver.HostResolver,
) error {
	reconciler := &networkpolicy.Reconciler{
		ConcurrentSyncs: cfg.Controllers.NetworkPolicy.ConcurrentSyncs,
		Resolver:        resolver,
		RuntimeNetworks: networkpolicy.RuntimeNetworkConfig{
			Pods:       cfg.SeedConfig.Spec.Networks.Pods,
			Services:   cfg.SeedConfig.Spec.Networks.Services,
			Nodes:      cfg.SeedConfig.Spec.Networks.Nodes,
			BlockCIDRs: cfg.SeedConfig.Spec.Networks.BlockCIDRs,
		},
	}

	reconciler.WatchRegisterers = append(reconciler.WatchRegisterers, func(c controller.Controller) error {
		return c.Watch(
			source.NewKindWithCache(&extensionsv1alpha1.Cluster{}, seedCluster.GetCache()),
			mapper.EnqueueRequestsFrom(mapper.MapFunc(reconciler.MapObjectToName), mapper.UpdateWithNew, mgr.GetLogger()),
			ClusterPredicate(),
		)
	})

	return reconciler.AddToManager(mgr, seedCluster)
}

// ClusterPredicate is a predicate which returns 'true' when the network CIDRs of a shoot cluster change.
func ClusterPredicate() predicate.Predicate {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			cluster, ok := e.ObjectNew.(*extensionsv1alpha1.Cluster)
			if !ok {
				return false
			}
			shoot, err := extensions.ShootFromCluster(cluster)
			if err != nil || shoot == nil {
				return false
			}

			oldCluster, ok := e.ObjectOld.(*extensionsv1alpha1.Cluster)
			if !ok {
				return false
			}
			oldShoot, err := extensions.ShootFromCluster(oldCluster)
			if err != nil || oldShoot == nil {
				return false
			}

			// if the shoot has no networking field, return false
			if shoot.Spec.Networking == nil {
				return false
			}

			if v1beta1helper.IsWorkerless(shoot) {
				// if the shoot has networking field set and the old shoot has nil, then we cannot compare services, so return true right away
				return oldShoot.Spec.Networking == nil || !pointer.StringEqual(shoot.Spec.Networking.Services, oldShoot.Spec.Networking.Services)
			}

			return !pointer.StringEqual(shoot.Spec.Networking.Pods, oldShoot.Spec.Networking.Pods) ||
				!pointer.StringEqual(shoot.Spec.Networking.Services, oldShoot.Spec.Networking.Services) ||
				!pointer.StringEqual(shoot.Spec.Networking.Nodes, oldShoot.Spec.Networking.Nodes)
		},
		CreateFunc:  func(event.CreateEvent) bool { return false },
		DeleteFunc:  func(event.DeleteEvent) bool { return false },
		GenericFunc: func(event.GenericEvent) bool { return false },
	}
}
