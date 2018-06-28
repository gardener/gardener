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

package backupinfrastructure

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
)

type backupInfrastructureStrategy struct {
	runtime.ObjectTyper
	names.NameGenerator
}

// Strategy defines the storage strategy for BackupInfrastructures.
var Strategy = backupInfrastructureStrategy{api.Scheme, names.SimpleNameGenerator}

func (backupInfrastructureStrategy) NamespaceScoped() bool {
	return true
}

func (backupInfrastructureStrategy) PrepareForCreate(ctx context.Context, obj runtime.Object) {
	backupInfrastructure := obj.(*garden.BackupInfrastructure)

	backupInfrastructure.Generation = 1
	backupInfrastructure.Status = garden.BackupInfrastructureStatus{}

	finalizers := sets.NewString(backupInfrastructure.Finalizers...)
	if !finalizers.Has(gardenv1beta1.GardenerName) {
		finalizers.Insert(gardenv1beta1.GardenerName)
	}
	backupInfrastructure.Finalizers = finalizers.UnsortedList()
}

func (backupInfrastructureStrategy) PrepareForUpdate(ctx context.Context, obj, old runtime.Object) {
	newBackupInfrastructure := obj.(*garden.BackupInfrastructure)
	oldBackupInfrastructure := old.(*garden.BackupInfrastructure)
	newBackupInfrastructure.Status = oldBackupInfrastructure.Status

	if mustIncreaseGeneration(oldBackupInfrastructure, newBackupInfrastructure) {
		newBackupInfrastructure.Generation = oldBackupInfrastructure.Generation + 1
	}
}

func mustIncreaseGeneration(oldBackupInfrastructure, newBackupInfrastructure *garden.BackupInfrastructure) bool {
	// The BackupInfrastructure specification changes.
	if !apiequality.Semantic.DeepEqual(oldBackupInfrastructure.Spec, newBackupInfrastructure.Spec) {
		return true
	}

	// The deletion timestamp was set.
	if oldBackupInfrastructure.DeletionTimestamp == nil && newBackupInfrastructure.DeletionTimestamp != nil {
		return true
	}

	return false
}

func (backupInfrastructureStrategy) Validate(ctx context.Context, obj runtime.Object) field.ErrorList {
	backupInfrastructure := obj.(*garden.BackupInfrastructure)
	return validation.ValidateBackupInfrastructure(backupInfrastructure)
}

func (backupInfrastructureStrategy) Canonicalize(obj runtime.Object) {
}

func (backupInfrastructureStrategy) AllowCreateOnUpdate() bool {
	return false
}

func (backupInfrastructureStrategy) ValidateUpdate(ctx context.Context, newObj, oldObj runtime.Object) field.ErrorList {
	oldBackupInfrastructure, newBackupInfrastructure := oldObj.(*garden.BackupInfrastructure), newObj.(*garden.BackupInfrastructure)
	return validation.ValidateBackupInfrastructureUpdate(newBackupInfrastructure, oldBackupInfrastructure)
}

func (backupInfrastructureStrategy) AllowUnconditionalUpdate() bool {
	return false
}

type backupInfrastructureStatusStrategy struct {
	backupInfrastructureStrategy
}

// StatusStrategy defines the storage strategy for the status subresource of BackupInfrastructures.
var StatusStrategy = backupInfrastructureStatusStrategy{Strategy}

func (backupInfrastructureStatusStrategy) PrepareForUpdate(ctx context.Context, obj, old runtime.Object) {
	newBackupInfrastructure := obj.(*garden.BackupInfrastructure)
	oldBackupInfrastructure := old.(*garden.BackupInfrastructure)
	newBackupInfrastructure.Spec = oldBackupInfrastructure.Spec
}

func (backupInfrastructureStatusStrategy) ValidateUpdate(ctx context.Context, obj, old runtime.Object) field.ErrorList {
	return validation.ValidateBackupInfrastructureStatusUpdate(obj.(*garden.BackupInfrastructure), old.(*garden.BackupInfrastructure))
}
