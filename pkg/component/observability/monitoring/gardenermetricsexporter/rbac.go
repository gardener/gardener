// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardenermetricsexporter

import (
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
)

const (
	clusterRoleName        = "gardener.cloud:metrics-exporter"
	clusterRoleBindingName = "gardener.cloud:metrics-exporter"
)

func (g *gardenerMetricsExporter) clusterRole() *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:   clusterRoleName,
			Labels: GetLabels(),
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{gardencorev1beta1.GroupName},
				Resources: []string{
					"secretbindings",
					"seeds",
					"shoots",
					"projects",
				},
				Verbs: []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{seedmanagementv1alpha1.GroupName},
				Resources: []string{
					"managedseeds",
				},
				Verbs: []string{"get", "list", "watch"},
			},
		},
	}
}

func (g *gardenerMetricsExporter) clusterRoleBinding(serviceAccountName string) *rbacv1.ClusterRoleBinding {
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
