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

package dashboard

import (
	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiregistrationv1beta1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1beta1"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
)

const (
	clusterRoleName        = "gardener.cloud:system:dashboard"
	clusterRoleBindingName = "gardener.cloud:system:dashboard"
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
			// TODO: Remove once https://github.com/gardener/monitoring/issues/11 is resolved.
			{
				APIGroups: []string{corev1.GroupName},
				Resources: []string{"secrets"},
				Verbs:     []string{"get"},
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
