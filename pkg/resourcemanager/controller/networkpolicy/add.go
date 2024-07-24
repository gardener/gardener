// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package networkpolicy

import (
	"context"
	"fmt"
	"maps"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
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

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils/mapper"
)

// ControllerName is the name of the controller.
const ControllerName = "networkpolicy"

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(ctx context.Context, mgr manager.Manager, targetCluster cluster.Cluster) error {
	if r.TargetClient == nil {
		r.TargetClient = targetCluster.GetClient()
	}

	for _, n := range r.Config.NamespaceSelectors {
		namespaceSelector := n

		selector, err := metav1.LabelSelectorAsSelector(&namespaceSelector)
		if err != nil {
			return fmt.Errorf("failed parsing namespace selector %s to labels.Selector: %w", namespaceSelector, err)
		}
		r.selectors = append(r.selectors, selector)
	}

	c, err := builder.
		ControllerManagedBy(mgr).
		Named(ControllerName).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: ptr.Deref(r.Config.ConcurrentSyncs, 0),
		}).
		WatchesRawSource(
			source.Kind[client.Object](targetCluster.GetCache(),
				&corev1.Service{},
				&handler.EnqueueRequestForObject{},
				builder.WithPredicates(r.ServicePredicate()),
			)).
		Build(r)
	if err != nil {
		return err
	}

	networkPolicy := &metav1.PartialObjectMetadata{}
	networkPolicy.SetGroupVersionKind(networkingv1.SchemeGroupVersion.WithKind("NetworkPolicy"))

	networkPolicyPredicate, err := predicate.LabelSelectorPredicate(metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{
		{Key: resourcesv1alpha1.NetworkingServiceName, Operator: metav1.LabelSelectorOpExists},
		{Key: resourcesv1alpha1.NetworkingServiceNamespace, Operator: metav1.LabelSelectorOpExists},
	}})
	if err != nil {
		return err
	}

	if err := c.Watch(
		source.Kind[client.Object](targetCluster.GetCache(),
			networkPolicy,
			mapper.EnqueueRequestsFrom(ctx, mgr.GetCache(), mapper.MapFunc(r.MapNetworkPolicyToService), mapper.UpdateWithNew, c.GetLogger()),
			networkPolicyPredicate,
		)); err != nil {
		return err
	}

	if r.Config.IngressControllerSelector != nil {
		if err := c.Watch(
			source.Kind[client.Object](targetCluster.GetCache(),
				&networkingv1.Ingress{},
				mapper.EnqueueRequestsFrom(ctx, mgr.GetCache(), mapper.MapFunc(r.MapIngressToServices), mapper.UpdateWithNew, c.GetLogger()),
				r.IngressPredicate(),
			)); err != nil {
			return err
		}
	}

	namespace := &metav1.PartialObjectMetadata{}
	namespace.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("Namespace"))

	return c.Watch(
		source.Kind[client.Object](targetCluster.GetCache(),
			namespace,
			mapper.EnqueueRequestsFrom(ctx, mgr.GetCache(), mapper.MapFunc(r.MapToAllServices), mapper.UpdateWithNew, c.GetLogger()),
		))
}

// ServicePredicate returns a predicate which filters UPDATE events on services such that only updates to the deletion
// timestamp, the port list, the pod label selector, or well-known annotations are relevant.
func (r *Reconciler) ServicePredicate() predicate.Predicate {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			service, ok := e.ObjectNew.(*corev1.Service)
			if !ok {
				return false
			}

			oldService, ok := e.ObjectOld.(*corev1.Service)
			if !ok {
				return false
			}

			return (oldService.DeletionTimestamp == nil && service.DeletionTimestamp != nil) ||
				!apiequality.Semantic.DeepEqual(service.Spec.Selector, oldService.Spec.Selector) ||
				!apiequality.Semantic.DeepEqual(service.Spec.Ports, oldService.Spec.Ports) ||
				oldService.Annotations[resourcesv1alpha1.NetworkingPodLabelSelectorNamespaceAlias] != service.Annotations[resourcesv1alpha1.NetworkingPodLabelSelectorNamespaceAlias] ||
				oldService.Annotations[resourcesv1alpha1.NetworkingNamespaceSelectors] != service.Annotations[resourcesv1alpha1.NetworkingNamespaceSelectors] ||
				oldService.Annotations[resourcesv1alpha1.NetworkingFromWorldToPorts] != service.Annotations[resourcesv1alpha1.NetworkingFromWorldToPorts] ||
				fromPolicyAnnotationsChanged(oldService.Annotations, service.Annotations)
		},
	}
}

func fromPolicyAnnotationsChanged(oldAnnotations, newAnnotations map[string]string) bool {
	var (
		oldFromPolicies = make(map[string]string)
		newFromPolicies = make(map[string]string)

		getPolicies = func(annotations, into map[string]string) {
			for k, allowedPorts := range annotations {
				match := fromPolicyRegexp.FindStringSubmatch(k)
				if len(match) != 2 {
					continue
				}
				customPodLabelSelector := match[1]
				into[customPodLabelSelector] = allowedPorts
			}
		}
	)

	getPolicies(oldAnnotations, oldFromPolicies)
	getPolicies(newAnnotations, newFromPolicies)

	return !maps.Equal(oldFromPolicies, newFromPolicies)
}

// IngressPredicate returns a predicate which filters UPDATE events on Ingresses such that only updates to the rules
// are relevant.
func (r *Reconciler) IngressPredicate() predicate.Predicate {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			ingress, ok := e.ObjectNew.(*networkingv1.Ingress)
			if !ok {
				return false
			}

			oldIngress, ok := e.ObjectOld.(*networkingv1.Ingress)
			if !ok {
				return false
			}

			return !apiequality.Semantic.DeepEqual(oldIngress.Spec.Rules, ingress.Spec.Rules)
		},
	}
}

// MapNetworkPolicyToService is a mapper.MapFunc for mapping a NetworkPolicy to the referenced service.
func (r *Reconciler) MapNetworkPolicyToService(_ context.Context, _ logr.Logger, _ client.Reader, obj client.Object) []reconcile.Request {
	if obj == nil || obj.GetLabels() == nil {
		return nil
	}

	return []reconcile.Request{{NamespacedName: types.NamespacedName{
		Name:      obj.GetLabels()[resourcesv1alpha1.NetworkingServiceName],
		Namespace: obj.GetLabels()[resourcesv1alpha1.NetworkingServiceNamespace],
	}}}
}

// MapToAllServices is a mapper.MapFunc for mapping a Namespace to all Services.
func (r *Reconciler) MapToAllServices(ctx context.Context, log logr.Logger, _ client.Reader, _ client.Object) []reconcile.Request {
	serviceList := &metav1.PartialObjectMetadataList{}
	serviceList.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("ServiceList"))
	if err := r.TargetClient.List(ctx, serviceList); err != nil {
		log.Error(err, "Failed to list services")
		return nil
	}

	var requests []reconcile.Request

	for _, service := range serviceList.Items {
		requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{Name: service.Name, Namespace: service.Namespace}})
	}

	return requests
}

// MapIngressToServices is a mapper.MapFunc for mapping a Ingresses to all referenced services.
func (r *Reconciler) MapIngressToServices(_ context.Context, _ logr.Logger, _ client.Reader, obj client.Object) []reconcile.Request {
	ingress, ok := obj.(*networkingv1.Ingress)
	if !ok {
		return nil
	}

	var requests []reconcile.Request

	for _, rule := range ingress.Spec.Rules {
		if rule.HTTP == nil {
			continue
		}

		for _, path := range rule.HTTP.Paths {
			if path.Backend.Service != nil {
				requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{Name: path.Backend.Service.Name, Namespace: ingress.Namespace}})
			}
		}
	}

	return requests
}
