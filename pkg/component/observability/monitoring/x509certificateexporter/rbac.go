// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0package x509certificateexporter

package x509certificateexporter

import (
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (x *x509CertificateExporter) serviceAccount(resName string) *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      resName,
			Namespace: x.namespace,
		},
	}
}

func (x *x509CertificateExporter) inClusterClusterRole(resName string, vals Values) *rbacv1.ClusterRole {
	var (
		nsses = []string{}
	)
	if len(vals.IncludeNamespaces) > 0 {
		nsses = vals.IncludeNamespaces
	}
	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: resName,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"secrets", "configmaps"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups:     []string{""},
				Resources:     []string{"namespaces"},
				Verbs:         []string{"get", "list", "watch"},
				ResourceNames: nsses,
			},
		},
	}
}

func (x *x509CertificateExporter) inClusterClusterRoleBinding(
	resName string, sa *corev1.ServiceAccount, cr *rbacv1.ClusterRole,
) *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: resName,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      sa.Name,
				Namespace: sa.Namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "ClusterRole",
			Name:     cr.Name,
			APIGroup: rbacv1.GroupName,
		},
	}
}
