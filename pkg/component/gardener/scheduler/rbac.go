// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package scheduler

import (
	coordinationv1beta1 "k8s.io/api/coordination/v1beta1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	schedulerconfigv1alpha1 "github.com/gardener/gardener/pkg/scheduler/apis/config/v1alpha1"
)

const (
	clusterRoleName        = "gardener.cloud:system:scheduler"
	clusterRoleBindingName = "gardener.cloud:system:scheduler"
)

func (g *gardenerScheduler) clusterRole() *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:   clusterRoleName,
			Labels: GetLabels(),
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"events"},
				Verbs:     []string{"create", "patch", "update"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"configmaps"},
				Verbs:     []string{"create", "delete", "get", "list", "watch", "patch", "update"},
			},
			{
				APIGroups: []string{gardencorev1beta1.GroupName},
				Resources: []string{
					"cloudprofiles",
					"namespacedcloudprofiles",
					"seeds",
				},
				Verbs: []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{gardencorev1beta1.GroupName},
				Resources: []string{
					"shoots",
					"shoots/status",
				},
				Verbs: []string{"get", "list", "watch", "patch", "update"},
			},
			{
				APIGroups: []string{gardencorev1beta1.GroupName},
				Resources: []string{
					"shoots/binding",
				},
				Verbs: []string{"update"},
			},
			{
				APIGroups: []string{coordinationv1beta1.GroupName},
				Resources: []string{
					"leases",
				},
				Verbs: []string{"create"},
			},
			{
				APIGroups: []string{coordinationv1beta1.GroupName},
				Resources: []string{
					"leases",
				},
				ResourceNames: []string{
					schedulerconfigv1alpha1.SchedulerDefaultLockObjectName,
				},
				Verbs: []string{"get", "watch", "update"},
			},
		},
	}
}

func (g *gardenerScheduler) clusterRoleBinding(serviceAccountName string) *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:   clusterRoleBindingName,
			Labels: GetLabels(),
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     clusterRoleName,
		},
		Subjects: []rbacv1.Subject{{
			Kind:      "ServiceAccount",
			Name:      serviceAccountName,
			Namespace: metav1.NamespaceSystem,
		}},
	}
}
