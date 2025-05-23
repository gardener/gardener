// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package prometheus

import (
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
)

func (p *prometheus) clusterRoleBinding() *rbacv1.ClusterRoleBinding {
	obj := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:        p.name(),
			Labels:      p.getLabels(),
			Annotations: map[string]string{resourcesv1alpha1.DeleteOnInvalidUpdate: "true"},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     "prometheus",
		},
		Subjects: []rbacv1.Subject{{
			Kind:      rbacv1.ServiceAccountKind,
			Name:      p.name(),
			Namespace: p.namespace,
		}},
	}

	if p.values.NamespaceUID != nil {
		obj.Name += "-" + string(*p.values.NamespaceUID)
		obj.OwnerReferences = append(obj.OwnerReferences, metav1.OwnerReference{
			APIVersion:         corev1.SchemeGroupVersion.String(),
			Kind:               "Namespace",
			Name:               p.namespace,
			UID:                *p.values.NamespaceUID,
			Controller:         ptr.To(true),
			BlockOwnerDeletion: ptr.To(true),
		})
	}

	return obj
}

func (p *prometheus) clusterRoleTarget() *rbacv1.ClusterRole {
	obj := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "gardener.cloud:monitoring:" + p.name(),
		},
		Rules: []rbacv1.PolicyRule{{
			NonResourceURLs: []string{"/metrics"},
			Verbs:           []string{"get"},
		}},
	}

	if p.values.TargetCluster.ScrapesMetrics {
		obj.Rules = append(obj.Rules,
			rbacv1.PolicyRule{
				APIGroups: []string{corev1.GroupName},
				Resources: []string{"nodes", "services", "endpoints", "pods"},
				Verbs:     []string{"get", "list", "watch"},
			},
			rbacv1.PolicyRule{
				APIGroups: []string{corev1.GroupName},
				Resources: []string{"nodes/metrics", "pods/log", "nodes/proxy", "services/proxy", "pods/proxy"},
				Verbs:     []string{"get"},
			},
		)
	}

	return obj
}

func (p *prometheus) clusterRoleBindingTarget() *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "gardener.cloud:monitoring:" + p.name(),
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     "gardener.cloud:monitoring:" + p.name(),
		},
		Subjects: []rbacv1.Subject{{
			Kind:      "ServiceAccount",
			Name:      p.values.TargetCluster.ServiceAccountName,
			Namespace: metav1.NamespaceSystem,
		}},
	}
}
