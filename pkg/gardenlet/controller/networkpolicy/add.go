// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/gardener/gardener/pkg/controllerutils/mapper"
	predicateutils "github.com/gardener/gardener/pkg/controllerutils/predicate"
	"github.com/gardener/gardener/pkg/gardenlet/controller/networkpolicy/hostnameresolver"
)

// ControllerName is the name of this controller.
const ControllerName = "networkpolicy"

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(mgr manager.Manager, seedCluster cluster.Cluster) error {
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

	// It's not possible to overwrite the event handler when using the controller builder. Hence, we have to build up
	// the controller manually.
	c, err := controller.New(
		ControllerName,
		mgr,
		controller.Options{
			Reconciler:              r,
			MaxConcurrentReconciles: pointer.IntDeref(r.Config.ConcurrentSyncs, 0),
			RecoverPanic:            true,
		},
	)
	if err != nil {
		return err
	}

	if err := c.Watch(
		source.NewKindWithCache(&corev1.Namespace{}, seedCluster.GetCache()),
		&handler.EnqueueRequestForObject{},
		predicateutils.ForEventTypes(predicateutils.Create, predicateutils.Update),
	); err != nil {
		return err
	}

	if err := c.Watch(
		source.NewKindWithCache(&corev1.Endpoints{}, seedCluster.GetCache()),
		mapper.EnqueueRequestsFrom(mapper.MapFunc(r.MapToNamespaces), mapper.UpdateWithNew, mgr.GetLogger()),
		r.IsKubernetesEndpoint(),
	); err != nil {
		return err
	}

	if err := c.Watch(
		source.NewKindWithCache(&networkingv1.NetworkPolicy{}, seedCluster.GetCache()),
		mapper.EnqueueRequestsFrom(mapper.MapFunc(r.MapObjectToNamespace), mapper.UpdateWithNew, mgr.GetLogger()),
		r.NetworkPolicyPredicate(),
	); err != nil {
		return err
	}

	return c.Watch(
		&source.Channel{Source: r.ResolverUpdate},
		mapper.EnqueueRequestsFrom(mapper.MapFunc(r.MapToNamespaces), mapper.UpdateWithNew, mgr.GetLogger()),
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

// MapToNamespaces is a mapper function which returns requests for all shoot namespaces + garden namespace + istio-system namespace.
func (r *Reconciler) MapToNamespaces(ctx context.Context, log logr.Logger, _ client.Reader, _ client.Object) []reconcile.Request {
	namespaceNames := sets.NewString()

	for _, config := range r.networkPolicyConfigs() {
		for _, selector := range config.namespaceSelectors {
			namespaceList := &corev1.NamespaceList{}
			if err := r.RuntimeClient.List(ctx, namespaceList, client.MatchingLabelsSelector{Selector: selector}); err != nil {
				log.Error(err, "Unable to list namespaces", "selector", selector)
				return nil
			}

			for _, namespace := range namespaceList.Items {
				namespaceNames.Insert(namespace.Name)
			}
		}
	}

	var requests []reconcile.Request
	for _, namespaceName := range namespaceNames.UnsortedList() {
		requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{Name: namespaceName}})
	}
	return requests
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
