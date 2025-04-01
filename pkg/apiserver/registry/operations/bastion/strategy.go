// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package bastion

import (
	"context"
	"fmt"
	"time"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/storage"
	"k8s.io/apiserver/pkg/storage/names"

	"github.com/gardener/gardener/pkg/api"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/apis/operations"
	operationsvalidation "github.com/gardener/gardener/pkg/apis/operations/validation"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

const (
	// TimeToLive is the duration the ExpirationTimestamp of a bastion will be extended
	// by on every heartbeat.
	TimeToLive = 1 * time.Hour
)

type bastionStrategy struct {
	runtime.ObjectTyper
	names.NameGenerator

	timeToLive time.Duration
}

// Strategy defines the storage strategy for Bastions.
var Strategy = bastionStrategy{api.Scheme, names.SimpleNameGenerator, TimeToLive}

func (bastionStrategy) NamespaceScoped() bool {
	return true
}

func (s bastionStrategy) PrepareForCreate(_ context.Context, obj runtime.Object) {
	bastion := obj.(*operations.Bastion)
	bastion.Generation = 1

	s.heartbeat(bastion)
}

func (s bastionStrategy) heartbeat(bastion *operations.Bastion) {
	now := metav1.NewTime(time.Now())
	expires := metav1.NewTime(now.Add(s.timeToLive))

	bastion.Status.LastHeartbeatTimestamp = &now
	bastion.Status.ExpirationTimestamp = &expires

	if bastion.Annotations[v1beta1constants.GardenerOperation] == v1beta1constants.GardenerOperationKeepalive {
		delete(bastion.Annotations, v1beta1constants.GardenerOperation)
	}
}

func (s bastionStrategy) PrepareForUpdate(_ context.Context, obj, old runtime.Object) {
	newBastion := obj.(*operations.Bastion)
	oldBastion := old.(*operations.Bastion)
	newBastion.Status = oldBastion.Status

	if mustIncreaseGeneration(oldBastion, newBastion) {
		newBastion.Generation = oldBastion.Generation + 1
	}

	if newBastion.Annotations[v1beta1constants.GardenerOperation] == v1beta1constants.GardenerOperationKeepalive {
		s.heartbeat(newBastion)
	}
}

func mustIncreaseGeneration(oldBastion, newBastion *operations.Bastion) bool {
	// The Bastion specification changes.
	if !apiequality.Semantic.DeepEqual(oldBastion.Spec, newBastion.Spec) {
		return true
	}

	// The deletion timestamp was set.
	if oldBastion.DeletionTimestamp == nil && newBastion.DeletionTimestamp != nil {
		return true
	}

	if kubernetesutils.HasMetaDataAnnotation(&newBastion.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationReconcile) {
		return true
	}

	if kubernetesutils.HasMetaDataAnnotation(&newBastion.ObjectMeta, v1beta1constants.AnnotationConfirmationForceDeletion, "true") &&
		!kubernetesutils.HasMetaDataAnnotation(&oldBastion.ObjectMeta, v1beta1constants.AnnotationConfirmationForceDeletion, "true") {
		return true
	}

	return false
}

func (bastionStrategy) Validate(_ context.Context, obj runtime.Object) field.ErrorList {
	bastion := obj.(*operations.Bastion)
	return operationsvalidation.ValidateBastion(bastion)
}

func (bastionStrategy) Canonicalize(_ runtime.Object) {
}

func (bastionStrategy) AllowCreateOnUpdate() bool {
	return false
}

func (bastionStrategy) ValidateUpdate(_ context.Context, newObj, oldObj runtime.Object) field.ErrorList {
	oldBastion, newBastion := oldObj.(*operations.Bastion), newObj.(*operations.Bastion)
	return operationsvalidation.ValidateBastionUpdate(newBastion, oldBastion)
}

func (bastionStrategy) AllowUnconditionalUpdate() bool {
	return false
}

// WarningsOnCreate returns warnings to the client performing a create.
func (bastionStrategy) WarningsOnCreate(_ context.Context, _ runtime.Object) []string {
	return nil
}

// WarningsOnUpdate returns warnings to the client performing the update.
func (bastionStrategy) WarningsOnUpdate(_ context.Context, _, _ runtime.Object) []string {
	return nil
}

type bastionStatusStrategy struct {
	bastionStrategy
}

// StatusStrategy defines the storage strategy for the status subresource of Bastions.
var StatusStrategy = bastionStatusStrategy{Strategy}

func (s bastionStatusStrategy) PrepareForUpdate(_ context.Context, obj, old runtime.Object) {
	newBastion := obj.(*operations.Bastion)
	oldBastion := old.(*operations.Bastion)
	newBastion.Spec = oldBastion.Spec

	// recalculate to prevent manipulation
	expires := metav1.NewTime(newBastion.Status.LastHeartbeatTimestamp.Add(s.timeToLive))
	newBastion.Status.ExpirationTimestamp = &expires
}

func (bastionStatusStrategy) ValidateUpdate(_ context.Context, obj, old runtime.Object) field.ErrorList {
	return operationsvalidation.ValidateBastionStatusUpdate(obj.(*operations.Bastion), old.(*operations.Bastion))
}

// ToSelectableFields returns a field set that represents the object
func ToSelectableFields(bastion *operations.Bastion) fields.Set {
	// The purpose of allocation with a given number of elements is to reduce
	// amount of allocations needed to create the fields.Set. If you add any
	// field here or the number of object-meta related fields changes, this should
	// be adjusted.
	bastionSpecificFieldsSet := make(fields.Set, 4)
	bastionSpecificFieldsSet[operations.BastionSeedName] = getSeedName(bastion)
	bastionSpecificFieldsSet[operations.BastionShootName] = bastion.Spec.ShootRef.Name
	return generic.AddObjectMetaFieldsSet(bastionSpecificFieldsSet, &bastion.ObjectMeta, true)
}

// GetAttrs returns labels and fields of a given object for filtering purposes.
func GetAttrs(obj runtime.Object) (labels.Set, fields.Set, error) {
	bastion, ok := obj.(*operations.Bastion)
	if !ok {
		return nil, nil, fmt.Errorf("not a bastion")
	}
	return labels.Set(bastion.Labels), ToSelectableFields(bastion), nil
}

// MatchBastion returns a generic matcher for a given label and field selector.
func MatchBastion(label labels.Selector, field fields.Selector) storage.SelectionPredicate {
	return storage.SelectionPredicate{
		Label:       label,
		Field:       field,
		GetAttrs:    GetAttrs,
		IndexFields: []string{operations.BastionSeedName},
	}
}

// SeedNameTriggerFunc returns spec.seedName of given Bastion.
func SeedNameTriggerFunc(obj runtime.Object) string {
	bastion, ok := obj.(*operations.Bastion)
	if !ok {
		return ""
	}

	return getSeedName(bastion)
}

func getSeedName(bastion *operations.Bastion) string {
	if bastion.Spec.SeedName == nil {
		return ""
	}

	return *bastion.Spec.SeedName
}
