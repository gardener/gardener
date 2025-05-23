// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package dashboard

import (
	authenticationv1 "k8s.io/api/authentication/v1"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiregistrationv1beta1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1beta1"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/utils"
)

const (
	clusterRoleName        = "gardener.cloud:system:dashboard"
	clusterRoleBindingName = "gardener.cloud:system:dashboard"

	clusterRoleTerminalProjectMemberName = "dashboard.gardener.cloud:system:project-member"
	clusterRoleBindingTerminalName       = "gardener.cloud:dashboard-terminal:admin"

	gitHubWebhookRoleName        = "gardener.cloud:system:dashboard-github-webhook"
	gitHubWebhookRoleBindingName = "gardener.cloud:system:dashboard-github-webhook"
)

func (g *gardenerDashboard) clusterRole() *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:   clusterRoleName,
			Labels: GetLabels(),
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{authenticationv1.GroupName},
				Resources: []string{"tokenreviews"},
				Verbs:     []string{"create"},
			},
			{
				APIGroups: []string{gardencorev1beta1.GroupName},
				Resources: []string{"quotas", "projects", "shoots", "controllerregistrations"},
				Verbs:     []string{"list", "watch"},
			},
			{
				APIGroups: []string{apiregistrationv1beta1.GroupName},
				Resources: []string{"apiservices"},
				Verbs:     []string{"get"},
			},
			{
				APIGroups:     []string{corev1.GroupName},
				Resources:     []string{"configmaps"},
				Verbs:         []string{"get"},
				ResourceNames: []string{v1beta1constants.ClusterIdentity},
			},
			{
				APIGroups: []string{corev1.GroupName},
				Resources: []string{"resourcequotas"},
				Verbs:     []string{"list", "watch"},
			},
		},
	}
}

func (g *gardenerDashboard) clusterRoleBinding(serviceAccountName string) *rbacv1.ClusterRoleBinding {
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

func (g *gardenerDashboard) clusterRoleBindingTerminal() *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:   clusterRoleBindingTerminalName,
			Labels: GetLabels(),
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     v1beta1constants.ClusterRoleNameGardenerAdministrators,
		},
		Subjects: []rbacv1.Subject{{
			Kind:      "ServiceAccount",
			Name:      serviceAccountNameTerminal,
			Namespace: metav1.NamespaceSystem,
		}},
	}
}

func (g *gardenerDashboard) clusterRoleTerminalProjectMember() *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:   clusterRoleTerminalProjectMemberName,
			Labels: utils.MergeStringMaps(GetLabels(), map[string]string{v1beta1constants.LabelKeyAggregateToProjectMember: "true"}),
		},
		Rules: []rbacv1.PolicyRule{{
			APIGroups: []string{"dashboard.gardener.cloud"},
			Resources: []string{"terminals"},
			Verbs:     []string{"create", "delete", "deletecollection", "get", "list", "watch", "patch", "update"},
		}},
	}
}

func (g *gardenerDashboard) role() *rbacv1.Role {
	return &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name: gitHubWebhookRoleName,
			// Must be in 'garden' namespace, see https://github.com/gardener/gardener/pull/9583#discussion_r1572529328
			Namespace: v1beta1constants.GardenNamespace,
			Labels:    GetLabels(),
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups:     []string{coordinationv1.GroupName},
				Resources:     []string{"leases"},
				ResourceNames: []string{"gardener-dashboard-github-webhook"},
				Verbs:         []string{"get", "patch", "watch", "list"},
			},
			{
				APIGroups: []string{coordinationv1.GroupName},
				Resources: []string{"leases"},
				Verbs:     []string{"create"},
			},
		},
	}
}

func (g *gardenerDashboard) roleBinding(serviceAccountName string) *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: gitHubWebhookRoleBindingName,
			// Must be in 'garden' namespace, see https://github.com/gardener/gardener/pull/9583#discussion_r1572529328
			Namespace: v1beta1constants.GardenNamespace,
			Labels:    GetLabels(),
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "Role",
			Name:     gitHubWebhookRoleName,
		},
		Subjects: []rbacv1.Subject{{
			Kind:      "ServiceAccount",
			Name:      serviceAccountName,
			Namespace: metav1.NamespaceSystem,
		}},
	}
}
