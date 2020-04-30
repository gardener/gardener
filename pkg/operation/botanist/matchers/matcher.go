// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package matchers

import (
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	botanistconstants "github.com/gardener/gardener/pkg/operation/botanist/constants"
	"github.com/gardener/gardener/pkg/operation/common"

	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	appsv1beta1 "k8s.io/api/apps/v1beta1"
	appsv1beta2 "k8s.io/api/apps/v1beta2"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	autoscalingv2beta1 "k8s.io/api/autoscaling/v2beta1"
	autoscalingv2beta2 "k8s.io/api/autoscaling/v2beta2"
	certificatesv1beta1 "k8s.io/api/certificates/v1beta1"
	coordinationv1 "k8s.io/api/coordination/v1"
	coordinationv1beta1 "k8s.io/api/coordination/v1beta1"
	corev1 "k8s.io/api/core/v1"
	eventsv1beta1 "k8s.io/api/events/v1beta1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	networkingv1 "k8s.io/api/networking/v1"
	networkingv1beta1 "k8s.io/api/networking/v1beta1"
	nodev1alpha1 "k8s.io/api/node/v1alpha1"
	nodev1beta1 "k8s.io/api/node/v1beta1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
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
	metricsv1alpha1 "k8s.io/metrics/pkg/apis/metrics/v1alpha1"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
)

var (
	kubeSystemLabels = labels.Set{
		common.ShootNoCleanup:            "true",
		v1beta1constants.LabelRole:       metav1.NamespaceSystem,
		v1beta1constants.GardenerPurpose: metav1.NamespaceSystem,
	}
	podsLabels = labels.Set{
		common.ShootNoCleanup:                           "true",
		botanistconstants.ManagedResourceLabelKeyOrigin: botanistconstants.ManagedResourceLabelValueGardener,
	}
	// WebhookConstraintMatchers contains a list of all api resources which can break
	// the waking up of a cluster.
	WebhookConstraintMatchers = []WebhookConstraintMatcher{
		{GVR: schema.GroupVersionResource{Group: "discovery.k8s.io", Version: "v1alpha1", Resource: "endpointslices"}, NamespaceLabels: kubeSystemLabels},
		{GVR: schema.GroupVersionResource{Group: "discovery.k8s.io", Version: "v1beta1", Resource: "endpointslices"}, NamespaceLabels: kubeSystemLabels},
		// v1 is still not available, but the APi will be promoted soon
		{GVR: schema.GroupVersionResource{Group: "discovery.k8s.io", Version: "v1", Resource: "endpointslices"}, NamespaceLabels: kubeSystemLabels},

		{GVR: corev1.SchemeGroupVersion.WithResource("pods"), NamespaceLabels: kubeSystemLabels, ObjectLabels: podsLabels},
		{GVR: corev1.SchemeGroupVersion.WithResource("pods"), NamespaceLabels: kubeSystemLabels, ObjectLabels: podsLabels, Subresource: "status"},
		{GVR: corev1.SchemeGroupVersion.WithResource("configmaps"), NamespaceLabels: kubeSystemLabels},
		{GVR: corev1.SchemeGroupVersion.WithResource("endpoints")},
		{GVR: corev1.SchemeGroupVersion.WithResource("events")},
		{GVR: corev1.SchemeGroupVersion.WithResource("secrets"), NamespaceLabels: kubeSystemLabels},
		{GVR: corev1.SchemeGroupVersion.WithResource("serviceaccounts"), NamespaceLabels: kubeSystemLabels},
		{GVR: corev1.SchemeGroupVersion.WithResource("services")},
		{GVR: corev1.SchemeGroupVersion.WithResource("services"), Subresource: "status"},
		{GVR: corev1.SchemeGroupVersion.WithResource("nodes"), ClusterScoped: true},
		{GVR: corev1.SchemeGroupVersion.WithResource("nodes"), ClusterScoped: true, Subresource: "status"},
		{GVR: corev1.SchemeGroupVersion.WithResource("namespaces"), ClusterScoped: true},
		{GVR: corev1.SchemeGroupVersion.WithResource("namespaces"), ClusterScoped: true, Subresource: "status"},
		{GVR: corev1.SchemeGroupVersion.WithResource("namespaces"), ClusterScoped: true, Subresource: "finalize"},

		{GVR: eventsv1beta1.SchemeGroupVersion.WithResource("events")},

		{GVR: appsv1.SchemeGroupVersion.WithResource("controllerrevisions"), NamespaceLabels: kubeSystemLabels},
		{GVR: appsv1.SchemeGroupVersion.WithResource("daemonsets"), NamespaceLabels: kubeSystemLabels},
		{GVR: appsv1.SchemeGroupVersion.WithResource("daemonsets"), NamespaceLabels: kubeSystemLabels, Subresource: "status"},
		{GVR: appsv1.SchemeGroupVersion.WithResource("deployments"), NamespaceLabels: kubeSystemLabels},
		{GVR: appsv1.SchemeGroupVersion.WithResource("deployments"), NamespaceLabels: kubeSystemLabels, Subresource: "scale"},
		{GVR: appsv1.SchemeGroupVersion.WithResource("replicasets"), NamespaceLabels: kubeSystemLabels},
		{GVR: appsv1.SchemeGroupVersion.WithResource("replicasets"), NamespaceLabels: kubeSystemLabels, Subresource: "status"},
		{GVR: appsv1.SchemeGroupVersion.WithResource("replicasets"), NamespaceLabels: kubeSystemLabels, Subresource: "scale"},

		{GVR: appsv1beta1.SchemeGroupVersion.WithResource("controllerrevisions"), NamespaceLabels: kubeSystemLabels},
		{GVR: appsv1beta1.SchemeGroupVersion.WithResource("daemonsets"), NamespaceLabels: kubeSystemLabels},
		{GVR: appsv1beta1.SchemeGroupVersion.WithResource("daemonsets"), NamespaceLabels: kubeSystemLabels, Subresource: "status"},
		{GVR: appsv1beta1.SchemeGroupVersion.WithResource("deployments"), NamespaceLabels: kubeSystemLabels},
		{GVR: appsv1beta1.SchemeGroupVersion.WithResource("deployments"), NamespaceLabels: kubeSystemLabels, Subresource: "scale"},
		{GVR: appsv1beta1.SchemeGroupVersion.WithResource("replicasets"), NamespaceLabels: kubeSystemLabels},
		{GVR: appsv1beta1.SchemeGroupVersion.WithResource("replicasets"), NamespaceLabels: kubeSystemLabels, Subresource: "status"},
		{GVR: appsv1beta1.SchemeGroupVersion.WithResource("replicasets"), NamespaceLabels: kubeSystemLabels, Subresource: "scale"},

		{GVR: appsv1beta2.SchemeGroupVersion.WithResource("controllerrevisions"), NamespaceLabels: kubeSystemLabels},
		{GVR: appsv1beta2.SchemeGroupVersion.WithResource("daemonsets"), NamespaceLabels: kubeSystemLabels},
		{GVR: appsv1beta2.SchemeGroupVersion.WithResource("daemonsets"), NamespaceLabels: kubeSystemLabels, Subresource: "status"},
		{GVR: appsv1beta2.SchemeGroupVersion.WithResource("deployments"), NamespaceLabels: kubeSystemLabels},
		{GVR: appsv1beta2.SchemeGroupVersion.WithResource("deployments"), NamespaceLabels: kubeSystemLabels, Subresource: "scale"},
		{GVR: appsv1beta2.SchemeGroupVersion.WithResource("replicasets"), NamespaceLabels: kubeSystemLabels},
		{GVR: appsv1beta2.SchemeGroupVersion.WithResource("replicasets"), NamespaceLabels: kubeSystemLabels, Subresource: "status"},
		{GVR: appsv1beta2.SchemeGroupVersion.WithResource("replicasets"), NamespaceLabels: kubeSystemLabels, Subresource: "scale"},

		{GVR: extensionsv1beta1.SchemeGroupVersion.WithResource("controllerrevisions"), NamespaceLabels: kubeSystemLabels},
		{GVR: extensionsv1beta1.SchemeGroupVersion.WithResource("daemonsets"), NamespaceLabels: kubeSystemLabels},
		{GVR: extensionsv1beta1.SchemeGroupVersion.WithResource("daemonsets"), NamespaceLabels: kubeSystemLabels, Subresource: "status"},
		{GVR: extensionsv1beta1.SchemeGroupVersion.WithResource("deployments"), NamespaceLabels: kubeSystemLabels},
		{GVR: extensionsv1beta1.SchemeGroupVersion.WithResource("deployments"), NamespaceLabels: kubeSystemLabels, Subresource: "scale"},
		{GVR: extensionsv1beta1.SchemeGroupVersion.WithResource("replicasets"), NamespaceLabels: kubeSystemLabels},
		{GVR: extensionsv1beta1.SchemeGroupVersion.WithResource("replicasets"), NamespaceLabels: kubeSystemLabels, Subresource: "status"},
		{GVR: extensionsv1beta1.SchemeGroupVersion.WithResource("replicasets"), NamespaceLabels: kubeSystemLabels, Subresource: "scale"},
		{GVR: extensionsv1beta1.SchemeGroupVersion.WithResource("networkpolicies"), NamespaceLabels: kubeSystemLabels},
		{GVR: extensionsv1beta1.SchemeGroupVersion.WithResource("podsecuritypolicies"), ClusterScoped: true},

		{GVR: coordinationv1.SchemeGroupVersion.WithResource("leases")},
		{GVR: coordinationv1beta1.SchemeGroupVersion.WithResource("leases")},

		{GVR: metricsv1alpha1.SchemeGroupVersion.WithResource("podmetrics"), NamespaceLabels: kubeSystemLabels},
		{GVR: metricsv1alpha1.SchemeGroupVersion.WithResource("nodemetrics"), NamespaceLabels: kubeSystemLabels},

		{GVR: metricsv1beta1.SchemeGroupVersion.WithResource("podmetrics"), NamespaceLabels: kubeSystemLabels},
		{GVR: metricsv1beta1.SchemeGroupVersion.WithResource("nodemetrics"), NamespaceLabels: kubeSystemLabels},

		{GVR: networkingv1.SchemeGroupVersion.WithResource("networkpolicies"), NamespaceLabels: kubeSystemLabels},
		{GVR: networkingv1beta1.SchemeGroupVersion.WithResource("networkpolicies"), NamespaceLabels: kubeSystemLabels},

		{GVR: policyv1beta1.SchemeGroupVersion.WithResource("podsecuritypolicies"), ClusterScoped: true},

		{GVR: autoscalingv1.SchemeGroupVersion.WithResource("horizontalpodautoscalers"), NamespaceLabels: kubeSystemLabels},
		{GVR: autoscalingv1.SchemeGroupVersion.WithResource("horizontalpodautoscalers"), NamespaceLabels: kubeSystemLabels, Subresource: "status"},

		{GVR: autoscalingv2beta1.SchemeGroupVersion.WithResource("horizontalpodautoscalers"), NamespaceLabels: kubeSystemLabels},
		{GVR: autoscalingv2beta1.SchemeGroupVersion.WithResource("horizontalpodautoscalers"), NamespaceLabels: kubeSystemLabels, Subresource: "status"},

		{GVR: autoscalingv2beta2.SchemeGroupVersion.WithResource("horizontalpodautoscalers"), NamespaceLabels: kubeSystemLabels},
		{GVR: autoscalingv2beta2.SchemeGroupVersion.WithResource("horizontalpodautoscalers"), NamespaceLabels: kubeSystemLabels, Subresource: "status"},

		{GVR: rbacv1.SchemeGroupVersion.WithResource("clusterroles"), ClusterScoped: true},
		{GVR: rbacv1.SchemeGroupVersion.WithResource("clusterrolebindings"), ClusterScoped: true},
		{GVR: rbacv1.SchemeGroupVersion.WithResource("roles"), NamespaceLabels: kubeSystemLabels},
		{GVR: rbacv1.SchemeGroupVersion.WithResource("rolebindings"), NamespaceLabels: kubeSystemLabels},

		{GVR: rbacv1alpha1.SchemeGroupVersion.WithResource("clusterroles"), ClusterScoped: true},
		{GVR: rbacv1alpha1.SchemeGroupVersion.WithResource("clusterrolebindings"), ClusterScoped: true},
		{GVR: rbacv1alpha1.SchemeGroupVersion.WithResource("roles"), NamespaceLabels: kubeSystemLabels},
		{GVR: rbacv1alpha1.SchemeGroupVersion.WithResource("rolebindings"), NamespaceLabels: kubeSystemLabels},

		{GVR: rbacv1beta1.SchemeGroupVersion.WithResource("clusterroles"), ClusterScoped: true},
		{GVR: rbacv1beta1.SchemeGroupVersion.WithResource("clusterrolebindings"), ClusterScoped: true},
		{GVR: rbacv1beta1.SchemeGroupVersion.WithResource("roles"), NamespaceLabels: kubeSystemLabels},
		{GVR: rbacv1beta1.SchemeGroupVersion.WithResource("rolebindings"), NamespaceLabels: kubeSystemLabels},

		{GVR: apiextensionsv1.SchemeGroupVersion.WithResource("customresourcedefinitions"), ClusterScoped: true},
		{GVR: apiextensionsv1.SchemeGroupVersion.WithResource("customresourcedefinitions"), ClusterScoped: true, Subresource: "status"},

		{GVR: apiextensionsv1beta1.SchemeGroupVersion.WithResource("customresourcedefinitions"), ClusterScoped: true},
		{GVR: apiextensionsv1beta1.SchemeGroupVersion.WithResource("customresourcedefinitions"), ClusterScoped: true, Subresource: "status"},

		{GVR: apiregistrationv1.SchemeGroupVersion.WithResource("apiservices"), ClusterScoped: true},
		{GVR: apiregistrationv1.SchemeGroupVersion.WithResource("apiservices"), ClusterScoped: true, Subresource: "status"},

		{GVR: apiregistrationv1beta1.SchemeGroupVersion.WithResource("apiservices"), ClusterScoped: true},
		{GVR: apiregistrationv1beta1.SchemeGroupVersion.WithResource("apiservices"), ClusterScoped: true, Subresource: "status"},

		{GVR: certificatesv1beta1.SchemeGroupVersion.WithResource("certificatesigningrequests"), ClusterScoped: true},
		{GVR: certificatesv1beta1.SchemeGroupVersion.WithResource("certificatesigningrequests"), ClusterScoped: true, Subresource: "status"},
		{GVR: certificatesv1beta1.SchemeGroupVersion.WithResource("certificatesigningrequests"), ClusterScoped: true, Subresource: "approval"},

		{GVR: nodev1alpha1.SchemeGroupVersion.WithResource("runtimeclasses"), ClusterScoped: true},
		{GVR: nodev1beta1.SchemeGroupVersion.WithResource("runtimeclasses"), ClusterScoped: true},

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
	ObjectLabels    labels.Labels
	NamespaceLabels labels.Labels
}

// Match rule with objLabelSelector and namespaceLabelSelector if
// the resource is not namespaced.
func (w *WebhookConstraintMatcher) Match(
	r admissionregistrationv1beta1.RuleWithOperations,
	objLabelSelector *metav1.LabelSelector,
	namespaceLabelSelector *metav1.LabelSelector,
) bool {
	var (
		objLabels = w.ObjectLabels
		nsLabels  = w.NamespaceLabels
	)

	if objLabels == nil {
		objLabels = labels.Set{}
	}

	if nsLabels == nil {
		nsLabels = labels.Set{}
	}

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

	matchObj := objSelector.Empty() || objSelector.Matches(objLabels)
	matchNS := nsSelector.Empty() || nsSelector.Matches(nsLabels)

	rm := ruleMatcher{rule: r, gvr: w.GVR, subresource: w.Subresource}
	if !w.ClusterScoped {
		rm.namespace = "dummy"
	}

	return matchObj && (w.ClusterScoped || matchNS) && rm.Matches()
}
