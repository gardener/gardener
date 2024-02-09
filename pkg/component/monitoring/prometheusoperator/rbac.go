// Copyright 2024 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
				Verbs: []string{rbacv1.VerbAll},
			},
			{
				APIGroups: []string{appsv1.GroupName},
				Resources: []string{"statefulsets"},
				Verbs:     []string{rbacv1.VerbAll},
			},
			{
				APIGroups: []string{corev1.GroupName},
				Resources: []string{
					"configmaps",
					"secrets",
				},
				Verbs: []string{rbacv1.VerbAll},
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
