// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
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
