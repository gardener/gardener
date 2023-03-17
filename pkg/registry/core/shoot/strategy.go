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
	"time"

	"github.com/Masterminds/semver"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/apiserver/pkg/storage"
	"k8s.io/apiserver/pkg/storage/names"
	utilfeature "k8s.io/apiserver/pkg/util/feature"

	"github.com/gardener/gardener/pkg/api"
	"github.com/gardener/gardener/pkg/api/core/shoot"
	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/apis/core/helper"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/apis/core/validation"
	"github.com/gardener/gardener/pkg/features"
	versionutils "github.com/gardener/gardener/pkg/utils/version"
)

type shootStrategy struct {
	runtime.ObjectTyper
	names.NameGenerator

	credentialsRotationInterval time.Duration
}

// NewStrategy returns a new storage strategy for Shoots.
func NewStrategy(credentialsRotationInterval time.Duration) shootStrategy {
	return shootStrategy{api.Scheme, names.SimpleNameGenerator, credentialsRotationInterval}
}

// Strategy should implement rest.RESTCreateUpdateStrategy
var _ rest.RESTCreateUpdateStrategy = shootStrategy{}

func (shootStrategy) NamespaceScoped() bool {
	return true
}

func (shootStrategy) PrepareForCreate(ctx context.Context, obj runtime.Object) {
	shoot := obj.(*core.Shoot)

	shoot.Generation = 1
	shoot.Status = core.ShootStatus{}

	dropDisabledFields(shoot, nil)
	dropEnableBasicAuthenticationField(shoot)
	setKubernetesDashboardAuthMode(shoot)
}

func (shootStrategy) PrepareForUpdate(ctx context.Context, obj, old runtime.Object) {
	newShoot := obj.(*core.Shoot)
	oldShoot := old.(*core.Shoot)

	newShoot.Status = oldShoot.Status               // can only be changed by shoots/status subresource
	newShoot.Spec.SeedName = oldShoot.Spec.SeedName // can only be changed by shoots/binding subresource

	if mustIncreaseGeneration(oldShoot, newShoot) {
		newShoot.Generation = oldShoot.Generation + 1
	}

	dropDisabledFields(newShoot, oldShoot)
}

// dropDisabledFields removes disabled fields from shoot.
func dropDisabledFields(newShoot, oldShoot *core.Shoot) {
	// Removes disabled HighAvailability related fields from shoot spec if it is not already used by the old spec
	oldShootIsHA := oldShoot != nil && helper.IsHAControlPlaneConfigured(oldShoot)
	if !utilfeature.DefaultFeatureGate.Enabled(features.HAControlPlanes) && !oldShootIsHA && newShoot.Spec.ControlPlane != nil {
		newShoot.Spec.ControlPlane.HighAvailability = nil
	}
}

func mustIncreaseGeneration(oldShoot, newShoot *core.Shoot) bool {
	// The Shoot specification changes.
	if mustIncreaseGenerationForSpecChanges(oldShoot, newShoot) {
		return true
	}

	// The deletion timestamp is set.
	if oldShoot.DeletionTimestamp == nil && newShoot.DeletionTimestamp != nil {
		return true
	}

	if lastOperation := newShoot.Status.LastOperation; lastOperation != nil {
		var (
			mustIncrease                  bool
			mustRemoveOperationAnnotation bool
		)

		switch lastOperation.State {
		case core.LastOperationStateFailed:
			if val, ok := newShoot.Annotations[v1beta1constants.GardenerOperation]; ok && val == v1beta1constants.ShootOperationRetry {
				mustIncrease, mustRemoveOperationAnnotation = true, true
			}

		default:
			switch newShoot.Annotations[v1beta1constants.GardenerOperation] {
			case v1beta1constants.GardenerOperationReconcile:
				mustIncrease, mustRemoveOperationAnnotation = true, true

			case v1beta1constants.OperationRotateCredentialsStart,
				v1beta1constants.OperationRotateCredentialsComplete,
				v1beta1constants.OperationRotateCAStart,
				v1beta1constants.OperationRotateCAComplete,
				v1beta1constants.OperationRotateServiceAccountKeyStart,
				v1beta1constants.OperationRotateServiceAccountKeyComplete,
				v1beta1constants.OperationRotateETCDEncryptionKeyStart,
				v1beta1constants.OperationRotateETCDEncryptionKeyComplete,
				v1beta1constants.ShootOperationRotateKubeconfigCredentials,
				v1beta1constants.ShootOperationRotateSSHKeypair,
				v1beta1constants.ShootOperationRotateObservabilityCredentials:
				// We don't want to remove the annotation so that the gardenlet can pick it up and perform
				// the rotation. It has to remove the annotation after it is done.
				mustIncrease, mustRemoveOperationAnnotation = true, false
			}
		}

		if mustRemoveOperationAnnotation {
			delete(newShoot.Annotations, v1beta1constants.GardenerOperation)
		}
		if mustIncrease {
			return true
		}
	}

	return false
}

func mustIncreaseGenerationForSpecChanges(oldShoot, newShoot *core.Shoot) bool {
	if newShoot.Spec.Maintenance != nil && newShoot.Spec.Maintenance.ConfineSpecUpdateRollout != nil && *newShoot.Spec.Maintenance.ConfineSpecUpdateRollout {
		return helper.HibernationIsEnabled(oldShoot) != helper.HibernationIsEnabled(newShoot)
	}

	return !apiequality.Semantic.DeepEqual(oldShoot.Spec, newShoot.Spec)
}

func (shootStrategy) Validate(ctx context.Context, obj runtime.Object) field.ErrorList {
	shoot := obj.(*core.Shoot)
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, validation.ValidateShoot(shoot)...)
	allErrs = append(allErrs, validation.ValidateTotalNodeCountWithPodCIDR(shoot)...)
	return allErrs
}

func (shootStrategy) Canonicalize(obj runtime.Object) {
	shoot := obj.(*core.Shoot)
	if versionutils.ConstraintK8sGreaterEqual125.Check(semver.MustParse(shoot.Spec.Kubernetes.Version)) {
		cleanupAdmissionPlugins(shoot)
	}
	dropEnableBasicAuthenticationField(shoot)
	setKubernetesDashboardAuthMode(shoot)
}

func cleanupAdmissionPlugins(shoot *core.Shoot) {
	var (
		admissionPlugins      []core.AdmissionPlugin
		shootAdmissionPlugins = shoot.Spec.Kubernetes.KubeAPIServer.AdmissionPlugins
	)

	for _, plugin := range shootAdmissionPlugins {
		if plugin.Name != "PodSecurityPolicy" {
			admissionPlugins = append(admissionPlugins, plugin)
		}
	}

	shoot.Spec.Kubernetes.KubeAPIServer.AdmissionPlugins = admissionPlugins
}

// dropEnableBasicAuthenticationField sets the enableBasicAuthentication to nil.
func dropEnableBasicAuthenticationField(shoot *core.Shoot) {
	if shoot.Spec.Kubernetes.KubeAPIServer != nil {
		shoot.Spec.Kubernetes.KubeAPIServer.EnableBasicAuthentication = nil
	}
}

// setKubernetesDashboardAuthMode sets the kubernetes-dashboard authentication mode to "token", if its current value is not "token" (for example "basic").
func setKubernetesDashboardAuthMode(shoot *core.Shoot) {
	if shoot.Spec.Addons != nil && shoot.Spec.Addons.KubernetesDashboard != nil {
		if authMode := shoot.Spec.Addons.KubernetesDashboard.AuthenticationMode; authMode != nil && *authMode != core.KubernetesDashboardAuthModeToken {
			defaultAuthMode := core.KubernetesDashboardAuthModeToken
			shoot.Spec.Addons.KubernetesDashboard.AuthenticationMode = &defaultAuthMode
		}
	}
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

// WarningsOnCreate returns warnings to the client performing a create.
func (s shootStrategy) WarningsOnCreate(ctx context.Context, obj runtime.Object) []string {
	return shoot.GetWarnings(ctx, obj.(*core.Shoot), nil, s.credentialsRotationInterval)
}

// WarningsOnUpdate returns warnings to the client performing the update.
func (s shootStrategy) WarningsOnUpdate(ctx context.Context, obj, old runtime.Object) []string {
	return shoot.GetWarnings(ctx, obj.(*core.Shoot), old.(*core.Shoot), s.credentialsRotationInterval)
}

type shootStatusStrategy struct {
	shootStrategy
}

// NewStatusStrategy returns a new storage strategy for the status subresource of Shoots.
func NewStatusStrategy() shootStatusStrategy {
	return shootStatusStrategy{NewStrategy(0)}
}

func (shootStatusStrategy) PrepareForUpdate(ctx context.Context, obj, old runtime.Object) {
	newShoot := obj.(*core.Shoot)
	oldShoot := old.(*core.Shoot)
	newShoot.Spec = oldShoot.Spec

	if lastOperation := newShoot.Status.LastOperation; lastOperation != nil && lastOperation.Type == core.LastOperationTypeMigrate &&
		(lastOperation.State == core.LastOperationStateSucceeded || lastOperation.State == core.LastOperationStateAborted) {
		newShoot.Generation = oldShoot.Generation + 1
	}
}

func (shootStatusStrategy) ValidateUpdate(ctx context.Context, obj, old runtime.Object) field.ErrorList {
	return validation.ValidateShootStatusUpdate(obj.(*core.Shoot).Status, old.(*core.Shoot).Status)
}

func (shootStatusStrategy) WarningsOnCreate(_ context.Context, _ runtime.Object) []string {
	return nil
}

func (shootStatusStrategy) WarningsOnUpdate(_ context.Context, _, _ runtime.Object) []string {
	return nil
}

type shootBindingStrategy struct {
	shootStrategy
}

// NewBindingStrategy returns a new storage strategy for the binding subresource of Shoots.
func NewBindingStrategy() shootBindingStrategy {
	return shootBindingStrategy{NewStrategy(0)}
}

func (shootBindingStrategy) PrepareForUpdate(ctx context.Context, obj, old runtime.Object) {
	newShoot := obj.(*core.Shoot)
	oldShoot := old.(*core.Shoot)

	newShoot.Status = oldShoot.Status

	if !apiequality.Semantic.DeepEqual(oldShoot.Spec, newShoot.Spec) {
		newShoot.Generation = oldShoot.Generation + 1
	}
}

func (shootBindingStrategy) WarningsOnCreate(_ context.Context, _ runtime.Object) []string {
	return nil
}

func (shootBindingStrategy) WarningsOnUpdate(_ context.Context, _, _ runtime.Object) []string {
	return nil
}

// ToSelectableFields returns a field set that represents the object
// TODO: fields are not labels, and the validation rules for them do not apply.
func ToSelectableFields(shoot *core.Shoot) fields.Set {
	// The purpose of allocation with a given number of elements is to reduce
	// amount of allocations needed to create the fields.Set. If you add any
	// field here or the number of object-meta related fields changes, this should
	// be adjusted.
	shootSpecificFieldsSet := make(fields.Set, 5)
	shootSpecificFieldsSet[core.ShootSeedName] = getSeedName(shoot)
	shootSpecificFieldsSet[core.ShootStatusSeedName] = getStatusSeedName(shoot)
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

func getStatusSeedName(shoot *core.Shoot) string {
	if shoot.Status.SeedName == nil {
		return ""
	}
	return *shoot.Status.SeedName
}
