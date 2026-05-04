// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package istioclusterconfiguration

import (
	"context"
	"slices"

	istionetworkingv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/controllerutils"
)

// ControllerName is the name of the controller.
const ControllerName = "istio-cluster-configuration"

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(mgr manager.Manager, targetCluster cluster.Cluster) error {
	if r.TargetClient == nil {
		r.TargetClient = targetCluster.GetClient()
	}

	gvk, err := apiutil.GVKForObject(&istionetworkingv1beta1.DestinationRule{}, targetCluster.GetScheme())
	if err != nil {
		return err
	}

	if _, err = targetCluster.GetRESTMapper().RESTMapping(gvk.GroupKind(), gvk.Version); err != nil {
		if !meta.IsNoMatchError(err) {
			return err
		}
		mgr.GetLogger().WithValues("controller", ControllerName).Info("No watches will be added because Istio CRDs are not installed")
		return nil
	}

	_, err = builder.
		ControllerManagedBy(mgr).
		Named(ControllerName).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: ptr.Deref(r.Config.ConcurrentSyncs, 0),
			ReconciliationTimeout:   controllerutils.DefaultReconciliationTimeout,
		}).
		WatchesRawSource(source.Kind[client.Object](
			targetCluster.GetCache(),
			&istionetworkingv1beta1.DestinationRule{},
			handler.EnqueueRequestsFromMapFunc(r.MapDestinationRuleToNamespace),
			r.DestinationRulePredicate(),
		)).
		WatchesRawSource(source.Kind[client.Object](
			targetCluster.GetCache(),
			&corev1.Service{},
			handler.EnqueueRequestsFromMapFunc(r.MapServiceToNamespaces),
			r.ServicePredicate(),
		)).
		WatchesRawSource(source.Kind[client.Object](
			targetCluster.GetCache(),
			&corev1.Namespace{},
			handler.EnqueueRequestsFromMapFunc(r.MapNamespaceToSourceNamespaces),
			r.NamespacePredicate(),
		)).
		Build(r)

	return err
}

// MapDestinationRuleToNamespace maps a DestinationRule event to the source namespace for reconciliation.
func (r *Reconciler) MapDestinationRuleToNamespace(_ context.Context, obj client.Object) []reconcile.Request {
	if obj == nil {
		return nil
	}
	return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: obj.GetNamespace()}}}
}

// MapServiceToNamespaces maps a Service change to all source namespaces that have DestinationRules referencing it.
func (r *Reconciler) MapServiceToNamespaces(ctx context.Context, obj client.Object) []reconcile.Request {
	service, ok := obj.(*corev1.Service)
	if !ok {
		return nil
	}

	destinationRules := &istionetworkingv1beta1.DestinationRuleList{}
	if err := r.TargetClient.List(ctx, destinationRules); err != nil {
		logf.FromContext(ctx).Error(err, "Failed to list DestinationRules")
		return nil
	}

	namespaces := sets.New[string]()

	for _, destinationRule := range destinationRules.Items {
		if serviceAndDestinationRuleMatch(service, destinationRule) {
			namespaces.Insert(destinationRule.Namespace)
		}
	}

	var requests []reconcile.Request

	for _, namespace := range namespaces.UnsortedList() {
		requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{Name: namespace}})
	}

	return requests
}

// MapNamespaceToSourceNamespaces maps istio-ingress namespace events to all source namespaces that have DRs.
func (r *Reconciler) MapNamespaceToSourceNamespaces(ctx context.Context, obj client.Object) []reconcile.Request {
	if obj == nil {
		return nil
	}

	destinationRules := &istionetworkingv1beta1.DestinationRuleList{}
	if err := r.TargetClient.List(ctx, destinationRules); err != nil {
		logf.FromContext(ctx).Error(err, "Failed to list DestinationRules")
		return nil
	}

	namespaces := sets.New[string]()

	for _, destinationRule := range destinationRules.Items {
		if len(destinationRule.Spec.ExportTo) == 0 ||
			slices.Contains(destinationRule.Spec.ExportTo, obj.GetName()) ||
			slices.Contains(destinationRule.Spec.ExportTo, "*") ||
			(slices.Contains(destinationRule.Spec.ExportTo, ".") && destinationRule.Namespace == obj.GetName()) {
			namespaces.Insert(destinationRule.Namespace)
		}
	}

	var requests []reconcile.Request

	for _, namespace := range namespaces.UnsortedList() {
		requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{Name: namespace}})
	}

	return requests
}

// DestinationRulePredicate filters DestinationRule events to those that affect cluster configuration.
func (r *Reconciler) DestinationRulePredicate() predicate.Predicate {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			newDestinationRule, ok := e.ObjectNew.(*istionetworkingv1beta1.DestinationRule)
			if !ok {
				return false
			}
			oldDestinationRule, ok := e.ObjectOld.(*istionetworkingv1beta1.DestinationRule)
			if !ok {
				return false
			}

			return newDestinationRule.Spec.Host != oldDestinationRule.Spec.Host ||
				!apiequality.Semantic.DeepEqual(newDestinationRule.Spec.ExportTo, oldDestinationRule.Spec.ExportTo) ||
				!apiequality.Semantic.DeepEqual(newDestinationRule.Spec.TrafficPolicy, oldDestinationRule.Spec.TrafficPolicy)
		},
	}
}

// ServicePredicate filters Service events to port name changes.
func (r *Reconciler) ServicePredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(_ event.CreateEvent) bool { return true },
		DeleteFunc: func(_ event.DeleteEvent) bool { return true },
		UpdateFunc: func(e event.UpdateEvent) bool {
			newService, ok := e.ObjectNew.(*corev1.Service)
			if !ok {
				return false
			}
			oldService, ok := e.ObjectOld.(*corev1.Service)
			if !ok {
				return false
			}
			return !apiequality.Semantic.DeepEqual(oldService.Spec.Ports, newService.Spec.Ports)
		},
	}
}

// NamespacePredicate filters Namespace events to istio-ingress label changes.
func (r *Reconciler) NamespacePredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return isIstioIngressNamespace(e.Object.GetLabels())
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return isIstioIngressNamespace(e.Object.GetLabels())
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return isIstioIngressNamespace(e.ObjectOld.GetLabels()) != isIstioIngressNamespace(e.ObjectNew.GetLabels())
		},
	}
}

func isIstioIngressNamespace(labels map[string]string) bool {
	return labels[v1beta1constants.GardenRole] == v1beta1constants.GardenRoleIstioIngress
}

func serviceAndDestinationRuleMatch(service *corev1.Service, destinationRule *istionetworkingv1beta1.DestinationRule) bool {
	if service == nil || destinationRule == nil {
		return false
	}

	if destinationRule.Spec.Host == getServiceFQDN(service.Name, service.Namespace) {
		return true
	}

	if service.Namespace == destinationRule.Namespace && destinationRule.Spec.Host == service.Name {
		return true
	}

	return false
}
