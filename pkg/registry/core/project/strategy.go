// Copyright 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"fmt"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/storage"
	"k8s.io/apiserver/pkg/storage/names"

	"github.com/gardener/gardener/pkg/api"
	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/apis/core/validation"
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
}

func (projectStrategy) PrepareForUpdate(_ context.Context, obj, old runtime.Object) {
	newProject := obj.(*core.Project)
	oldProject := old.(*core.Project)
	newProject.Status = oldProject.Status

	if !apiequality.Semantic.DeepEqual(oldProject.Spec, newProject.Spec) {
		newProject.Generation = oldProject.Generation + 1
	}
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

// WarningsOnCreate returns warnings to the client performing a create.
func (projectStrategy) WarningsOnCreate(ctx context.Context, obj runtime.Object) []string {
	return nil
}

// WarningsOnUpdate returns warnings to the client performing the update.
func (projectStrategy) WarningsOnUpdate(ctx context.Context, obj, old runtime.Object) []string {
	return nil
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

// ToSelectableFields returns a field set that represents the object
// TODO: fields are not labels, and the validation rules for them do not apply.
func ToSelectableFields(project *core.Project) fields.Set {
	// The purpose of allocation with a given number of elements is to reduce
	// amount of allocations needed to create the fields.Set. If you add any
	// field here or the number of object-meta related fields changes, this should
	// be adjusted.
	projectSpecificFieldsSet := make(fields.Set, 2)
	projectSpecificFieldsSet[core.ProjectNamespace] = getNamespace(project)
	return generic.AddObjectMetaFieldsSet(projectSpecificFieldsSet, &project.ObjectMeta, false)
}

// GetAttrs returns labels and fields of a given object for filtering purposes.
func GetAttrs(obj runtime.Object) (labels.Set, fields.Set, error) {
	project, ok := obj.(*core.Project)
	if !ok {
		return nil, nil, fmt.Errorf("not a project")
	}
	return project.ObjectMeta.Labels, ToSelectableFields(project), nil
}

// MatchProject returns a generic matcher for a given label and field selector.
func MatchProject(label labels.Selector, field fields.Selector) storage.SelectionPredicate {
	return storage.SelectionPredicate{
		Label:       label,
		Field:       field,
		GetAttrs:    GetAttrs,
		IndexFields: []string{core.ProjectNamespace},
	}
}

// NamespaceTriggerFunc returns spec.namespace of given Project.
func NamespaceTriggerFunc(obj runtime.Object) string {
	project, ok := obj.(*core.Project)
	if !ok {
		return ""
	}
	return getNamespace(project)
}

func getNamespace(project *core.Project) string {
	if project.Spec.Namespace == nil {
		return ""
	}
	return *project.Spec.Namespace
}
