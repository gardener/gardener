// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shoot

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	gardencorehelper "github.com/gardener/gardener/pkg/apis/core/helper"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/apis/core/validation"
	"github.com/gardener/gardener/pkg/features"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/plugin/pkg/utils"
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

func (shootStrategy) PrepareForCreate(_ context.Context, obj runtime.Object) {
	newShoot := obj.(*core.Shoot)

	newShoot.Generation = 1
	newShoot.Status = core.ShootStatus{}

	utils.SyncCloudProfileFields(nil, newShoot)

	if !utilfeature.DefaultFeatureGate.Enabled(features.ShootCredentialsBinding) {
		newShoot.Spec.CredentialsBindingName = nil
	}
}

func (shootStrategy) PrepareForUpdate(_ context.Context, obj, old runtime.Object) {
	newShoot := obj.(*core.Shoot)
	oldShoot := old.(*core.Shoot)

	newShoot.Status = oldShoot.Status               // can only be changed by shoots/status subresource
	newShoot.Spec.SeedName = oldShoot.Spec.SeedName // can only be changed by shoots/binding subresource

	if mustIncreaseGeneration(oldShoot, newShoot) {
		newShoot.Generation = oldShoot.Generation + 1
	}

	utils.SyncCloudProfileFields(oldShoot, newShoot)

	if oldShoot.Spec.CredentialsBindingName == nil && !utilfeature.DefaultFeatureGate.Enabled(features.ShootCredentialsBinding) {
		newShoot.Spec.CredentialsBindingName = nil
	}

	syncLegacyAccessRestrictionLabelWithNewFieldOnUpdate(newShoot, oldShoot)
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

	// Force delete annotation is set.
	// This is necessary because we want to trigger a reconciliation right away even if the Shoot is failed.
	if !gardencorehelper.ShootNeedsForceDeletion(oldShoot) && gardencorehelper.ShootNeedsForceDeletion(newShoot) {
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
				v1beta1constants.OperationRotateCredentialsStartWithoutWorkersRollout,
				v1beta1constants.OperationRotateCredentialsComplete,
				v1beta1constants.OperationRotateCAStart,
				v1beta1constants.OperationRotateCAStartWithoutWorkersRollout,
				v1beta1constants.OperationRotateCAComplete,
				v1beta1constants.OperationRotateServiceAccountKeyStart,
				v1beta1constants.OperationRotateServiceAccountKeyStartWithoutWorkersRollout,
				v1beta1constants.OperationRotateServiceAccountKeyComplete,
				v1beta1constants.OperationRotateETCDEncryptionKeyStart,
				v1beta1constants.OperationRotateETCDEncryptionKeyComplete,
				v1beta1constants.OperationRotateObservabilityCredentials:
				// We don't want to remove the annotation so that the gardenlet can pick it up and perform
				// the rotation. It has to remove the annotation after it is done.
				mustIncrease, mustRemoveOperationAnnotation = true, false

			case v1beta1constants.ShootOperationRotateSSHKeypair:
				if !gardencorehelper.ShootEnablesSSHAccess(newShoot) {
					// If SSH is not enabled for the Shoot, don't increase generation, just remove the annotation
					mustIncrease, mustRemoveOperationAnnotation = false, true
				} else {
					mustIncrease, mustRemoveOperationAnnotation = true, false
				}
			}

			if strings.HasPrefix(newShoot.Annotations[v1beta1constants.GardenerOperation], v1beta1constants.OperationRotateRolloutWorkers) {
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

	// Shoot needs to be reconciled when the disable-istio-tls-termination annotation changes.
	if newShoot.Annotations[v1beta1constants.ShootDisableIstioTLSTermination] != oldShoot.Annotations[v1beta1constants.ShootDisableIstioTLSTermination] {
		return true
	}

	return false
}

func mustIncreaseGenerationForSpecChanges(oldShoot, newShoot *core.Shoot) bool {
	if newShoot.Spec.Maintenance != nil && newShoot.Spec.Maintenance.ConfineSpecUpdateRollout != nil && *newShoot.Spec.Maintenance.ConfineSpecUpdateRollout {
		return gardencorehelper.HibernationIsEnabled(oldShoot) != gardencorehelper.HibernationIsEnabled(newShoot)
	}

	return !apiequality.Semantic.DeepEqual(oldShoot.Spec, newShoot.Spec)
}

func (shootStrategy) Validate(_ context.Context, obj runtime.Object) field.ErrorList {
	shoot := obj.(*core.Shoot)
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, validation.ValidateShoot(shoot)...)
	allErrs = append(allErrs, validation.ValidateForceDeletion(shoot, nil)...)
	allErrs = append(allErrs, validation.ValidateFinalizersOnCreation(shoot.Finalizers, field.NewPath("metadata", "finalizers"))...)
	return allErrs
}

func (shootStrategy) Canonicalize(obj runtime.Object) {
	shoot := obj.(*core.Shoot)

	gardenerutils.MaintainSeedNameLabels(shoot, shoot.Spec.SeedName, shoot.Status.SeedName)
	syncLegacyAccessRestrictionLabelWithNewField(shoot)

	// TODO(shafeeqes): Remove this in gardener v1.120
	shoot.Spec.Kubernetes.EnableStaticTokenKubeconfig = nil
	if shoot.Status.Credentials != nil && shoot.Status.Credentials.Rotation != nil {
		shoot.Status.Credentials.Rotation.Kubeconfig = nil
	}
}

func (shootStrategy) AllowCreateOnUpdate() bool {
	return false
}

func (shootStrategy) ValidateUpdate(_ context.Context, newObj, oldObj runtime.Object) field.ErrorList {
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

func (shootStatusStrategy) PrepareForUpdate(_ context.Context, obj, old runtime.Object) {
	newShoot := obj.(*core.Shoot)
	oldShoot := old.(*core.Shoot)
	newShoot.Spec = oldShoot.Spec

	if lastOperation := newShoot.Status.LastOperation; lastOperation != nil && lastOperation.Type == core.LastOperationTypeMigrate &&
		(lastOperation.State == core.LastOperationStateSucceeded || lastOperation.State == core.LastOperationStateAborted) {
		newShoot.Generation = oldShoot.Generation + 1
	}
}

func (shootStatusStrategy) ValidateUpdate(_ context.Context, obj, old runtime.Object) field.ErrorList {
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

func (shootBindingStrategy) PrepareForUpdate(_ context.Context, obj, old runtime.Object) {
	newShoot := obj.(*core.Shoot)
	oldShoot := old.(*core.Shoot)

	newShoot.Status = oldShoot.Status

	// Remove "Create Pending" from status if seed name got set
	if lastOp := newShoot.Status.LastOperation; lastOp != nil &&
		lastOp.Type == core.LastOperationTypeCreate && lastOp.State == core.LastOperationStatePending &&
		oldShoot.Spec.SeedName == nil && newShoot.Spec.SeedName != nil {
		newShoot.Status.LastOperation = nil
	}

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
func ToSelectableFields(shoot *core.Shoot) fields.Set {
	// The purpose of allocation with a given number of elements is to reduce
	// amount of allocations needed to create the fields.Set. If you add any
	// field here or the number of object-meta related fields changes, this should
	// be adjusted.
	shootSpecificFieldsSet := make(fields.Set, 7)
	shootSpecificFieldsSet[core.ShootSeedName] = getSeedName(shoot)
	shootSpecificFieldsSet[core.ShootStatusSeedName] = getStatusSeedName(shoot)
	if shoot.Spec.CloudProfileName != nil {
		shootSpecificFieldsSet[core.ShootCloudProfileName] = *shoot.Spec.CloudProfileName
	}
	if shoot.Spec.CloudProfile != nil {
		shootSpecificFieldsSet[core.ShootCloudProfileRefName] = shoot.Spec.CloudProfile.Name
		shootSpecificFieldsSet[core.ShootCloudProfileRefKind] = shoot.Spec.CloudProfile.Kind
	}
	return generic.AddObjectMetaFieldsSet(shootSpecificFieldsSet, &shoot.ObjectMeta, true)
}

// GetAttrs returns labels and fields of a given object for filtering purposes.
func GetAttrs(obj runtime.Object) (labels.Set, fields.Set, error) {
	shoot, ok := obj.(*core.Shoot)
	if !ok {
		return nil, nil, fmt.Errorf("not a shoot")
	}
	return shoot.ObjectMeta.Labels, ToSelectableFields(shoot), nil
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

// TODO(rfranzke): Remove everything below this line and the legacy access restriction label after
// https://github.com/gardener/dashboard/issues/2120 has been merged and ~6 months have passed to make sure all clients
// have adapted to the new fields in the specifications, and are rolled out.
func syncLegacyAccessRestrictionLabelWithNewField(shoot *core.Shoot) {
	if shoot.Spec.SeedSelector != nil && shoot.Spec.SeedSelector.MatchLabels["seed.gardener.cloud/eu-access"] == "true" {
		if !slices.ContainsFunc(shoot.Spec.AccessRestrictions, func(accessRestriction core.AccessRestrictionWithOptions) bool {
			return accessRestriction.Name == "eu-access-only"
		}) {
			shoot.Spec.AccessRestrictions = append(shoot.Spec.AccessRestrictions, core.AccessRestrictionWithOptions{AccessRestriction: core.AccessRestriction{Name: "eu-access-only"}})
		}
	}

	if slices.ContainsFunc(shoot.Spec.AccessRestrictions, func(accessRestriction core.AccessRestrictionWithOptions) bool {
		return accessRestriction.Name == "eu-access-only"
	}) {
		if shoot.Spec.SeedSelector == nil {
			shoot.Spec.SeedSelector = &core.SeedSelector{}
		}
		if shoot.Spec.SeedSelector.MatchLabels == nil {
			shoot.Spec.SeedSelector.MatchLabels = make(map[string]string)
		}
		shoot.Spec.SeedSelector.MatchLabels["seed.gardener.cloud/eu-access"] = "true"
	}

	if i := slices.IndexFunc(shoot.Spec.AccessRestrictions, func(accessRestriction core.AccessRestrictionWithOptions) bool {
		return accessRestriction.Name == "eu-access-only"
	}); i != -1 {
		for _, key := range []string{
			"support.gardener.cloud/eu-access-for-cluster-addons",
			"support.gardener.cloud/eu-access-for-cluster-nodes",
		} {
			if v, ok := shoot.Annotations[key]; ok {
				if shoot.Spec.AccessRestrictions[i].Options == nil {
					shoot.Spec.AccessRestrictions[i].Options = make(map[string]string)
				}
				shoot.Spec.AccessRestrictions[i].Options[key] = v
			}
		}

		for k, v := range shoot.Spec.AccessRestrictions[i].Options {
			if k == "support.gardener.cloud/eu-access-for-cluster-addons" || k == "support.gardener.cloud/eu-access-for-cluster-nodes" {
				metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, k, v)
			}
		}
	}
}

func syncLegacyAccessRestrictionLabelWithNewFieldOnUpdate(shoot, oldShoot *core.Shoot) {
	filterEUAccessOnlyRestriction := func(accessRestriction core.AccessRestrictionWithOptions) bool {
		return accessRestriction.Name == "eu-access-only"
	}

	hasAccessRestriction := func(accessRestrictions []core.AccessRestrictionWithOptions) bool {
		return slices.ContainsFunc(accessRestrictions, filterEUAccessOnlyRestriction)
	}

	removeAccessRestriction := func(accessRestrictions []core.AccessRestrictionWithOptions, name string) []core.AccessRestrictionWithOptions {
		var updatedAccessRestrictions []core.AccessRestrictionWithOptions
		for _, accessRestriction := range accessRestrictions {
			if accessRestriction.Name != name {
				updatedAccessRestrictions = append(updatedAccessRestrictions, accessRestriction)
			}
		}
		return updatedAccessRestrictions
	}

	updateOptionInAccessRestrictions := func(accessRestrictions []core.AccessRestrictionWithOptions, key, value string) {
		i := slices.IndexFunc(accessRestrictions, filterEUAccessOnlyRestriction)
		if i == -1 {
			return
		}
		if accessRestrictions[i].Options == nil {
			accessRestrictions[i].Options = make(map[string]string)
		}
		accessRestrictions[i].Options[key] = value
	}

	removeOptionFromAccessRestrictions := func(accessRestrictions []core.AccessRestrictionWithOptions, key string) {
		i := slices.IndexFunc(accessRestrictions, filterEUAccessOnlyRestriction)
		if i == -1 {
			return
		}
		delete(accessRestrictions[i].Options, key)
	}

	if oldShoot.Spec.SeedSelector != nil && oldShoot.Spec.SeedSelector.MatchLabels["seed.gardener.cloud/eu-access"] == "true" &&
		(shoot.Spec.SeedSelector == nil || shoot.Spec.SeedSelector.MatchLabels["seed.gardener.cloud/eu-access"] != "true") {
		shoot.Spec.AccessRestrictions = removeAccessRestriction(shoot.Spec.AccessRestrictions, "eu-access-only")
	}

	for _, key := range []string{
		"support.gardener.cloud/eu-access-for-cluster-addons",
		"support.gardener.cloud/eu-access-for-cluster-nodes",
	} {
		oldValue, oldOk := oldShoot.Annotations[key]
		newValue, newOk := shoot.Annotations[key]
		if oldOk && newOk && oldValue != newValue {
			updateOptionInAccessRestrictions(shoot.Spec.AccessRestrictions, key, shoot.Annotations[key])
		}

		if hasAccessRestriction(oldShoot.Spec.AccessRestrictions) && hasAccessRestriction(shoot.Spec.AccessRestrictions) {
			oldValue, oldOk := oldShoot.Spec.AccessRestrictions[slices.IndexFunc(oldShoot.Spec.AccessRestrictions, filterEUAccessOnlyRestriction)].Options[key]
			newValue, newOk := shoot.Spec.AccessRestrictions[slices.IndexFunc(shoot.Spec.AccessRestrictions, filterEUAccessOnlyRestriction)].Options[key]
			if oldOk && newOk && oldValue != newValue {
				metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, key, shoot.Spec.AccessRestrictions[slices.IndexFunc(shoot.Spec.AccessRestrictions, filterEUAccessOnlyRestriction)].Options[key])
			}
		}

		if oldShoot.Annotations[key] != "" &&
			shoot.Annotations[key] == "" {
			removeOptionFromAccessRestrictions(shoot.Spec.AccessRestrictions, key)
		}
	}
}
