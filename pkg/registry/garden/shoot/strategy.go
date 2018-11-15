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

package shoot

import (
	"context"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/storage/names"

	"github.com/gardener/gardener/pkg/api"
	"github.com/gardener/gardener/pkg/apis/garden"
	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/gardener/gardener/pkg/apis/garden/validation"
	"github.com/gardener/gardener/pkg/operation/common"
)

type shootStrategy struct {
	runtime.ObjectTyper
	names.NameGenerator
}

// Strategy defines the storage strategy for Shoots.
var Strategy = shootStrategy{api.Scheme, names.SimpleNameGenerator}

func (shootStrategy) NamespaceScoped() bool {
	return true
}

func (shootStrategy) PrepareForCreate(ctx context.Context, obj runtime.Object) {
	shoot := obj.(*garden.Shoot)

	shoot.Generation = 1
	shoot.Status = garden.ShootStatus{}

	finalizers := sets.NewString(shoot.Finalizers...)
	if !finalizers.Has(gardenv1beta1.GardenerName) {
		finalizers.Insert(gardenv1beta1.GardenerName)
	}
	shoot.Finalizers = finalizers.UnsortedList()
}

func (shootStrategy) PrepareForUpdate(ctx context.Context, obj, old runtime.Object) {
	newShoot := obj.(*garden.Shoot)
	oldShoot := old.(*garden.Shoot)
	newShoot.Status = oldShoot.Status

	if mustIncreaseGeneration(oldShoot, newShoot) {
		newShoot.Generation = oldShoot.Generation + 1
	}
}

func mustIncreaseGeneration(oldShoot, newShoot *garden.Shoot) bool {
	// The Shoot specification changes.
	if !apiequality.Semantic.DeepEqual(oldShoot.Spec, newShoot.Spec) {
		return true
	}

	// The deletion timestamp is set.
	if oldShoot.DeletionTimestamp == nil && newShoot.DeletionTimestamp != nil {
		return true
	}

	if lastOperation := newShoot.Status.LastOperation; lastOperation != nil {
		mustIncrease := false

		switch lastOperation.State {
		case garden.ShootLastOperationStateFailed:
			// The shoot state is failed and the retry annotation is set.
			if val, ok := newShoot.Annotations[common.ShootOperation]; ok && val == common.ShootOperationRetry {
				mustIncrease = true
			}
		default:
			// The shoot state is not failed and the reconcile annotation is set.
			if val, ok := newShoot.Annotations[common.ShootOperation]; ok && val == common.ShootOperationReconcile {
				mustIncrease = true
			}
		}

		if mustIncrease {
			delete(newShoot.Annotations, common.ShootOperation)
			return true
		}
	}

	return false
}

func (shootStrategy) Validate(ctx context.Context, obj runtime.Object) field.ErrorList {
	shoot := obj.(*garden.Shoot)
	return validation.ValidateShoot(shoot)
}

func (shootStrategy) Canonicalize(obj runtime.Object) {
}

func (shootStrategy) AllowCreateOnUpdate() bool {
	return false
}

func (shootStrategy) ValidateUpdate(ctx context.Context, newObj, oldObj runtime.Object) field.ErrorList {
	newShoot := newObj.(*garden.Shoot)
	oldShoot := oldObj.(*garden.Shoot)
	return validation.ValidateShootUpdate(newShoot, oldShoot)
}

func (shootStrategy) AllowUnconditionalUpdate() bool {
	return false
}

type shootStatusStrategy struct {
	shootStrategy
}

// StatusStrategy defines the storage strategy for the status subresource of Shoots.
var StatusStrategy = shootStatusStrategy{Strategy}

func (shootStatusStrategy) PrepareForUpdate(ctx context.Context, obj, old runtime.Object) {
	newShoot := obj.(*garden.Shoot)
	oldShoot := old.(*garden.Shoot)
	newShoot.Spec = oldShoot.Spec
}

func (shootStatusStrategy) ValidateUpdate(ctx context.Context, obj, old runtime.Object) field.ErrorList {
	return validation.ValidateShootStatusUpdate(obj.(*garden.Shoot).Status, old.(*garden.Shoot).Status)
}
