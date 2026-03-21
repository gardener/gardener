// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package networkpolicy

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"strings"

	"github.com/go-logr/logr"
	istionetworkingv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils"
)

// ControllerName is the name of the controller.
const ControllerName = "networkpolicy"

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(ctx context.Context, mgr manager.Manager, targetCluster cluster.Cluster) error {
	if r.TargetClient == nil {
		r.TargetClient = targetCluster.GetClient()
	}
	if r.Recorder == nil {
		r.Recorder = mgr.GetEventRecorder(ControllerName + "-controller")
	}

	for _, n := range r.Config.NamespaceSelectors {
		namespaceSelector := n

		selector, err := metav1.LabelSelectorAsSelector(&namespaceSelector)
		if err != nil {
			return fmt.Errorf("failed parsing namespace selector %s to labels.Selector: %w", namespaceSelector, err)
		}
		r.selectors = append(r.selectors, selector)
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

	namespace := &metav1.PartialObjectMetadata{}
	namespace.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("Namespace"))

	c, err := builder.
		ControllerManagedBy(mgr).
		Named(ControllerName).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: ptr.Deref(r.Config.ConcurrentSyncs, 0),
			ReconciliationTimeout:   controllerutils.DefaultReconciliationTimeout,
		}).
		WatchesRawSource(source.Kind[client.Object](
			targetCluster.GetCache(),
			&corev1.Service{},
			&handler.EnqueueRequestForObject{},
			r.ServicePredicate(),
		)).
		WatchesRawSource(source.Kind[client.Object](
			targetCluster.GetCache(),
			networkPolicy,
			handler.EnqueueRequestsFromMapFunc(r.MapNetworkPolicyToService),
			networkPolicyPredicate,
		)).
		WatchesRawSource(source.Kind[client.Object](
			targetCluster.GetCache(),
			namespace,
			r.EventHandlerForNamespace(mgr.GetLogger().WithValues("controller", ControllerName)),
		)).
		Build(r)
	if err != nil {
		return err
	}

	if r.Config.IngressControllerSelector != nil {
		if err := c.Watch(source.Kind[client.Object](
			targetCluster.GetCache(),
			&networkingv1.Ingress{},
			handler.EnqueueRequestsFromMapFunc(r.MapIngressToServices),
			r.IngressPredicate(),
		)); err != nil {
			return err
		}
	}

	crd := &metav1.PartialObjectMetadata{}
	crd.SetGroupVersionKind(apiextensionsv1.SchemeGroupVersion.WithKind("CustomResourceDefinition"))
	if err := targetCluster.GetAPIReader().Get(ctx, client.ObjectKey{Name: "virtualservices.networking.istio.io"}, crd); err != nil {
		logf.FromContext(ctx).Info("Network policy controller deactivated because istio CRDs are not installed", "error", err)
		return nil
	}
	r.istioCRDsFound = true

	return c.Watch(source.Kind[client.Object](
		targetCluster.GetCache(),
		&istionetworkingv1beta1.VirtualService{},
		handler.EnqueueRequestsFromMapFunc(r.MapVirtualServiceToServices),
		r.VirtualServicePredicate(),
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

// VirtualServicePredicate returns a predicate which filters UPDATE events on VirtualServices such that only updates to
// the hosts, gateways, http, TLS or TCP routes are relevant.
func (r *Reconciler) VirtualServicePredicate() predicate.Predicate {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			virtualService, ok := e.ObjectNew.(*istionetworkingv1beta1.VirtualService)
			if !ok {
				return false
			}

			oldVirtualService, ok := e.ObjectOld.(*istionetworkingv1beta1.VirtualService)
			if !ok {
				return false
			}

			return !apiequality.Semantic.DeepEqual(oldVirtualService.Spec.Hosts, virtualService.Spec.Hosts) ||
				!apiequality.Semantic.DeepEqual(oldVirtualService.Spec.Gateways, virtualService.Spec.Gateways) ||
				!apiequality.Semantic.DeepEqual(oldVirtualService.Spec.Http, virtualService.Spec.Http) ||
				!apiequality.Semantic.DeepEqual(oldVirtualService.Spec.Tls, virtualService.Spec.Tls) ||
				!apiequality.Semantic.DeepEqual(oldVirtualService.Spec.Tcp, virtualService.Spec.Tcp)
		},
	}
}

// MapNetworkPolicyToService is a handler.MapFunc for mapping a NetworkPolicy to the referenced service.
func (r *Reconciler) MapNetworkPolicyToService(_ context.Context, obj client.Object) []reconcile.Request {
	if obj == nil || obj.GetLabels() == nil {
		return nil
	}

	return []reconcile.Request{{NamespacedName: types.NamespacedName{
		Name:      obj.GetLabels()[resourcesv1alpha1.NetworkingServiceName],
		Namespace: obj.GetLabels()[resourcesv1alpha1.NetworkingServiceNamespace],
	}}}
}

// EventHandlerForNamespace returns an EventHandler that enqueues reconcile requests for Services
// associated with the given Namespace object.
func (r *Reconciler) EventHandlerForNamespace(log logr.Logger) handler.EventHandler {
	return handler.Funcs{
		CreateFunc: func(ctx context.Context, e event.CreateEvent, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
			r.requeueServicesForNamespace(ctx, e.Object, q, log)
		},
		UpdateFunc: func(ctx context.Context, e event.UpdateEvent, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
			if apiequality.Semantic.DeepEqual(e.ObjectOld.GetLabels(), e.ObjectNew.GetLabels()) {
				return
			}

			requests := sets.New(r.getRelevantServiceForNamespace(ctx, e.ObjectOld, log)...).Union(sets.New(r.getRelevantServiceForNamespace(ctx, e.ObjectNew, log)...))
			for req := range requests {
				q.Add(req)
			}
		},
		DeleteFunc: func(ctx context.Context, e event.DeleteEvent, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
			r.requeueServicesForNamespace(ctx, e.Object, q, log)
		},
		GenericFunc: func(ctx context.Context, e event.GenericEvent, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
			r.requeueServicesForNamespace(ctx, e.Object, q, log)
		},
	}
}

// requeueServicesForNamespace is a helper to find and enqueue services that select a given namespace.
func (r *Reconciler) requeueServicesForNamespace(ctx context.Context, namespace client.Object, q workqueue.TypedRateLimitingInterface[reconcile.Request], log logr.Logger) {
	requests := r.getRelevantServiceForNamespace(ctx, namespace, log)
	for _, request := range requests {
		q.Add(request)
	}
}

func (r *Reconciler) getRelevantServiceForNamespace(ctx context.Context, namespace client.Object, log logr.Logger) []reconcile.Request {
	serviceList := &metav1.PartialObjectMetadataList{}
	serviceList.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("ServiceList"))
	if err := r.TargetClient.List(ctx, serviceList); err != nil {
		log.Error(err, "Failed to list services")
		return nil
	}

	var requests []reconcile.Request

	for _, service := range serviceList.Items {
		// enqueue all the services in the same namespace
		if service.Namespace == namespace.GetName() {
			requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{Name: service.Name, Namespace: service.Namespace}})
			continue
		}

		var namespaceSelectors []metav1.LabelSelector
		if v, ok := service.Annotations[resourcesv1alpha1.NetworkingNamespaceSelectors]; ok {
			if err := json.Unmarshal([]byte(v), &namespaceSelectors); err != nil {
				log.Error(err, "Failed to parse NetworkingNamespaceSelectors", "service", service.Name)
				continue
			}
		}

		for _, namespaceSelector := range namespaceSelectors {
			selector, err := metav1.LabelSelectorAsSelector(&namespaceSelector)
			if err != nil {
				log.Error(err, "Failed to convert LabelSelector", "service", service.Name)
				continue
			}

			if selector.Matches(labels.Set(namespace.GetLabels())) {
				requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{Name: service.Name, Namespace: service.Namespace}})
				break // no need to check other selectors
			}
		}
	}

	return requests
}

// MapIngressToServices is a handler.MapFunc for mapping a Ingresses to all referenced services.
func (r *Reconciler) MapIngressToServices(_ context.Context, obj client.Object) []reconcile.Request {
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

// MapVirtualServiceToServices is a handler.MapFunc for mapping a VirtualService to all referenced services in its routes.
func (r *Reconciler) MapVirtualServiceToServices(_ context.Context, obj client.Object) []reconcile.Request {
	virtualService, ok := obj.(*istionetworkingv1beta1.VirtualService)
	if !ok {
		return nil
	}

	var requests []reconcile.Request

	for _, httpRoute := range virtualService.Spec.Http {
		for _, route := range httpRoute.Route {
			if route.Destination != nil && route.Destination.Host != "" {
				if svc, ok := r.extractServiceNameFromDomain(route.Destination.Host); ok {
					requests = append(requests, reconcile.Request{NamespacedName: svc})
				}
			}
		}
	}

	for _, tlsRoute := range virtualService.Spec.Tls {
		for _, route := range tlsRoute.Route {
			if route.Destination != nil && route.Destination.Host != "" {
				if svc, ok := r.extractServiceNameFromDomain(route.Destination.Host); ok {
					requests = append(requests, reconcile.Request{NamespacedName: svc})
				}
			}
		}
	}

	for _, tcpRoute := range virtualService.Spec.Tcp {
		for _, route := range tcpRoute.Route {
			if route.Destination != nil && route.Destination.Host != "" {
				if svc, ok := r.extractServiceNameFromDomain(route.Destination.Host); ok {
					requests = append(requests, reconcile.Request{NamespacedName: svc})
				}
			}
		}
	}

	return requests
}

func (r *Reconciler) extractServiceNameFromDomain(domain string) (types.NamespacedName, bool) {
	// Only support fully qualified domain names in the form <service>.<namespace>.svc.<cluster-domain>.
	const defaultSuffix = ".svc." + gardencorev1beta1.DefaultDomain
	if !strings.HasSuffix(domain, defaultSuffix) {
		return types.NamespacedName{}, false
	}

	serviceAndNamespace := domain[:len(domain)-len(defaultSuffix)]
	parts := strings.Split(serviceAndNamespace, ".")
	if len(parts) != 2 {
		return types.NamespacedName{}, false
	}

	return types.NamespacedName{Name: parts[0], Namespace: parts[1]}, true
}
