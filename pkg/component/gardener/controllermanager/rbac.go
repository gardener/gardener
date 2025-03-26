// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controllermanager

import (
	authorizationv1 "k8s.io/api/authorization/v1"
	certificatesv1 "k8s.io/api/certificates/v1"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	eventsv1 "k8s.io/api/events/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	operationsv1alpha1 "github.com/gardener/gardener/pkg/apis/operations/v1alpha1"
	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	settingsv1alpha1 "github.com/gardener/gardener/pkg/apis/settings/v1alpha1"
)

const (
	clusterRoleName        = "gardener.cloud:system:controller-manager"
	clusterRoleBindingName = "gardener.cloud:system:controller-manager"
)

func (g *gardenerControllerManager) clusterRole() *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:   clusterRoleName,
			Labels: GetLabels(),
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{operationsv1alpha1.GroupName},
				Resources: []string{
					"bastions",
				},
				Verbs: []string{"get", "list", "watch", "create", "patch", "update", "delete", "deletecollection"},
			},
			{
				APIGroups: []string{seedmanagementv1alpha1.GroupName},
				Resources: []string{
					"managedseeds",
					"managedseedsets",
					"managedseedsets/status",
				},
				Verbs: []string{"get", "list", "watch", "patch", "update"},
			},
			{
				APIGroups: []string{coordinationv1.GroupName},
				Resources: []string{"leases"},
				Verbs:     []string{"get", "list", "create", "watch", "patch", "update"},
			},
			{
				APIGroups: []string{securityv1alpha1.GroupName},
				Resources: []string{
					"credentialsbindings",
					"workloadidentities",
				},
				Verbs: []string{"create", "delete", "deletecollection", "get", "list", "watch", "patch", "update"},
			},
			{
				APIGroups: []string{certificatesv1.GroupName},
				Resources: []string{
					"certificatesigningrequests",
					"certificatesigningrequests/approval",
					"certificatesigningrequests/seedclient",
				},
				Verbs: []string{"get", "list", "watch", "create", "patch", "update", "approve", "deny"},
			},
			{
				APIGroups: []string{certificatesv1.GroupName},
				Resources: []string{
					"signers",
				},
				ResourceNames: []string{
					"kubernetes.io/kube-apiserver-client",
				},
				Verbs: []string{"patch", "update", "approve", "deny"},
			},
			{
				APIGroups: []string{authorizationv1.GroupName},
				Resources: []string{
					"subjectaccessreviews",
				},
				Verbs: []string{"create"},
			},
			{
				APIGroups: []string{settingsv1alpha1.GroupName},
				Resources: []string{
					"openidconnectpresets",
				},
				Verbs: []string{"get", "list", "create", "watch", "patch", "update", "delete", "deletecollection"},
			},
			{
				APIGroups: []string{gardencorev1beta1.GroupName},
				Resources: []string{
					"backupbuckets",
					"backupentries",
					"controllerregistrations",
					"controllerdeployments",
					"controllerinstallations",
					"exposureclasses",
					"secretbindings",
					"seeds",
					"seeds/status",
					"shoots",
					"shoots/status",
					"shoots/viewerkubeconfig",
					"quotas",
					"cloudprofiles",
					"namespacedcloudprofiles",
				},
				Verbs: []string{"get", "list", "create", "watch", "patch", "update", "delete", "deletecollection"},
			},
			{
				APIGroups: []string{gardencorev1beta1.GroupName},
				Resources: []string{
					"projects",
					"projects/status",
				},
				Verbs: []string{"get", "list", "watch", "patch", "update", "manage-members", "delete"},
			},
			{
				APIGroups: []string{rbacv1.GroupName},
				Resources: []string{
					"clusterroles",
					"clusterrolebindings",
					"roles",
					"rolebindings",
				},
				Verbs: []string{"get", "list", "watch", "create", "patch", "update", "delete", "deletecollection"},
			},
			{
				APIGroups: []string{corev1.GroupName},
				Resources: []string{
					"resourcequotas",
					"namespaces",
					"events",
					"serviceaccounts",
					"secrets",
					"configmaps",
				},
				Verbs: []string{"get", "list", "watch", "create", "patch", "update", "delete", "deletecollection"},
			},
			{
				APIGroups: []string{corev1.GroupName},
				Resources: []string{
					"serviceaccounts/token",
				},
				Verbs: []string{"create"},
			},
			{
				APIGroups: []string{eventsv1.GroupName},
				Resources: []string{
					"events",
				},
				Verbs: []string{"get", "list", "watch"},
			},
		},
	}
}

func (g *gardenerControllerManager) clusterRoleBinding(serviceAccountName string) *rbacv1.ClusterRoleBinding {
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
