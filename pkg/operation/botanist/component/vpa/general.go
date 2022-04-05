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
	rbacv1 "k8s.io/api/rbac/v1"
)

func (v *vpa) generalResourceConfigs() resourceConfigs {
	var (
		clusterRoleActor = v.emptyClusterRole("actor")
	)

	return resourceConfigs{
		{obj: clusterRoleActor, class: application, mutateFn: func() { v.reconcileGeneralClusterRoleActor(clusterRoleActor) }},
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
