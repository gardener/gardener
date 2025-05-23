// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package quota

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/storage/names"

	"github.com/gardener/gardener/pkg/api"
	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/apis/core/validation"
)

type quotaStrategy struct {
	runtime.ObjectTyper
	names.NameGenerator
}

// Strategy defines the storage strategy for Quotas.
var Strategy = quotaStrategy{api.Scheme, names.SimpleNameGenerator}

func (quotaStrategy) NamespaceScoped() bool {
	return true
}

func (quotaStrategy) PrepareForCreate(_ context.Context, _ runtime.Object) {
}

func (quotaStrategy) Validate(_ context.Context, obj runtime.Object) field.ErrorList {
	quota := obj.(*core.Quota)
	return validation.ValidateQuota(quota)
}

func (quotaStrategy) Canonicalize(_ runtime.Object) {
}

func (quotaStrategy) AllowCreateOnUpdate() bool {
	return false
}

func (quotaStrategy) PrepareForUpdate(_ context.Context, newObj, oldObj runtime.Object) {
	_ = oldObj.(*core.Quota)
	_ = newObj.(*core.Quota)
}

func (quotaStrategy) ValidateUpdate(_ context.Context, newObj, oldObj runtime.Object) field.ErrorList {
	oldQuota, newQuota := oldObj.(*core.Quota), newObj.(*core.Quota)
	return validation.ValidateQuotaUpdate(newQuota, oldQuota)
}

func (quotaStrategy) AllowUnconditionalUpdate() bool {
	return true
}

// WarningsOnCreate returns warnings to the client performing a create.
func (quotaStrategy) WarningsOnCreate(_ context.Context, _ runtime.Object) []string {
	return nil
}

// WarningsOnUpdate returns warnings to the client performing the update.
func (quotaStrategy) WarningsOnUpdate(_ context.Context, _, _ runtime.Object) []string {
	return nil
}
