// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"context"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils/mapper"
	predicateutils "github.com/gardener/gardener/pkg/controllerutils/predicate"
	"github.com/gardener/gardener/pkg/extensions"
	"github.com/gardener/gardener/pkg/gardenlet/controller/networkpolicy/hostnameresolver"
)

// ControllerName is the name of this controller.
const ControllerName = "networkpolicy"

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(mgr manager.Manager, gardenCluster, seedCluster cluster.Cluster) error {
	if r.GardenClient == nil {
		r.GardenClient = gardenCluster.GetClient()
	}
	if r.RuntimeClient == nil {
		r.RuntimeClient = seedCluster.GetClient()
	}
	if r.Resolver == nil {
		resolver, err := hostnameresolver.CreateForCluster(seedCluster.GetConfig(), mgr.GetLogger())
		if err != nil {
			return fmt.Errorf("failed to get hostnameresolver: %w", err)
		}
		resolverUpdate := make(chan event.GenericEvent)
		resolver.WithCallback(func() { resolverUpdate <- event.GenericEvent{} })
		if err := mgr.Add(resolver); err != nil {
			return fmt.Errorf("failed to add hostnameresolver to manager: %w", err)
		}
		r.Resolver = resolver
		r.ResolverUpdate = resolverUpdate
	}
	if r.ResolverUpdate == nil {
		r.ResolverUpdate = make(chan event.GenericEvent)
	}

	c, err := builder.
		ControllerManagedBy(mgr).
		Named(ControllerName).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: pointer.IntDeref(r.Config.ConcurrentSyncs, 0),
		}).
		Watches(
			source.NewKindWithCache(&corev1.Namespace{}, seedCluster.GetCache()),
			&handler.EnqueueRequestForObject{},
			builder.WithPredicates(predicateutils.ForEventTypes(predicateutils.Create, predicateutils.Update)),
		).
		Build(r)
	if err != nil {
		return err
	}

	if err := c.Watch(
		source.NewKindWithCache(&corev1.Endpoints{}, seedCluster.GetCache()),
		mapper.EnqueueRequestsFrom(mapper.MapFunc(r.MapToNamespaces), mapper.UpdateWithNew, c.GetLogger()),
		r.IsKubernetesEndpoint(),
	); err != nil {
		return err
	}

	if err := c.Watch(
		source.NewKindWithCache(&networkingv1.NetworkPolicy{}, seedCluster.GetCache()),
		mapper.EnqueueRequestsFrom(mapper.MapFunc(r.MapObjectToNamespace), mapper.UpdateWithNew, c.GetLogger()),
		r.NetworkPolicyPredicate(),
	); err != nil {
		return err
	}

	if err := c.Watch(
		source.NewKindWithCache(&extensionsv1alpha1.Cluster{}, seedCluster.GetCache()),
		mapper.EnqueueRequestsFrom(mapper.MapFunc(r.MapObjectToName), mapper.UpdateWithNew, mgr.GetLogger()),
		r.ClusterPredicate(),
	); err != nil {
		return err
	}

	return c.Watch(
		&source.Channel{Source: r.ResolverUpdate},
		mapper.EnqueueRequestsFrom(mapper.MapFunc(r.MapToNamespaces), mapper.UpdateWithNew, c.GetLogger()),
	)
}

// NetworkPolicyPredicate is a predicate which returns true in case the network policy name matches with one of those
// managed by this reconciler.
func (r *Reconciler) NetworkPolicyPredicate() predicate.Predicate {
	var (
		configs    = r.networkPolicyConfigs()
		predicates = make([]predicate.Predicate, 0, len(configs))
	)

	for _, config := range configs {
		predicates = append(predicates, predicateutils.HasName(config.name))
	}

	return predicate.Or(predicates...)
}

// ClusterPredicate is a predicate which returns 'true' when the network CIDRs of a shoot cluster change.
func (r *Reconciler) ClusterPredicate() predicate.Predicate {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			cluster, ok := e.ObjectNew.(*extensionsv1alpha1.Cluster)
			if !ok {
				return false
			}
			shoot, err := extensions.ShootFromCluster(cluster)
			if err != nil {
				return false
			}

			oldCluster, ok := e.ObjectOld.(*extensionsv1alpha1.Cluster)
			if !ok {
				return false
			}
			oldShoot, err := extensions.ShootFromCluster(oldCluster)
			if err != nil {
				return false
			}

			// if the shoot has no networking field, nothing to do here
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

// MapToNamespaces is a mapper function which returns requests for all relevant namespaces.
func (r *Reconciler) MapToNamespaces(ctx context.Context, log logr.Logger, _ client.Reader, _ client.Object) []reconcile.Request {
	var selectors []labels.Selector
	for _, config := range r.networkPolicyConfigs() {
		selectors = append(selectors, config.namespaceSelectors...)
	}

	namespaceList := &corev1.NamespaceList{}
	if err := r.RuntimeClient.List(ctx, namespaceList); err != nil {
		log.Error(err, "Unable to list all namespaces")
		return nil
	}

	namespaceNames := sets.New[string]()
	for _, namespace := range namespaceList.Items {
		if labelsMatchAnySelector(namespace.Labels, selectors) {
			namespaceNames.Insert(namespace.Name)
		}
	}

	var requests []reconcile.Request
	for _, namespaceName := range namespaceNames.UnsortedList() {
		requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{Name: namespaceName}})
	}
	return requests
}

// MapObjectToName is a mapper function which maps an object to its name.
func (r *Reconciler) MapObjectToName(_ context.Context, _ logr.Logger, _ client.Reader, obj client.Object) []reconcile.Request {
	return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: obj.GetName()}}}
}

// MapObjectToNamespace is a mapper function which maps an object to its namespace.
func (r *Reconciler) MapObjectToNamespace(_ context.Context, _ logr.Logger, _ client.Reader, obj client.Object) []reconcile.Request {
	return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: obj.GetNamespace()}}}
}

// IsKubernetesEndpoint returns a predicate which evaluates if the object is the kubernetes endpoint.
func (r *Reconciler) IsKubernetesEndpoint() predicate.Predicate {
	return predicate.NewPredicateFuncs(func(obj client.Object) bool {
		return obj.GetNamespace() == corev1.NamespaceDefault && obj.GetName() == "kubernetes"
	})
}
