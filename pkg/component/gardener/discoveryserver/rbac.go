// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package discoveryserver

import (
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

const (
	clusterRoleName        = "gardener.cloud:system:discovery-server"
	clusterRoleBindingName = "gardener.cloud:system:discovery-server"

	roleName        = "gardener.cloud:system:discovery-server"
	roleBindingName = "gardener.cloud:system:discovery-server"
)

func (g *gardenerDiscoveryServer) clusterRole() *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:   clusterRoleName,
			Labels: labels(),
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{gardencorev1beta1.GroupName},
				Resources: []string{"projects", "shoots"},
				Verbs:     []string{"get", "watch", "list"},
			},
			{
				APIGroups: []string{corev1.GroupName},
				Resources: []string{"configmaps", "namespaces"},
				Verbs:     []string{"get", "list", "watch"},
			},
		},
	}
}

func (g *gardenerDiscoveryServer) clusterRoleBinding(serviceAccountName string) *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:   clusterRoleBindingName,
			Labels: labels(),
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

func (g *gardenerDiscoveryServer) role() *rbacv1.Role {
	return &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      roleName,
			Namespace: gardencorev1beta1.GardenerShootIssuerNamespace,
			Labels:    labels(),
		},
		Rules: []rbacv1.PolicyRule{{
			APIGroups: []string{corev1.GroupName},
			Resources: []string{"secrets"},
			Verbs:     []string{"get", "watch", "list"},
		}},
	}
}

func (g *gardenerDiscoveryServer) roleBinding(serviceAccountName string) *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      roleBindingName,
			Namespace: gardencorev1beta1.GardenerShootIssuerNamespace,
			Labels:    labels(),
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "Role",
			Name:     roleName,
		},
		Subjects: []rbacv1.Subject{{
			Kind:      "ServiceAccount",
			Name:      serviceAccountName,
			Namespace: metav1.NamespaceSystem,
		}},
	}
}
