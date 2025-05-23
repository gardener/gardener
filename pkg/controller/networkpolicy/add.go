// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package networkpolicy

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"
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

	"github.com/gardener/gardener/pkg/controller/networkpolicy/hostnameresolver"
	predicateutils "github.com/gardener/gardener/pkg/controllerutils/predicate"
)

// ControllerName is the name of this controller.
const ControllerName = "networkpolicy"

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(mgr manager.Manager, runtimeCluster cluster.Cluster) error {
	if r.RuntimeClient == nil {
		r.RuntimeClient = runtimeCluster.GetClient()
	}
	if r.Resolver == nil {
		resolver, err := hostnameresolver.CreateForCluster(runtimeCluster.GetConfig(), mgr.GetLogger())
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
	for _, l := range r.AdditionalNamespaceSelectors {
		namespaceSelector := l
		selector, err := metav1.LabelSelectorAsSelector(&namespaceSelector)
		if err != nil {
			return fmt.Errorf("failed parsing namespace selector %s to labels.Selector: %w", namespaceSelector, err)
		}
		r.additionalNamespaceLabelSelectors = append(r.additionalNamespaceLabelSelectors, selector)
	}

	c, err := builder.
		ControllerManagedBy(mgr).
		Named(ControllerName).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: ptr.Deref(r.ConcurrentSyncs, 0),
		}).
		WatchesRawSource(source.Kind[client.Object](
			runtimeCluster.GetCache(),
			&corev1.Namespace{},
			&handler.EnqueueRequestForObject{},
			predicateutils.ForEventTypes(predicateutils.Create, predicateutils.Update),
		)).
		WatchesRawSource(source.Kind[client.Object](
			runtimeCluster.GetCache(),
			&corev1.Endpoints{},
			handler.EnqueueRequestsFromMapFunc(r.MapToNamespaces(mgr.GetLogger().WithValues("controller", ControllerName))),
			r.IsKubernetesEndpoint(),
		)).
		WatchesRawSource(source.Kind[client.Object](
			runtimeCluster.GetCache(),
			&networkingv1.NetworkPolicy{},
			handler.EnqueueRequestsFromMapFunc(r.MapObjectToNamespace),
			r.NetworkPolicyPredicate(),
		)).
		WatchesRawSource(source.Channel(
			r.ResolverUpdate,
			handler.EnqueueRequestsFromMapFunc(r.MapToNamespaces(mgr.GetLogger().WithValues("controller", ControllerName))),
		)).
		Build(r)
	if err != nil {
		return err
	}

	for _, registerer := range r.WatchRegisterers {
		if err := registerer(c); err != nil {
			return err
		}
	}

	return nil
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

// MapToNamespaces is a mapper function which returns requests for all relevant namespaces.
func (r *Reconciler) MapToNamespaces(log logr.Logger) handler.MapFunc {
	return func(ctx context.Context, _ client.Object) []reconcile.Request {
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
}

// MapObjectToName is a mapper function which maps an object to its name.
func (r *Reconciler) MapObjectToName(_ context.Context, obj client.Object) []reconcile.Request {
	return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: obj.GetName()}}}
}

// MapObjectToNamespace is a mapper function which maps an object to its namespace.
func (r *Reconciler) MapObjectToNamespace(_ context.Context, obj client.Object) []reconcile.Request {
	return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: obj.GetNamespace()}}}
}

// IsKubernetesEndpoint returns a predicate which evaluates if the object is the kubernetes endpoint.
func (r *Reconciler) IsKubernetesEndpoint() predicate.Predicate {
	return predicate.NewPredicateFuncs(func(obj client.Object) bool {
		return obj.GetNamespace() == corev1.NamespaceDefault && obj.GetName() == "kubernetes"
	})
}
