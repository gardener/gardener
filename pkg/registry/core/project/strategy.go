// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package project

import (
	"context"

	"github.com/gardener/gardener/pkg/api"
	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/apis/core/validation"

	rbacv1 "k8s.io/api/rbac/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/storage/names"
)

type projectStrategy struct {
	runtime.ObjectTyper
	names.NameGenerator
}

// Strategy defines the storage strategy for Projects.
var Strategy = projectStrategy{api.Scheme, names.SimpleNameGenerator}

func (projectStrategy) NamespaceScoped() bool {
	return false
}

func (projectStrategy) PrepareForCreate(_ context.Context, obj runtime.Object) {
	project := obj.(*core.Project)

	project.Generation = 1
	project.Status = core.ProjectStatus{}

	mergeDuplicateMembers(project)
}

func (projectStrategy) PrepareForUpdate(_ context.Context, obj, old runtime.Object) {
	newProject := obj.(*core.Project)
	oldProject := old.(*core.Project)
	newProject.Status = oldProject.Status

	if !apiequality.Semantic.DeepEqual(oldProject.Spec, newProject.Spec) {
		newProject.Generation = oldProject.Generation + 1
	}

	mergeDuplicateMembers(newProject)
}

// TODO: This code is needed now that we have introduced validation that forbids specifying duplicates in the
// spec.members list (this code wasn't there before). Hence, we have to remove the duplicates now to not break the API
// incompatibly.
// This code can be removed in a future version.
func mergeDuplicateMembers(project *core.Project) {
	var (
		oldMembersToRoles = make(map[string]map[string]struct{})
		memberToNewRoles  = make(map[string][]string)
		newMembers        []core.ProjectMember
	)

	for _, member := range project.Spec.Members {
		apiGroup, kind, namespace, name, err := validation.ProjectMemberProperties(member)
		if err != nil {
			// No meaningful way to handle the error here
			continue
		}
		id := validation.ProjectMemberId(apiGroup, kind, namespace, name)

		if _, ok := oldMembersToRoles[id]; !ok {
			newMembers = append(newMembers, core.ProjectMember{
				Subject: rbacv1.Subject{
					APIGroup:  apiGroup,
					Kind:      kind,
					Namespace: namespace,
					Name:      name,
				},
			})
			oldMembersToRoles[id] = make(map[string]struct{})
		}

		for _, role := range member.Roles {
			if _, ok := oldMembersToRoles[id][role]; !ok {
				memberToNewRoles[id] = append(memberToNewRoles[id], role)
			}
			oldMembersToRoles[id][role] = struct{}{}
		}
	}

	for i, member := range newMembers {
		apiGroup, kind, namespace, name, err := validation.ProjectMemberProperties(member)
		if err != nil {
			// No meaningful way to handle the error here
			continue
		}
		id := validation.ProjectMemberId(apiGroup, kind, namespace, name)

		newMembers[i].Roles = memberToNewRoles[id]
	}

	project.Spec.Members = newMembers
}

func (projectStrategy) Validate(ctx context.Context, obj runtime.Object) field.ErrorList {
	project := obj.(*core.Project)
	return validation.ValidateProject(project)
}

func (projectStrategy) Canonicalize(obj runtime.Object) {
}

func (projectStrategy) AllowCreateOnUpdate() bool {
	return false
}

func (projectStrategy) AllowUnconditionalUpdate() bool {
	return true
}

func (projectStrategy) ValidateUpdate(ctx context.Context, newObj, oldObj runtime.Object) field.ErrorList {
	oldProject, newProject := oldObj.(*core.Project), newObj.(*core.Project)
	return validation.ValidateProjectUpdate(newProject, oldProject)
}

type projectStatusStrategy struct {
	projectStrategy
}

// StatusStrategy defines the storage strategy for the status subresource of Projects.
var StatusStrategy = projectStatusStrategy{Strategy}

func (projectStatusStrategy) PrepareForUpdate(ctx context.Context, obj, old runtime.Object) {
	newProject := obj.(*core.Project)
	oldProject := old.(*core.Project)
	newProject.Spec = oldProject.Spec
}

func (projectStatusStrategy) ValidateUpdate(ctx context.Context, obj, old runtime.Object) field.ErrorList {
	return validation.ValidateProjectStatusUpdate(obj.(*core.Project), old.(*core.Project))
}
