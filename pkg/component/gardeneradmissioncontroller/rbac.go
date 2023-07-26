// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package gardeneradmissioncontroller

import (
	certificatesv1 "k8s.io/api/certificates/v1"
	coordinationv1beta1 "k8s.io/api/coordination/v1beta1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	operationsv1alpha1 "github.com/gardener/gardener/pkg/apis/operations/v1alpha1"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
)

const (
	clusterRoleName        = "gardener.cloud:system:admission-controller"
	clusterRoleBindingName = "gardener.cloud:admission-controller"
)

func (a *gardenerAdmissionController) clusterRole() *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:   clusterRoleName,
			Labels: GetLabels(),
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{gardencorev1beta1.GroupName},
				Resources: []string{
					"backupbuckets",
					"backupentries",
					"controllerinstallations",
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
			{
				APIGroups: []string{operationsv1alpha1.GroupName},
				Resources: []string{
					"bastions",
				},
				Verbs: []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{corev1.GroupName},
				Resources: []string{
					"configmaps",
				},
				Verbs: []string{"get"},
			},
			{
				APIGroups: []string{corev1.GroupName},
				Resources: []string{
					"namespaces",
					"secrets",
					"serviceaccounts",
				},
				Verbs: []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{coordinationv1beta1.GroupName},
				Resources: []string{
					"leases",
				},
				Verbs: []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{certificatesv1.GroupName},
				Resources: []string{
					"certificatesigningrequests",
				},
				Verbs: []string{"get", "list", "watch"},
			},
		},
	}
}

func (a *gardenerAdmissionController) clusterRoleBinding(serviceAccountName string) *rbacv1.ClusterRoleBinding {
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
