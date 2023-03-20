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

package v1beta1

import (
	rbacv1 "k8s.io/api/rbac/v1"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
)

// SetDefaults_Project sets default values for Project objects.
func SetDefaults_Project(obj *Project) {
	if obj.Spec.Owner != nil {
		setDefaults_Subject(obj.Spec.Owner)
	}

	if obj.Spec.Namespace != nil && *obj.Spec.Namespace == v1beta1constants.GardenNamespace {
		if obj.Spec.Tolerations == nil {
			obj.Spec.Tolerations = &ProjectTolerations{}
		}
		addTolerations(&obj.Spec.Tolerations.Whitelist, Toleration{Key: SeedTaintProtected})
		addTolerations(&obj.Spec.Tolerations.Defaults, Toleration{Key: SeedTaintProtected})
	}
}

// setDefaults_Subject sets default values for rbacv1.Subject objects.
// It is not exported and called manually, because we cannot create defaulting functions for foreign API groups.
func setDefaults_Subject(obj *rbacv1.Subject) {
	if len(obj.APIGroup) == 0 {
		switch obj.Kind {
		case rbacv1.ServiceAccountKind:
			obj.APIGroup = ""
		case rbacv1.UserKind:
			obj.APIGroup = rbacv1.GroupName
		case rbacv1.GroupKind:
			obj.APIGroup = rbacv1.GroupName
		}
	}
}

// SetDefaults_ProjectMember sets default values for ProjectMember objects.
func SetDefaults_ProjectMember(obj *ProjectMember) {
	setDefaults_Subject(&obj.Subject)

	if len(obj.Role) == 0 && len(obj.Roles) == 0 {
		obj.Role = ProjectMemberViewer
	}
}
