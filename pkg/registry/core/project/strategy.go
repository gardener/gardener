// SPDX-FileCopyrightText: 2018 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package project

import (
	"context"

	"github.com/gardener/gardener/pkg/api"
	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/apis/core/validation"

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

func (projectStrategy) PrepareForCreate(ctx context.Context, obj runtime.Object) {
	project := obj.(*core.Project)

	project.Generation = 1
	project.Status = core.ProjectStatus{}
}

func (projectStrategy) PrepareForUpdate(ctx context.Context, obj, old runtime.Object) {
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
