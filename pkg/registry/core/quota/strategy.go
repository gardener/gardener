// SPDX-FileCopyrightText: 2018 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package quota

import (
	"context"

	"github.com/gardener/gardener/pkg/api"
	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/apis/core/validation"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/storage/names"
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

func (quotaStrategy) PrepareForCreate(ctx context.Context, obj runtime.Object) {
}

func (quotaStrategy) Validate(ctx context.Context, obj runtime.Object) field.ErrorList {
	quota := obj.(*core.Quota)
	return validation.ValidateQuota(quota)
}

func (quotaStrategy) Canonicalize(obj runtime.Object) {
}

func (quotaStrategy) AllowCreateOnUpdate() bool {
	return false
}

func (quotaStrategy) PrepareForUpdate(ctx context.Context, newObj, oldObj runtime.Object) {
	_ = oldObj.(*core.Quota)
	_ = newObj.(*core.Quota)
}

func (quotaStrategy) ValidateUpdate(ctx context.Context, newObj, oldObj runtime.Object) field.ErrorList {
	oldQuota, newQuota := oldObj.(*core.Quota), newObj.(*core.Quota)
	return validation.ValidateQuotaUpdate(newQuota, oldQuota)
}

func (quotaStrategy) AllowUnconditionalUpdate() bool {
	return true
}
