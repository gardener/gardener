// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package persesoperator

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	clusterRoleName        = "perses-operator"
	leaderElectionRoleName = "perses-operator-leader-election"
)

func (p *persesOperator) clusterRole() *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:   clusterRoleName,
			Labels: GetLabels(),
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{appsv1.GroupName},
				Resources: []string{"deployments", "statefulsets"},
				Verbs:     []string{"create", "delete", "get", "list", "patch", "update", "watch"},
			},
			{
				APIGroups: []string{corev1.GroupName},
				Resources: []string{"events"},
				Verbs:     []string{"create", "patch"},
			},
			{
				APIGroups: []string{corev1.GroupName},
				Resources: []string{"services", "configmaps"},
				Verbs:     []string{"create", "delete", "get", "list", "patch", "update", "watch"},
			},
			{
				APIGroups: []string{"perses.dev"},
				Resources: []string{"perses"},
				Verbs:     []string{"create", "delete", "get", "list", "patch", "update", "watch"},
			},
			{
				APIGroups: []string{"perses.dev"},
				Resources: []string{"perses/finalizers"},
				Verbs:     []string{"update"},
			},
			{
				APIGroups: []string{"perses.dev"},
				Resources: []string{"perses/status"},
				Verbs:     []string{"get", "patch", "update"},
			},
			{
				APIGroups: []string{"perses.dev"},
				Resources: []string{"persesdashboards"},
				Verbs:     []string{"create", "delete", "get", "list", "patch", "update", "watch"},
			},
			{
				APIGroups: []string{"perses.dev"},
				Resources: []string{"persesdashboards/finalizers"},
				Verbs:     []string{"update"},
			},
			{
				APIGroups: []string{"perses.dev"},
				Resources: []string{"persesdashboards/status"},
				Verbs:     []string{"get", "patch", "update"},
			},
			{
				APIGroups: []string{"perses.dev"},
				Resources: []string{"persesdatasources"},
				Verbs:     []string{"create", "delete", "get", "list", "patch", "update", "watch"},
			},
			{
				APIGroups: []string{"perses.dev"},
				Resources: []string{"persesdatasources/finalizers"},
				Verbs:     []string{"update"},
			},
			{
				APIGroups: []string{"perses.dev"},
				Resources: []string{"persesdatasources/status"},
				Verbs:     []string{"get", "patch", "update"},
			},
		},
	}
}

func (p *persesOperator) clusterRoleBinding() *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:   clusterRoleName,
			Labels: GetLabels(),
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     clusterRoleName,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      serviceAccountName,
				Namespace: p.namespace,
			},
		},
	}
}
