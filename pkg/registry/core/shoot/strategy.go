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
	"fmt"

	"github.com/gardener/gardener/pkg/api"
	"github.com/gardener/gardener/pkg/apis/core"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/apis/core/validation"
	"github.com/gardener/gardener/pkg/operation/common"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/apiserver/pkg/storage"
	"k8s.io/apiserver/pkg/storage/names"
)

type shootStrategy struct {
	runtime.ObjectTyper
	names.NameGenerator
}

// Strategy defines the storage strategy for Shoots.
var Strategy = shootStrategy{api.Scheme, names.SimpleNameGenerator}

// Strategy should implement rest.RESTCreateUpdateStrategy
var _ rest.RESTCreateUpdateStrategy = Strategy

func (shootStrategy) NamespaceScoped() bool {
	return true
}

func (shootStrategy) PrepareForCreate(ctx context.Context, obj runtime.Object) {
	shoot := obj.(*core.Shoot)

	shoot.Generation = 1
	shoot.Status = core.ShootStatus{}
}

func (shootStrategy) PrepareForUpdate(ctx context.Context, obj, old runtime.Object) {
	newShoot := obj.(*core.Shoot)
	oldShoot := old.(*core.Shoot)
	newShoot.Status = oldShoot.Status

	if mustIncreaseGeneration(oldShoot, newShoot) {
		newShoot.Generation = oldShoot.Generation + 1
	}

	// We have introduced minimum values for the pod pids limits and have to ensure that
	// the value is taken.
	// TODO: This can be removed in a future release.
	updateKubeletConfig(newShoot.Spec.Kubernetes.Kubelet)
	for i, worker := range newShoot.Spec.Provider.Workers {
		if worker.Kubernetes != nil {
			updateKubeletConfig(newShoot.Spec.Provider.Workers[i].Kubernetes.Kubelet)
		}
	}
	updateKubeletConfig(oldShoot.Spec.Kubernetes.Kubelet)
	for i, worker := range oldShoot.Spec.Provider.Workers {
		if worker.Kubernetes != nil {
			updateKubeletConfig(oldShoot.Spec.Provider.Workers[i].Kubernetes.Kubelet)
		}
	}
}

func updateKubeletConfig(kubeletConfig *core.KubeletConfig) {
	if kubeletConfig == nil {
		return
	}

	if kubeletConfig.PodPIDsLimit != nil && *kubeletConfig.PodPIDsLimit < validation.PodPIDsLimitMinimum {
		v := validation.PodPIDsLimitMinimum
		kubeletConfig.PodPIDsLimit = &v
	}
}

func mustIncreaseGeneration(oldShoot, newShoot *core.Shoot) bool {
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
		case core.LastOperationStateFailed:
			// The shoot state is failed and the retry annotation is set.
			if val, ok := common.GetShootOperationAnnotation(newShoot.Annotations); ok && val == common.ShootOperationRetry {
				mustIncrease = true
			}
		default:
			// The shoot state is not failed and the reconcile or rotate-credentials annotation is set.
			if val, ok := common.GetShootOperationAnnotation(newShoot.Annotations); ok {
				if val == common.ShootOperationReconcile {
					mustIncrease = true
				}
				if val == common.ShootOperationRotateKubeconfigCredentials {
					// We don't want to remove the annotation so that the controller-manager can pick it up and rotate
					// the credentials. It has to remove the annotation after it is done.
					return true
				}
			}
		}

		if mustIncrease {
			delete(newShoot.Annotations, v1beta1constants.GardenerOperation)
			delete(newShoot.Annotations, common.ShootOperationDeprecated)
			return true
		}
	}

	// TODO: Just a temporary solution. Remove this in a future version once Kyma is moved out again.
	if oldShoot.ObjectMeta.Annotations[common.ShootExperimentalAddonKyma] != newShoot.ObjectMeta.Annotations[common.ShootExperimentalAddonKyma] {
		return true
	}

	return false
}

func (shootStrategy) Validate(ctx context.Context, obj runtime.Object) field.ErrorList {
	shoot := obj.(*core.Shoot)
	return validation.ValidateShoot(shoot)
}

func (shootStrategy) Canonicalize(obj runtime.Object) {
}

func (shootStrategy) AllowCreateOnUpdate() bool {
	return false
}

func (shootStrategy) ValidateUpdate(ctx context.Context, newObj, oldObj runtime.Object) field.ErrorList {
	newShoot := newObj.(*core.Shoot)
	oldShoot := oldObj.(*core.Shoot)
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
	newShoot := obj.(*core.Shoot)
	oldShoot := old.(*core.Shoot)
	newShoot.Spec = oldShoot.Spec
}

func (shootStatusStrategy) ValidateUpdate(ctx context.Context, obj, old runtime.Object) field.ErrorList {
	return validation.ValidateShootStatusUpdate(obj.(*core.Shoot).Status, old.(*core.Shoot).Status)
}

// ToSelectableFields returns a field set that represents the object
// TODO: fields are not labels, and the validation rules for them do not apply.
func ToSelectableFields(shoot *core.Shoot) fields.Set {
	// The purpose of allocation with a given number of elements is to reduce
	// amount of allocations needed to create the fields.Set. If you add any
	// field here or the number of object-meta related fields changes, this should
	// be adjusted.
	shootSpecificFieldsSet := make(fields.Set, 4)
	shootSpecificFieldsSet[core.ShootSeedName] = getSeedName(shoot)
	shootSpecificFieldsSet[core.ShootCloudProfileName] = shoot.Spec.CloudProfileName
	return generic.AddObjectMetaFieldsSet(shootSpecificFieldsSet, &shoot.ObjectMeta, true)
}

// GetAttrs returns labels and fields of a given object for filtering purposes.
func GetAttrs(obj runtime.Object) (labels.Set, fields.Set, error) {
	shoot, ok := obj.(*core.Shoot)
	if !ok {
		return nil, nil, fmt.Errorf("not a shoot")
	}
	return labels.Set(shoot.ObjectMeta.Labels), ToSelectableFields(shoot), nil
}

// MatchShoot returns a generic matcher for a given label and field selector.
func MatchShoot(label labels.Selector, field fields.Selector) storage.SelectionPredicate {
	return storage.SelectionPredicate{
		Label:       label,
		Field:       field,
		GetAttrs:    GetAttrs,
		IndexFields: []string{core.ShootSeedName},
	}
}

// SeedNameTriggerFunc returns spec.seedName of given Shoot.
func SeedNameTriggerFunc(obj runtime.Object) string {
	shoot, ok := obj.(*core.Shoot)
	if !ok {
		return ""
	}

	return getSeedName(shoot)
}

func getSeedName(shoot *core.Shoot) string {
	if shoot.Spec.SeedName == nil {
		return ""
	}
	return *shoot.Spec.SeedName
}
