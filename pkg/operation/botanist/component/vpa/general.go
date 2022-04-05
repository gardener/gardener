// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package vpa

import (
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"

	rbacv1 "k8s.io/api/rbac/v1"
)

func (v *vpa) generalResourceConfigs() resourceConfigs {
	var (
		clusterRoleActor               = v.emptyClusterRole("actor")
		clusterRoleBindingActor        = v.emptyClusterRoleBinding("actor")
		clusterRoleTargetReader        = v.emptyClusterRole("target-reader")
		clusterRoleBindingTargetReader = v.emptyClusterRoleBinding("target-reader")
	)

	return resourceConfigs{
		{obj: clusterRoleActor, class: application, mutateFn: func() { v.reconcileGeneralClusterRoleActor(clusterRoleActor) }},
		{obj: clusterRoleBindingActor, class: application, mutateFn: func() { v.reconcileGeneralClusterRoleBindingActor(clusterRoleBindingActor, clusterRoleActor) }},
		{obj: clusterRoleTargetReader, class: application, mutateFn: func() { v.reconcileGeneralClusterRoleTargetReader(clusterRoleTargetReader) }},
		{obj: clusterRoleBindingTargetReader, class: application, mutateFn: func() {
			v.reconcileGeneralClusterRoleBindingTargetReader(clusterRoleBindingTargetReader, clusterRoleTargetReader)
		}},
	}
}

func (v *vpa) reconcileGeneralClusterRoleActor(clusterRole *rbacv1.ClusterRole) {
	clusterRole.Labels = getRoleLabel()
	clusterRole.Rules = []rbacv1.PolicyRule{
		{
			APIGroups: []string{""},
			Resources: []string{"pods", "nodes", "limitranges"},
			Verbs:     []string{"get", "list", "watch"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"events"},
			Verbs:     []string{"get", "list", "watch", "create"},
		},
		{
			APIGroups: []string{"poc.autoscaling.k8s.io"},
			Resources: []string{"verticalpodautoscalers"},
			Verbs:     []string{"get", "list", "watch", "patch"},
		},
		{
			APIGroups: []string{"autoscaling.k8s.io"},
			Resources: []string{"verticalpodautoscalers"},
			Verbs:     []string{"get", "list", "watch", "patch"},
		},
		{
			APIGroups: []string{"coordination.k8s.io"},
			Resources: []string{"leases"},
			Verbs:     []string{"get", "list", "watch"},
		},
	}
}

func (v *vpa) reconcileGeneralClusterRoleBindingActor(clusterRoleBinding *rbacv1.ClusterRoleBinding, clusterRole *rbacv1.ClusterRole) {
	clusterRoleBinding.Labels = getRoleLabel()
	clusterRoleBinding.Annotations = map[string]string{resourcesv1alpha1.DeleteOnInvalidUpdate: "true"}
	clusterRoleBinding.RoleRef = rbacv1.RoleRef{
		APIGroup: rbacv1.GroupName,
		Kind:     "ClusterRole",
		Name:     clusterRole.Name,
	}
	clusterRoleBinding.Subjects = []rbacv1.Subject{
		{
			Kind:      rbacv1.ServiceAccountKind,
			Name:      recommender,
			Namespace: v.serviceAccountNamespace(),
		},
		{
			Kind:      rbacv1.ServiceAccountKind,
			Name:      updater,
			Namespace: v.serviceAccountNamespace(),
		},
	}
}

func (v *vpa) reconcileGeneralClusterRoleTargetReader(clusterRole *rbacv1.ClusterRole) {
	clusterRole.Labels = getRoleLabel()
	clusterRole.Rules = []rbacv1.PolicyRule{
		{
			APIGroups: []string{"*"},
			Resources: []string{"*/scale"},
			Verbs:     []string{"get", "watch"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"replicationcontrollers"},
			Verbs:     []string{"get", "list", "watch"},
		},
		{
			APIGroups: []string{"apps"},
			Resources: []string{"daemonsets", "deployments", "replicasets", "statefulsets"},
			Verbs:     []string{"get", "list", "watch"},
		},
		{
			APIGroups: []string{"batch"},
			Resources: []string{"jobs", "cronjobs"},
			Verbs:     []string{"get", "list", "watch"},
		},
		{
			APIGroups: []string{"druid.gardener.cloud"},
			Resources: []string{"etcds", "etcds/scale"},
			Verbs:     []string{"get", "list", "watch"},
		},
	}
}

func (v *vpa) reconcileGeneralClusterRoleBindingTargetReader(clusterRoleBinding *rbacv1.ClusterRoleBinding, clusterRole *rbacv1.ClusterRole) {
	clusterRoleBinding.Labels = getRoleLabel()
	clusterRoleBinding.Annotations = map[string]string{resourcesv1alpha1.DeleteOnInvalidUpdate: "true"}
	clusterRoleBinding.RoleRef = rbacv1.RoleRef{
		APIGroup: rbacv1.GroupName,
		Kind:     "ClusterRole",
		Name:     clusterRole.Name,
	}
	clusterRoleBinding.Subjects = []rbacv1.Subject{
		{
			Kind:      rbacv1.ServiceAccountKind,
			Name:      admissionController,
			Namespace: v.serviceAccountNamespace(),
		},
		{
			Kind:      rbacv1.ServiceAccountKind,
			Name:      recommender,
			Namespace: v.serviceAccountNamespace(),
		},
		{
			Kind:      rbacv1.ServiceAccountKind,
			Name:      updater,
			Namespace: v.serviceAccountNamespace(),
		},
	}
}
