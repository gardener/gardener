// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package prometheusoperator

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const clusterRoleName = "prometheus-operator"

func (p *prometheusOperator) clusterRole() *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:   clusterRoleName,
			Labels: GetLabels(),
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"monitoring.coreos.com"},
				Resources: []string{
					"alertmanagers",
					"alertmanagers/finalizers",
					"alertmanagers/status",
					"alertmanagerconfigs",
					"prometheuses",
					"prometheuses/finalizers",
					"prometheuses/status",
					"prometheusagents",
					"prometheusagents/finalizers",
					"prometheusagents/status",
					"thanosrulers",
					"thanosrulers/finalizers",
					"thanosrulers/status",
					"scrapeconfigs",
					"servicemonitors",
					"podmonitors",
					"probes",
					"prometheusrules",
				},
				Verbs: []string{"create", "get", "list", "watch", "patch", "update", "delete", "deletecollection"},
			},
			{
				APIGroups: []string{appsv1.GroupName},
				Resources: []string{"statefulsets"},
				Verbs:     []string{"create", "get", "list", "watch", "patch", "update", "delete", "deletecollection"},
			},
			{
				APIGroups: []string{corev1.GroupName},
				Resources: []string{
					"configmaps",
					"secrets",
				},
				Verbs: []string{"create", "get", "list", "watch", "patch", "update", "delete", "deletecollection"},
			},
			{
				APIGroups: []string{corev1.GroupName},
				Resources: []string{"pods"},
				Verbs:     []string{"list", "delete"},
			},
			{
				APIGroups: []string{corev1.GroupName},
				Resources: []string{
					"services",
					"services/finalizers",
					"endpoints",
				},
				Verbs: []string{"get", "create", "update", "delete"},
			},
			{
				APIGroups: []string{corev1.GroupName},
				Resources: []string{"nodes"},
				Verbs:     []string{"list", "watch"},
			},
			{
				APIGroups: []string{corev1.GroupName},
				Resources: []string{"namespaces"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{corev1.GroupName},
				Resources: []string{"events"},
				Verbs:     []string{"patch", "create"},
			},
			{
				APIGroups: []string{networkingv1.GroupName},
				Resources: []string{"ingresses"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{storagev1.GroupName},
				Resources: []string{"storageclasses"},
				Verbs:     []string{"get"},
			},
		},
	}
}

func (p *prometheusOperator) clusterRoleBinding() *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "prometheus-operator",
			Labels: GetLabels(),
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.SchemeGroupVersion.Group,
			Kind:     "ClusterRole",
			Name:     clusterRoleName,
		},
		Subjects: []rbacv1.Subject{{
			Kind:      rbacv1.ServiceAccountKind,
			Name:      serviceAccountName,
			Namespace: p.namespace,
		}},
	}
}

func (p *prometheusOperator) clusterRolePrometheus() *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "prometheus",
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{corev1.GroupName},
				Resources: []string{
					"nodes",
					"services",
					"endpoints",
					"pods",
				},
				Verbs: []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{corev1.GroupName},
				Resources: []string{
					"nodes/metrics",
					"nodes/proxy", // TODO: Remove once legacy Prometheis are gone (ones using proxy to scrape kubelets)
				},
				Verbs: []string{"get"},
			},
			{
				NonResourceURLs: []string{
					"/metrics",
					"/metrics/*", // TODO: Remove once legacy Prometheis are gone (ones using proxy to scrape kubelets)
				},
				Verbs: []string{"get"},
			},
		},
	}
}
