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
	"github.com/gardener/gardener/pkg/apis/garden"
	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/gardener/gardener/pkg/apis/garden/validation"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
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
	project := obj.(*garden.Project)

	project.Generation = 1
	project.Status = garden.ProjectStatus{}

	finalizers := sets.NewString(project.Finalizers...)
	if !finalizers.Has(gardenv1beta1.GardenerName) {
		finalizers.Insert(gardenv1beta1.GardenerName)
	}
	project.Finalizers = finalizers.UnsortedList()
}

func (projectStrategy) PrepareForUpdate(ctx context.Context, obj, old runtime.Object) {
	newProject := obj.(*garden.Project)
	oldProject := old.(*garden.Project)
	newProject.Status = oldProject.Status

	if !apiequality.Semantic.DeepEqual(oldProject.Spec, newProject.Spec) {
		newProject.Generation = oldProject.Generation + 1
	}
}

func (projectStrategy) Validate(ctx context.Context, obj runtime.Object) field.ErrorList {
	project := obj.(*garden.Project)
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
	oldProject, newProject := oldObj.(*garden.Project), newObj.(*garden.Project)
	return validation.ValidateProjectUpdate(newProject, oldProject)
}

type projectStatusStrategy struct {
	projectStrategy
}

// StatusStrategy defines the storage strategy for the status subresource of Projects.
var StatusStrategy = projectStatusStrategy{Strategy}

func (projectStatusStrategy) PrepareForUpdate(ctx context.Context, obj, old runtime.Object) {
	newProject := obj.(*garden.Project)
	oldProject := old.(*garden.Project)
	newProject.Spec = oldProject.Spec
}

func (projectStatusStrategy) ValidateUpdate(ctx context.Context, obj, old runtime.Object) field.ErrorList {
	return validation.ValidateProjectStatusUpdate(obj.(*garden.Project), old.(*garden.Project))
}
