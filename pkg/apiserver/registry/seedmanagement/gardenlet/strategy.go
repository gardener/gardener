// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardenlet

import (
	"context"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/storage/names"

	"github.com/gardener/gardener/pkg/api"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/apis/seedmanagement"
	"github.com/gardener/gardener/pkg/apis/seedmanagement/validation"
	"github.com/gardener/gardener/pkg/apiserver/registry/seedmanagement/internal/utils"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

type gardenletStrategy struct {
	runtime.ObjectTyper
	names.NameGenerator
}

// NewStrategy return a storage strategy for gardenlets.
func NewStrategy() gardenletStrategy {
	return gardenletStrategy{
		api.Scheme,
		names.SimpleNameGenerator,
	}
}

func (gardenletStrategy) NamespaceScoped() bool { return true }

func (gardenletStrategy) PrepareForCreate(_ context.Context, obj runtime.Object) {
	gardenlet := obj.(*seedmanagement.Gardenlet)

	gardenlet.Generation = 1

	syncSeedBackupCredentials(gardenlet)
}

func (gardenletStrategy) PrepareForUpdate(_ context.Context, obj, old runtime.Object) {
	newGardenlet := obj.(*seedmanagement.Gardenlet)
	oldGardenlet := old.(*seedmanagement.Gardenlet)

	syncSeedBackupCredentials(newGardenlet)

	if mustIncreaseGeneration(oldGardenlet, newGardenlet) {
		newGardenlet.Generation = oldGardenlet.Generation + 1
	}
}

func mustIncreaseGeneration(oldGardenlet, newGardenlet *seedmanagement.Gardenlet) bool {
	// The specification changes.
	if !apiequality.Semantic.DeepEqual(oldGardenlet.Spec, newGardenlet.Spec) {
		return true
	}

	// The deletion timestamp was set.
	if oldGardenlet.DeletionTimestamp == nil && newGardenlet.DeletionTimestamp != nil {
		return true
	}

	// The operation annotation was added with value "reconcile"
	if kubernetesutils.HasMetaDataAnnotation(&newGardenlet.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationReconcile) {
		delete(newGardenlet.Annotations, v1beta1constants.GardenerOperation)
		return true
	}

	// The operation annotation was added with value "renew-kubeconfig"
	if kubernetesutils.HasMetaDataAnnotation(&newGardenlet.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationRenewKubeconfig) {
		return true
	}

	return false
}

func (gardenletStrategy) Validate(_ context.Context, obj runtime.Object) field.ErrorList {
	gardenlet := obj.(*seedmanagement.Gardenlet)
	return validation.ValidateGardenlet(gardenlet)
}

func (gardenletStrategy) ValidateUpdate(_ context.Context, newObj, oldObj runtime.Object) field.ErrorList {
	oldGardenlet, newGardenlet := oldObj.(*seedmanagement.Gardenlet), newObj.(*seedmanagement.Gardenlet)
	return validation.ValidateGardenletUpdate(newGardenlet, oldGardenlet)
}

func (gardenletStrategy) Canonicalize(_ runtime.Object) {
}

func (gardenletStrategy) AllowCreateOnUpdate() bool {
	return false
}

func (gardenletStrategy) AllowUnconditionalUpdate() bool {
	return false
}

func (gardenletStrategy) WarningsOnCreate(_ context.Context, _ runtime.Object) []string {
	return nil
}

func (gardenletStrategy) WarningsOnUpdate(_ context.Context, _, _ runtime.Object) []string {
	return nil
}

type statusStrategy struct {
	gardenletStrategy
}

// NewStatusStrategy defines the storage strategy for the status subresource of Gardenlets.
func NewStatusStrategy() statusStrategy {
	return statusStrategy{NewStrategy()}
}

func (s statusStrategy) PrepareForUpdate(_ context.Context, obj, old runtime.Object) {
	newGardenlet := obj.(*seedmanagement.Gardenlet)
	oldGardenlet := old.(*seedmanagement.Gardenlet)
	newGardenlet.Spec = oldGardenlet.Spec
}

func (statusStrategy) ValidateUpdate(_ context.Context, obj, old runtime.Object) field.ErrorList {
	return validation.ValidateGardenletStatusUpdate(obj.(*seedmanagement.Gardenlet), old.(*seedmanagement.Gardenlet))
}

func syncSeedBackupCredentials(gardenlet *seedmanagement.Gardenlet) {
	if gardenlet.Spec.Config == nil {
		return
	}
	gardenletConfig, ok := gardenlet.Spec.Config.(*gardenletconfigv1alpha1.GardenletConfiguration)
	if !ok {
		return
	}

	if gardenletConfig.SeedConfig == nil {
		return
	}

	utils.SyncBackupSecretRefAndCredentialsRef(gardenletConfig.SeedConfig.Spec.Backup)
}
