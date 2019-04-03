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

package plant

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

type plantStrategy struct {
	runtime.ObjectTyper
	names.NameGenerator
}

// Strategy defines the storage strategy for plants.
var Strategy = plantStrategy{api.Scheme, names.SimpleNameGenerator}

func (plantStrategy) NamespaceScoped() bool {
	return true
}

func (plantStrategy) PrepareForCreate(ctx context.Context, obj runtime.Object) {
	plant := obj.(*core.Plant)

	plant.Generation = 1
	plant.Status = core.PlantStatus{}
}

func (plantStrategy) PrepareForUpdate(ctx context.Context, obj, old runtime.Object) {
	newplant := obj.(*core.Plant)
	oldplant := old.(*core.Plant)
	newplant.Status = oldplant.Status

	if mustIncreaseGeneration(oldplant, newplant) {
		newplant.Generation = oldplant.Generation + 1
	}
}

func mustIncreaseGeneration(oldplant, newplant *core.Plant) bool {
	// The plant specification changes.
	if !apiequality.Semantic.DeepEqual(oldplant.Spec, newplant.Spec) {
		return true
	}

	// The deletion timestamp was set.
	if oldplant.DeletionTimestamp == nil && newplant.DeletionTimestamp != nil {
		return true
	}

	return false
}

func (plantStrategy) Validate(ctx context.Context, obj runtime.Object) field.ErrorList {
	plant := obj.(*core.Plant)
	return validation.ValidatePlant(plant)
}

func (plantStrategy) Canonicalize(obj runtime.Object) {
}

func (plantStrategy) AllowCreateOnUpdate() bool {
	return false
}

func (plantStrategy) ValidateUpdate(ctx context.Context, newObj, oldObj runtime.Object) field.ErrorList {
	newplant := newObj.(*core.Plant)
	oldplant := oldObj.(*core.Plant)
	return validation.ValidatePlantUpdate(newplant, oldplant)
}

func (plantStrategy) AllowUnconditionalUpdate() bool {
	return false
}

type plantStatusStrategy struct {
	plantStrategy
}

// StatusStrategy defines the storage strategy for the status subresource of plants.
var StatusStrategy = plantStatusStrategy{Strategy}

func (plantStatusStrategy) PrepareForUpdate(ctx context.Context, obj, old runtime.Object) {
	newplant := obj.(*core.Plant)
	oldplant := old.(*core.Plant)
	newplant.Spec = oldplant.Spec
}

func (plantStatusStrategy) ValidateUpdate(ctx context.Context, obj, old runtime.Object) field.ErrorList {
	return validation.ValidatePlantStatusUpdate(obj.(*core.Plant).Status, old.(*core.Plant).Status)
}
