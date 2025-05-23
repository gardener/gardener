// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package matchers

import (
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	appsv1beta1 "k8s.io/api/apps/v1beta1"
	appsv1beta2 "k8s.io/api/apps/v1beta2"
	certificatesv1 "k8s.io/api/certificates/v1"
	coordinationv1 "k8s.io/api/coordination/v1"
	coordinationv1beta1 "k8s.io/api/coordination/v1beta1"
	corev1 "k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	networkingv1 "k8s.io/api/networking/v1"
	networkingv1beta1 "k8s.io/api/networking/v1beta1"
	rbacv1 "k8s.io/api/rbac/v1"
	rbacv1alpha1 "k8s.io/api/rbac/v1alpha1"
	rbacv1beta1 "k8s.io/api/rbac/v1beta1"
	schedulingv1 "k8s.io/api/scheduling/v1"
	schedulingv1alpha1 "k8s.io/api/scheduling/v1alpha1"
	schedulingv1beta1 "k8s.io/api/scheduling/v1beta1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	apiregistrationv1beta1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1beta1"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/utils/managedresources"
)

var (
	defaultNamespaceLabels = labels.Set{
		corev1.LabelMetadataName: metav1.NamespaceDefault,
	}

	kubeSystemNamespaceLabels = labels.Set{
		v1beta1constants.ShootNoCleanup:  "true",
		v1beta1constants.GardenerPurpose: metav1.NamespaceSystem,
		corev1.LabelMetadataName:         metav1.NamespaceSystem,
	}

	podsLabels = labels.Set{
		v1beta1constants.ShootNoCleanup: "true",
		managedresources.LabelKeyOrigin: managedresources.LabelValueGardener,
	}

	// WebhookConstraintMatchersForLeases contains a list of lease API resources that can break
	// leader election of essential control plane controllers.
	WebhookConstraintMatchersForLeases = []WebhookConstraintMatcher{
		// object selector here is added to support the use case described in - https://github.com/gardener/gardener/pull/8034
		{GVR: coordinationv1.SchemeGroupVersion.WithResource("leases"), NamespaceLabels: kubeSystemNamespaceLabels, ObjectLabels: labels.Set{}},
		{GVR: coordinationv1beta1.SchemeGroupVersion.WithResource("leases"), NamespaceLabels: kubeSystemNamespaceLabels, ObjectLabels: labels.Set{}},
	}

	// WebhookConstraintMatchers contains a list of all api resources which can break
	// the waking up of a cluster.
	WebhookConstraintMatchers = []WebhookConstraintMatcher{
		{GVR: corev1.SchemeGroupVersion.WithResource("pods"), NamespaceLabels: kubeSystemNamespaceLabels, ObjectLabels: podsLabels},
		{GVR: corev1.SchemeGroupVersion.WithResource("pods"), NamespaceLabels: kubeSystemNamespaceLabels, ObjectLabels: podsLabels, Subresource: "status"},

		// Leader election of kube-controller-manager, kube-scheduler, cloud-controller-manager, cluster-autoscaler, ...
		{GVR: corev1.SchemeGroupVersion.WithResource("configmaps"), NamespaceLabels: kubeSystemNamespaceLabels},
		// kube-system and default namespaces for leader election and apiserver in-cluster discovery.
		{GVR: corev1.SchemeGroupVersion.WithResource("endpoints"), NamespaceLabels: defaultNamespaceLabels},
		{GVR: corev1.SchemeGroupVersion.WithResource("endpoints"), NamespaceLabels: kubeSystemNamespaceLabels},

		{GVR: corev1.SchemeGroupVersion.WithResource("secrets"), NamespaceLabels: kubeSystemNamespaceLabels},
		{GVR: corev1.SchemeGroupVersion.WithResource("serviceaccounts"), NamespaceLabels: kubeSystemNamespaceLabels},

		{GVR: corev1.SchemeGroupVersion.WithResource("services"), NamespaceLabels: kubeSystemNamespaceLabels},
		{GVR: corev1.SchemeGroupVersion.WithResource("services"), NamespaceLabels: kubeSystemNamespaceLabels, Subresource: "status"},

		// Match services in the `default` namespace because the `kubernetes` service is essential for in-cluster communication.
		// Unfortunately, it's not possible to restrict the match to this specific service since a remediated webhook on services
		// would widen the scope, i.e. the webhook will have a combination of namespace and object selectors.
		{GVR: corev1.SchemeGroupVersion.WithResource("services"), NamespaceLabels: defaultNamespaceLabels},
		{GVR: corev1.SchemeGroupVersion.WithResource("services"), NamespaceLabels: defaultNamespaceLabels, Subresource: "status"},

		// Kubelet must be allowed to register itself.
		{GVR: corev1.SchemeGroupVersion.WithResource("nodes"), ClusterScoped: true},
		{GVR: corev1.SchemeGroupVersion.WithResource("nodes"), ClusterScoped: true, Subresource: "status"},

		// Needed for gardener-resource-manager to update "kube-system" namespace labels.
		{GVR: corev1.SchemeGroupVersion.WithResource("namespaces"), ClusterScoped: true, ObjectLabels: kubeSystemNamespaceLabels, NamespaceLabels: kubeSystemNamespaceLabels},
		{GVR: corev1.SchemeGroupVersion.WithResource("namespaces"), ClusterScoped: true, ObjectLabels: kubeSystemNamespaceLabels, NamespaceLabels: kubeSystemNamespaceLabels, Subresource: "status"},

		{GVR: appsv1.SchemeGroupVersion.WithResource("controllerrevisions"), NamespaceLabels: kubeSystemNamespaceLabels},
		{GVR: appsv1.SchemeGroupVersion.WithResource("daemonsets"), NamespaceLabels: kubeSystemNamespaceLabels},
		{GVR: appsv1.SchemeGroupVersion.WithResource("daemonsets"), NamespaceLabels: kubeSystemNamespaceLabels, Subresource: "status"},
		{GVR: appsv1.SchemeGroupVersion.WithResource("deployments"), NamespaceLabels: kubeSystemNamespaceLabels},
		{GVR: appsv1.SchemeGroupVersion.WithResource("deployments"), NamespaceLabels: kubeSystemNamespaceLabels, Subresource: "scale"},
		{GVR: appsv1.SchemeGroupVersion.WithResource("replicasets"), NamespaceLabels: kubeSystemNamespaceLabels},
		{GVR: appsv1.SchemeGroupVersion.WithResource("replicasets"), NamespaceLabels: kubeSystemNamespaceLabels, Subresource: "status"},
		{GVR: appsv1.SchemeGroupVersion.WithResource("replicasets"), NamespaceLabels: kubeSystemNamespaceLabels, Subresource: "scale"},

		{GVR: appsv1beta1.SchemeGroupVersion.WithResource("controllerrevisions"), NamespaceLabels: kubeSystemNamespaceLabels},
		{GVR: appsv1beta1.SchemeGroupVersion.WithResource("daemonsets"), NamespaceLabels: kubeSystemNamespaceLabels},
		{GVR: appsv1beta1.SchemeGroupVersion.WithResource("daemonsets"), NamespaceLabels: kubeSystemNamespaceLabels, Subresource: "status"},
		{GVR: appsv1beta1.SchemeGroupVersion.WithResource("deployments"), NamespaceLabels: kubeSystemNamespaceLabels},
		{GVR: appsv1beta1.SchemeGroupVersion.WithResource("deployments"), NamespaceLabels: kubeSystemNamespaceLabels, Subresource: "scale"},
		{GVR: appsv1beta1.SchemeGroupVersion.WithResource("replicasets"), NamespaceLabels: kubeSystemNamespaceLabels},
		{GVR: appsv1beta1.SchemeGroupVersion.WithResource("replicasets"), NamespaceLabels: kubeSystemNamespaceLabels, Subresource: "status"},
		{GVR: appsv1beta1.SchemeGroupVersion.WithResource("replicasets"), NamespaceLabels: kubeSystemNamespaceLabels, Subresource: "scale"},

		{GVR: appsv1beta2.SchemeGroupVersion.WithResource("controllerrevisions"), NamespaceLabels: kubeSystemNamespaceLabels},
		{GVR: appsv1beta2.SchemeGroupVersion.WithResource("daemonsets"), NamespaceLabels: kubeSystemNamespaceLabels},
		{GVR: appsv1beta2.SchemeGroupVersion.WithResource("daemonsets"), NamespaceLabels: kubeSystemNamespaceLabels, Subresource: "status"},
		{GVR: appsv1beta2.SchemeGroupVersion.WithResource("deployments"), NamespaceLabels: kubeSystemNamespaceLabels},
		{GVR: appsv1beta2.SchemeGroupVersion.WithResource("deployments"), NamespaceLabels: kubeSystemNamespaceLabels, Subresource: "scale"},
		{GVR: appsv1beta2.SchemeGroupVersion.WithResource("replicasets"), NamespaceLabels: kubeSystemNamespaceLabels},
		{GVR: appsv1beta2.SchemeGroupVersion.WithResource("replicasets"), NamespaceLabels: kubeSystemNamespaceLabels, Subresource: "status"},
		{GVR: appsv1beta2.SchemeGroupVersion.WithResource("replicasets"), NamespaceLabels: kubeSystemNamespaceLabels, Subresource: "scale"},

		{GVR: extensionsv1beta1.SchemeGroupVersion.WithResource("controllerrevisions"), NamespaceLabels: kubeSystemNamespaceLabels},
		{GVR: extensionsv1beta1.SchemeGroupVersion.WithResource("daemonsets"), NamespaceLabels: kubeSystemNamespaceLabels},
		{GVR: extensionsv1beta1.SchemeGroupVersion.WithResource("daemonsets"), NamespaceLabels: kubeSystemNamespaceLabels, Subresource: "status"},
		{GVR: extensionsv1beta1.SchemeGroupVersion.WithResource("deployments"), NamespaceLabels: kubeSystemNamespaceLabels},
		{GVR: extensionsv1beta1.SchemeGroupVersion.WithResource("deployments"), NamespaceLabels: kubeSystemNamespaceLabels, Subresource: "scale"},
		{GVR: extensionsv1beta1.SchemeGroupVersion.WithResource("replicasets"), NamespaceLabels: kubeSystemNamespaceLabels},
		{GVR: extensionsv1beta1.SchemeGroupVersion.WithResource("replicasets"), NamespaceLabels: kubeSystemNamespaceLabels, Subresource: "status"},
		{GVR: extensionsv1beta1.SchemeGroupVersion.WithResource("replicasets"), NamespaceLabels: kubeSystemNamespaceLabels, Subresource: "scale"},
		{GVR: extensionsv1beta1.SchemeGroupVersion.WithResource("networkpolicies"), NamespaceLabels: kubeSystemNamespaceLabels},

		// Needed for kubelet and kube-system controllers leader election.
		{GVR: coordinationv1.SchemeGroupVersion.WithResource("leases")},
		{GVR: coordinationv1beta1.SchemeGroupVersion.WithResource("leases")},

		// Modifications might be needed for old clusters with new policies.
		{GVR: networkingv1.SchemeGroupVersion.WithResource("networkpolicies"), NamespaceLabels: kubeSystemNamespaceLabels},
		{GVR: networkingv1beta1.SchemeGroupVersion.WithResource("networkpolicies"), NamespaceLabels: kubeSystemNamespaceLabels},

		// Needed as part of /readyz/poststarthook/rbac/bootstrap-roles in kube-apiserver.
		{GVR: rbacv1.SchemeGroupVersion.WithResource("clusterroles"), ClusterScoped: true},
		{GVR: rbacv1.SchemeGroupVersion.WithResource("clusterrolebindings"), ClusterScoped: true},
		{GVR: rbacv1.SchemeGroupVersion.WithResource("roles"), NamespaceLabels: kubeSystemNamespaceLabels},
		{GVR: rbacv1.SchemeGroupVersion.WithResource("rolebindings"), NamespaceLabels: kubeSystemNamespaceLabels},

		{GVR: rbacv1alpha1.SchemeGroupVersion.WithResource("clusterroles"), ClusterScoped: true},
		{GVR: rbacv1alpha1.SchemeGroupVersion.WithResource("clusterrolebindings"), ClusterScoped: true},
		{GVR: rbacv1alpha1.SchemeGroupVersion.WithResource("roles"), NamespaceLabels: kubeSystemNamespaceLabels},
		{GVR: rbacv1alpha1.SchemeGroupVersion.WithResource("rolebindings"), NamespaceLabels: kubeSystemNamespaceLabels},

		{GVR: rbacv1beta1.SchemeGroupVersion.WithResource("clusterroles"), ClusterScoped: true},
		{GVR: rbacv1beta1.SchemeGroupVersion.WithResource("clusterrolebindings"), ClusterScoped: true},
		{GVR: rbacv1beta1.SchemeGroupVersion.WithResource("roles"), NamespaceLabels: kubeSystemNamespaceLabels},
		{GVR: rbacv1beta1.SchemeGroupVersion.WithResource("rolebindings"), NamespaceLabels: kubeSystemNamespaceLabels},

		// Needed for networking extensions.
		{GVR: apiextensionsv1.SchemeGroupVersion.WithResource("customresourcedefinitions"), ClusterScoped: true},
		{GVR: apiextensionsv1.SchemeGroupVersion.WithResource("customresourcedefinitions"), ClusterScoped: true, Subresource: "status"},

		{GVR: apiextensionsv1beta1.SchemeGroupVersion.WithResource("customresourcedefinitions"), ClusterScoped: true},
		{GVR: apiextensionsv1beta1.SchemeGroupVersion.WithResource("customresourcedefinitions"), ClusterScoped: true, Subresource: "status"},

		// Needed as part of /healthz/poststarthook/apiservice-openapi-controller in kube-apiserver.
		{GVR: apiregistrationv1.SchemeGroupVersion.WithResource("apiservices"), ClusterScoped: true},
		{GVR: apiregistrationv1.SchemeGroupVersion.WithResource("apiservices"), ClusterScoped: true, Subresource: "status"},

		{GVR: apiregistrationv1beta1.SchemeGroupVersion.WithResource("apiservices"), ClusterScoped: true},
		{GVR: apiregistrationv1beta1.SchemeGroupVersion.WithResource("apiservices"), ClusterScoped: true, Subresource: "status"},

		// Kubelet uses it to request a certificate for itself.
		{GVR: certificatesv1.SchemeGroupVersion.WithResource("certificatesigningrequests"), ClusterScoped: true},
		{GVR: certificatesv1.SchemeGroupVersion.WithResource("certificatesigningrequests"), ClusterScoped: true, Subresource: "status"},
		{GVR: certificatesv1.SchemeGroupVersion.WithResource("certificatesigningrequests"), ClusterScoped: true, Subresource: "approval"},

		// Needed as part of /healthz/poststarthook/scheduling/bootstrap-system-priority-classes in kube-apiserver.
		{GVR: schedulingv1.SchemeGroupVersion.WithResource("priorityclasses"), ClusterScoped: true},
		{GVR: schedulingv1alpha1.SchemeGroupVersion.WithResource("priorityclasses"), ClusterScoped: true},
		{GVR: schedulingv1beta1.SchemeGroupVersion.WithResource("priorityclasses"), ClusterScoped: true},
	}
)

// selectors in webhooks are defaulted to matchAll selector.
func defaultEmptySelector(ls *metav1.LabelSelector) (labels.Selector, error) {
	if ls == nil {
		ls = &metav1.LabelSelector{}
	}

	return metav1.LabelSelectorAsSelector(ls)
}

// WebhookConstraintMatcher contains an api resource matcher.
type WebhookConstraintMatcher struct {
	GVR             schema.GroupVersionResource
	Subresource     string
	ClusterScoped   bool
	ObjectLabels    labels.Set
	NamespaceLabels labels.Set
}

// Match rule with objLabelSelector and namespaceLabelSelector if
// the resource is not namespaced.
func (w *WebhookConstraintMatcher) Match(
	r admissionregistrationv1.RuleWithOperations,
	objLabelSelector *metav1.LabelSelector,
	namespaceLabelSelector *metav1.LabelSelector,
) bool {
	var (
		objLabels = w.ObjectLabels
		nsLabels  = w.NamespaceLabels

		matchAllObjects    = objLabels == nil
		matchAllNamespaces = nsLabels == nil
	)

	nsSelector, err := defaultEmptySelector(namespaceLabelSelector)
	if err != nil {
		// this should really not happen
		return true
	}

	objSelector, err := defaultEmptySelector(objLabelSelector)
	if err != nil {
		// this should really not happen
		return true
	}

	matchObj := matchAllObjects || objSelector.Empty() || objSelector.Matches(objLabels)
	matchNS := matchAllNamespaces || nsSelector.Empty() || nsSelector.Matches(nsLabels)

	rm := ruleMatcher{rule: r, gvr: w.GVR, subresource: w.Subresource}
	if !w.ClusterScoped {
		rm.namespace = "dummy"
	}

	// namespaceSelector can be used to select namespace objects (although the namespace is cluster-scoped resource)
	if w.GVR == namespaceResource {
		return matchObj && matchNS && rm.Matches()
	}

	return matchObj && (w.ClusterScoped || matchNS) && rm.Matches()
}
