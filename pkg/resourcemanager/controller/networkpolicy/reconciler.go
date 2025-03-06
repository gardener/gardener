// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package networkpolicy

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils"
	resourcemanagerconfigv1alpha1 "github.com/gardener/gardener/pkg/resourcemanager/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/utils/flow"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

var fromPolicyRegexp = regexp.MustCompile(resourcesv1alpha1.NetworkPolicyFromPolicyAnnotationPrefix + "(.*)" + resourcesv1alpha1.NetworkPolicyFromPolicyAnnotationSuffix)

// Reconciler reconciles Service objects and creates NetworkPolicy objects.
type Reconciler struct {
	TargetClient client.Client
	Config       resourcemanagerconfigv1alpha1.NetworkPolicyControllerConfig

	selectors []labels.Selector
}

// Reconcile performs the main reconciliation logic.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	ctx, cancel := controllerutils.GetMainReconciliationContext(ctx, controllerutils.DefaultReconciliationTimeout)
	defer cancel()

	networkPolicyList := &metav1.PartialObjectMetadataList{}
	networkPolicyList.SetGroupVersionKind(networkingv1.SchemeGroupVersion.WithKind("NetworkPolicyList"))
	if err := r.TargetClient.List(ctx, networkPolicyList, client.MatchingLabels{
		resourcesv1alpha1.NetworkingServiceName:      request.NamespacedName.Name,
		resourcesv1alpha1.NetworkingServiceNamespace: request.NamespacedName.Namespace,
	}); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed listing network policies for service %s: %w", request.NamespacedName, err)
	}

	isNamespaceHandled, err := r.namespaceIsHandled(ctx, request.NamespacedName.Namespace)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed checking whether namespace %s is handled: %w", request.NamespacedName.Namespace, err)
	}

	onlyDeleteStalePolicies := !isNamespaceHandled

	service := &corev1.Service{}
	if err := r.TargetClient.Get(ctx, request.NamespacedName, service); err != nil {
		if !apierrors.IsNotFound(err) {
			return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
		}
		log.V(1).Info("Object is gone, cleaning up")
		onlyDeleteStalePolicies = true
	}

	if onlyDeleteStalePolicies || service.DeletionTimestamp != nil || service.Spec.Selector == nil {
		deleteTaskFns := r.deleteStalePolicies(networkPolicyList, nil)
		return reconcile.Result{}, flow.Parallel(deleteTaskFns...)(ctx)
	}

	namespaceNames, err := r.fetchRelevantNamespaceNames(ctx, service)
	if err != nil {
		return reconcile.Result{}, err
	}

	reconcileTaskFns, desiredObjectMetaKeys, err := r.reconcileDesiredPolicies(ctx, service, namespaceNames)
	if err != nil {
		return reconcile.Result{}, err
	}
	deleteTaskFns := r.deleteStalePolicies(networkPolicyList, desiredObjectMetaKeys)

	return reconcile.Result{}, flow.Parallel(append(reconcileTaskFns, deleteTaskFns...)...)(ctx)
}

func (r *Reconciler) namespaceIsHandled(ctx context.Context, namespaceName string) (bool, error) {
	namespace := &metav1.PartialObjectMetadata{}
	namespace.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("Namespace"))
	if err := r.TargetClient.Get(ctx, client.ObjectKey{Name: namespaceName}, namespace); client.IgnoreNotFound(err) != nil {
		return false, fmt.Errorf("failed to get namespace %q: %w", namespaceName, err)
	}

	if len(r.selectors) == 0 {
		return true, nil
	}

	for _, selector := range r.selectors {
		if selector.Matches(labels.Set(namespace.GetLabels())) {
			return true, nil
		}
	}

	return false, nil
}

func (r *Reconciler) fetchRelevantNamespaceNames(ctx context.Context, service *corev1.Service) (sets.Set[string], error) {
	var namespaceSelectors []metav1.LabelSelector
	if v, ok := service.Annotations[resourcesv1alpha1.NetworkingNamespaceSelectors]; ok {
		if err := json.Unmarshal([]byte(v), &namespaceSelectors); err != nil {
			return nil, fmt.Errorf("failed unmarshalling %s: %w", v, err)
		}
	}

	var (
		namespaceNames                    = sets.New[string]()
		considerNamespaceIfNotTerminating = func(namespaces ...metav1.PartialObjectMetadata) {
			for _, namespace := range namespaces {
				if namespace.DeletionTimestamp == nil {
					namespaceNames.Insert(namespace.Name)
				}
			}
		}
	)

	namespace := &metav1.PartialObjectMetadata{}
	namespace.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("Namespace"))
	if err := r.TargetClient.Get(ctx, client.ObjectKey{Name: service.Namespace}, namespace); err != nil {
		return nil, fmt.Errorf("failed fetching service namespace %s: %w", service.Namespace, err)
	}
	considerNamespaceIfNotTerminating(*namespace)

	for _, namespaceSelector := range namespaceSelectors {
		selector, err := metav1.LabelSelectorAsSelector(&namespaceSelector)
		if err != nil {
			return nil, fmt.Errorf("failed parsing %s to labels.Selector: %w", namespaceSelector, err)
		}

		namespaceList := &metav1.PartialObjectMetadataList{}
		namespaceList.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("NamespaceList"))
		if err := r.TargetClient.List(ctx, namespaceList, client.MatchingLabelsSelector{Selector: selector}); err != nil {
			return nil, fmt.Errorf("failed listing namespaces with selector %s: %w", selector.String(), err)
		}
		considerNamespaceIfNotTerminating(namespaceList.Items...)
	}

	return namespaceNames, nil
}

func (r *Reconciler) reconcileDesiredPolicies(ctx context.Context, service *corev1.Service, namespaceNames sets.Set[string]) ([]flow.TaskFn, []string, error) {
	var (
		taskFns               []flow.TaskFn
		desiredObjectMetaKeys []string

		addTasksForPort = func(
			port networkingv1.NetworkPolicyPort,
			policyID string,
			namespaceName string,
			podSelector metav1.LabelSelector,
			ingressObjectMetaFunc func(string, string, string) metav1.ObjectMeta,
			egressObjectMetaFunc func(string, string, string) metav1.ObjectMeta,
		) {
			for _, fns := range []struct {
				objectMetaFunc func(string, string, string) metav1.ObjectMeta
				reconcileFunc  func(context.Context, *corev1.Service, networkingv1.NetworkPolicyPort, metav1.ObjectMeta, string, metav1.LabelSelector) error
			}{
				{objectMetaFunc: ingressObjectMetaFunc, reconcileFunc: r.reconcileIngressPolicy},
				{objectMetaFunc: egressObjectMetaFunc, reconcileFunc: r.reconcileEgressPolicy},
			} {
				reconcileFn := fns.reconcileFunc
				objectMeta := fns.objectMetaFunc(policyID, service.Namespace, namespaceName)
				desiredObjectMetaKeys = append(desiredObjectMetaKeys, key(objectMeta))

				taskFns = append(taskFns, func(ctx context.Context) error {
					return reconcileFn(ctx, service, port, objectMeta, namespaceName, podSelector)
				})
			}
		}

		addTasksForRelevantNamespacesAndPort = func(port networkingv1.NetworkPolicyPort, customPodLabelSelector string) {
			policyID := policyIDFor(service.Name, port)
			podLabelSelector := policyID

			if customPodLabelSelector != "" {
				policyID += "-via-" + customPodLabelSelector
				podLabelSelector = customPodLabelSelector
			}

			for _, n := range namespaceNames.UnsortedList() {
				namespaceName := n
				matchLabels := matchLabelsForServiceAndNamespace(podLabelSelector, service, namespaceName)
				addTasksForPort(port, policyID, namespaceName, metav1.LabelSelector{MatchLabels: matchLabels}, ingressPolicyObjectMetaFor, egressPolicyObjectMetaFor)
			}
		}
	)

	for _, p := range service.Spec.Ports {
		port := p
		addTasksForRelevantNamespacesAndPort(networkingv1.NetworkPolicyPort{Protocol: &port.Protocol, Port: &port.TargetPort}, "")
	}

	for k, allowedPorts := range service.Annotations {
		match := fromPolicyRegexp.FindStringSubmatch(k)
		if len(match) != 2 {
			continue
		}

		var (
			customPodLabelSelector = match[1]
			ports                  []networkingv1.NetworkPolicyPort
		)

		if err := json.Unmarshal([]byte(allowedPorts), &ports); err != nil {
			return nil, nil, fmt.Errorf("failed unmarshalling %s: %w", allowedPorts, err)
		}

		for _, port := range ports {
			addTasksForRelevantNamespacesAndPort(port, customPodLabelSelector)
		}
	}

	if _, ok := service.Annotations[resourcesv1alpha1.NetworkingFromWorldToPorts]; ok {
		objectMeta := metav1.ObjectMeta{Name: "ingress-to-" + service.Name + "-from-world", Namespace: service.Namespace}
		desiredObjectMetaKeys = append(desiredObjectMetaKeys, key(objectMeta))
		taskFns = append(taskFns, func(ctx context.Context) error {
			return r.reconcileIngressFromWorldPolicy(ctx, service, objectMeta)
		})
	}

	portsExposedViaIngresses, err := r.portsExposedByIngressResources(ctx, service)
	if err != nil {
		return nil, nil, err
	}

	for _, p := range portsExposedViaIngresses {
		port := p
		policyID := policyIDFor(service.Name, port)
		addTasksForPort(port, policyID, r.Config.IngressControllerSelector.Namespace, r.Config.IngressControllerSelector.PodSelector, ingressPolicyObjectMetaWhenExposedViaIngressFor, egressPolicyObjectMetaWhenExposedViaIngressFor)
	}

	return taskFns, desiredObjectMetaKeys, nil
}

func (r *Reconciler) deleteStalePolicies(networkPolicyList *metav1.PartialObjectMetadataList, desiredObjectMetaKeys []string) []flow.TaskFn {
	objectMetaKeysForDesiredPolicies := make(map[string]struct{}, len(desiredObjectMetaKeys))
	for _, objectMetaKey := range desiredObjectMetaKeys {
		objectMetaKeysForDesiredPolicies[objectMetaKey] = struct{}{}
	}

	var taskFns []flow.TaskFn

	for _, n := range networkPolicyList.Items {
		networkPolicy := n

		if _, ok := objectMetaKeysForDesiredPolicies[key(networkPolicy.ObjectMeta)]; !ok {
			taskFns = append(taskFns, func(ctx context.Context) error {
				return kubernetesutils.DeleteObject(ctx, r.TargetClient, &networkPolicy)
			})
		}
	}

	return taskFns
}

func (r *Reconciler) reconcileIngressPolicy(
	ctx context.Context,
	service *corev1.Service,
	port networkingv1.NetworkPolicyPort,
	networkPolicyObjectMeta metav1.ObjectMeta,
	namespaceName string,
	podSelector metav1.LabelSelector,
) error {
	networkPolicy := &networkingv1.NetworkPolicy{ObjectMeta: networkPolicyObjectMeta}

	_, err := controllerutils.GetAndCreateOrMergePatch(ctx, r.TargetClient, networkPolicy, func() error {
		metav1.SetMetaDataLabel(&networkPolicy.ObjectMeta, resourcesv1alpha1.NetworkingServiceName, service.Name)
		metav1.SetMetaDataLabel(&networkPolicy.ObjectMeta, resourcesv1alpha1.NetworkingServiceNamespace, service.Namespace)

		metav1.SetMetaDataAnnotation(&networkPolicy.ObjectMeta, v1beta1constants.GardenerDescription, fmt.Sprintf("Allows "+
			"ingress %s traffic to port %s for pods selected by the %s service selector from pods running in namespace %s labeled "+
			"with %s.", *port.Protocol, port.Port.String(), client.ObjectKeyFromObject(service), namespaceName, podSelector))

		networkPolicy.Spec.Ingress = []networkingv1.NetworkPolicyIngressRule{{
			From: []networkingv1.NetworkPolicyPeer{{
				PodSelector:       &podSelector,
				NamespaceSelector: ingressNamespaceSelectorFor(service.Namespace, namespaceName),
			}},
			Ports: []networkingv1.NetworkPolicyPort{port},
		}}
		networkPolicy.Spec.Egress = nil
		networkPolicy.Spec.PodSelector = metav1.LabelSelector{MatchLabels: service.Spec.Selector}
		networkPolicy.Spec.PolicyTypes = []networkingv1.PolicyType{networkingv1.PolicyTypeIngress}

		return nil
	}, controllerutils.SkipEmptyPatch{})

	return err
}

func (r *Reconciler) reconcileEgressPolicy(
	ctx context.Context,
	service *corev1.Service,
	port networkingv1.NetworkPolicyPort,
	networkPolicyObjectMeta metav1.ObjectMeta,
	namespaceName string,
	podLabelSelector metav1.LabelSelector,
) error {
	networkPolicy := &networkingv1.NetworkPolicy{ObjectMeta: networkPolicyObjectMeta}

	_, err := controllerutils.GetAndCreateOrMergePatch(ctx, r.TargetClient, networkPolicy, func() error {
		metav1.SetMetaDataLabel(&networkPolicy.ObjectMeta, resourcesv1alpha1.NetworkingServiceName, service.Name)
		metav1.SetMetaDataLabel(&networkPolicy.ObjectMeta, resourcesv1alpha1.NetworkingServiceNamespace, service.Namespace)

		metav1.SetMetaDataAnnotation(&networkPolicy.ObjectMeta, v1beta1constants.GardenerDescription, fmt.Sprintf("Allows "+
			"egress %s traffic to port %s from pods running in namespace %s labeled with %s to pods selected by the %s service "+
			"selector.", *port.Protocol, port.Port.String(), namespaceName, podLabelSelector, client.ObjectKeyFromObject(service)))

		networkPolicy.Spec.Ingress = nil
		networkPolicy.Spec.Egress = []networkingv1.NetworkPolicyEgressRule{{
			To: []networkingv1.NetworkPolicyPeer{{
				PodSelector:       &metav1.LabelSelector{MatchLabels: service.Spec.Selector},
				NamespaceSelector: egressNamespaceSelectorFor(service.Namespace, namespaceName),
			}},
			Ports: []networkingv1.NetworkPolicyPort{port},
		}}
		networkPolicy.Spec.PodSelector = podLabelSelector
		networkPolicy.Spec.PolicyTypes = []networkingv1.PolicyType{networkingv1.PolicyTypeEgress}

		return nil
	}, controllerutils.SkipEmptyPatch{})

	return err
}

func (r *Reconciler) reconcileIngressFromWorldPolicy(ctx context.Context, service *corev1.Service, networkPolicyObjectMeta metav1.ObjectMeta) error {
	var ports []networkingv1.NetworkPolicyPort
	if err := json.Unmarshal([]byte(service.Annotations[resourcesv1alpha1.NetworkingFromWorldToPorts]), &ports); err != nil {
		return fmt.Errorf("failed unmarshalling %s: %w", service.Annotations[resourcesv1alpha1.NetworkingFromWorldToPorts], err)
	}

	networkPolicy := &networkingv1.NetworkPolicy{ObjectMeta: networkPolicyObjectMeta}
	_, err := controllerutils.GetAndCreateOrMergePatch(ctx, r.TargetClient, networkPolicy, func() error {
		metav1.SetMetaDataLabel(&networkPolicy.ObjectMeta, resourcesv1alpha1.NetworkingServiceName, service.Name)
		metav1.SetMetaDataLabel(&networkPolicy.ObjectMeta, resourcesv1alpha1.NetworkingServiceNamespace, service.Namespace)

		metav1.SetMetaDataAnnotation(&networkPolicy.ObjectMeta, v1beta1constants.GardenerDescription, fmt.Sprintf("Allows "+
			"ingress traffic from everywhere to ports %v for pods selected by the %s service selector.", portAndProtocolOf(ports),
			client.ObjectKeyFromObject(service)))

		networkPolicy.Spec.Ingress = []networkingv1.NetworkPolicyIngressRule{{Ports: ports}}
		networkPolicy.Spec.Egress = nil
		networkPolicy.Spec.PodSelector = metav1.LabelSelector{MatchLabels: service.Spec.Selector}
		networkPolicy.Spec.PolicyTypes = []networkingv1.PolicyType{networkingv1.PolicyTypeIngress}

		return nil
	}, controllerutils.SkipEmptyPatch{})
	return err
}

func portAndProtocolOf(ports []networkingv1.NetworkPolicyPort) []string {
	var result []string
	for _, v := range ports {
		if v.Protocol == nil {
			result = append(result, fmt.Sprintf("%v", v.Port))
		} else {
			result = append(result, fmt.Sprintf("%v/%v", *v.Protocol, v.Port))
		}
	}
	return result
}

func (r *Reconciler) portsExposedByIngressResources(ctx context.Context, service *corev1.Service) ([]networkingv1.NetworkPolicyPort, error) {
	if r.Config.IngressControllerSelector == nil {
		return nil, nil
	}

	ingressList := &networkingv1.IngressList{}
	if err := r.TargetClient.List(ctx, ingressList, client.InNamespace(service.Namespace)); err != nil {
		return nil, err
	}

	var serviceBackendPorts []networkingv1.ServiceBackendPort

	for _, ingress := range ingressList.Items {
		for _, rule := range ingress.Spec.Rules {
			if rule.HTTP == nil {
				continue
			}

			for _, path := range rule.HTTP.Paths {
				if path.Backend.Service != nil && path.Backend.Service.Name == service.Name {
					serviceBackendPorts = append(serviceBackendPorts, path.Backend.Service.Port)
				}
			}
		}
	}

	return serviceBackendPortsToNetworkPolicyPorts(service, serviceBackendPorts), nil
}

func serviceBackendPortsToNetworkPolicyPorts(service *corev1.Service, serviceBackendPorts []networkingv1.ServiceBackendPort) (networkPolicyPorts []networkingv1.NetworkPolicyPort) {
	networkPolicyPortForBackendPort := func(backendPort networkingv1.ServiceBackendPort) *networkingv1.NetworkPolicyPort {
		for _, port := range service.Spec.Ports {
			if backendPort.Name == port.Name || backendPort.Number == port.Port {
				return &networkingv1.NetworkPolicyPort{
					Protocol: &port.Protocol,
					Port:     &port.TargetPort,
				}
			}
		}
		return nil
	}

	for _, backendPort := range serviceBackendPorts {
		if networkPolicyPort := networkPolicyPortForBackendPort(backendPort); networkPolicyPort != nil {
			networkPolicyPorts = append(networkPolicyPorts, *networkPolicyPort)
		}
	}

	return
}

func policyIDFor(serviceName string, port networkingv1.NetworkPolicyPort) string {
	return fmt.Sprintf("%s-%s-%s", serviceName, strings.ToLower(string(*port.Protocol)), port.Port.String())
}

func matchLabelsForServiceAndNamespace(podLabelSelector string, service *corev1.Service, namespaceName string) map[string]string {
	var infix string

	if service.Namespace != namespaceName {
		infix = service.Namespace

		if namespaceAlias, ok := service.Annotations[resourcesv1alpha1.NetworkingPodLabelSelectorNamespaceAlias]; ok {
			infix = namespaceAlias
		}

		infix += "-"
	}

	return map[string]string{"networking.resources.gardener.cloud/to-" + infix + podLabelSelector: v1beta1constants.LabelNetworkPolicyAllowed}
}

func ingressPolicyObjectMetaFor(policyID, serviceNamespace, namespaceName string) metav1.ObjectMeta {
	name := "ingress-to-" + policyID
	if serviceNamespace != namespaceName {
		name += "-from-" + namespaceName
	}

	return metav1.ObjectMeta{Name: name, Namespace: serviceNamespace}
}

func egressPolicyObjectMetaFor(policyID, serviceNamespace, namespaceName string) metav1.ObjectMeta {
	name := "egress-to-" + policyID
	if serviceNamespace != namespaceName {
		name = "egress-to-" + serviceNamespace + "-" + policyID
	}

	return metav1.ObjectMeta{Name: name, Namespace: namespaceName}
}

func ingressPolicyObjectMetaWhenExposedViaIngressFor(policyID, serviceNamespace, _ string) metav1.ObjectMeta {
	name := "ingress-to-" + policyID + "-from-ingress-controller"
	return metav1.ObjectMeta{Name: name, Namespace: serviceNamespace}
}

func egressPolicyObjectMetaWhenExposedViaIngressFor(policyID, serviceNamespace, ingressControllerNamespace string) metav1.ObjectMeta {
	name := "egress-to-" + policyID
	if serviceNamespace != ingressControllerNamespace {
		name = "egress-to-" + serviceNamespace + "-" + policyID
	}

	return metav1.ObjectMeta{Name: name + "-from-ingress-controller", Namespace: ingressControllerNamespace}
}

func ingressNamespaceSelectorFor(serviceNamespace, namespaceName string) *metav1.LabelSelector {
	if serviceNamespace == namespaceName {
		return nil
	}

	return &metav1.LabelSelector{MatchLabels: map[string]string{corev1.LabelMetadataName: namespaceName}}
}

func egressNamespaceSelectorFor(serviceNamespace, namespaceName string) *metav1.LabelSelector {
	if serviceNamespace == namespaceName {
		return nil
	}

	return &metav1.LabelSelector{MatchLabels: map[string]string{corev1.LabelMetadataName: serviceNamespace}}
}

func key(meta metav1.ObjectMeta) string {
	return meta.Namespace + "/" + meta.Name
}
