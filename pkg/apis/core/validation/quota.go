// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/apis/core/helper"
	kubernetescorevalidation "github.com/gardener/gardener/pkg/utils/validation/kubernetes/core"
)

// ValidateQuota validates a Quota object.
func ValidateQuota(quota *core.Quota) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMeta(&quota.ObjectMeta, true, ValidateName, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidateQuotaSpec(&quota.Spec, field.NewPath("spec"))...)

	return allErrs
}

// ValidateQuotaUpdate validates a Quota object before an update.
func ValidateQuotaUpdate(newQuota, oldQuota *core.Quota) field.ErrorList {
	allErrs := apivalidation.ValidateObjectMetaUpdate(&newQuota.ObjectMeta, &oldQuota.ObjectMeta, field.NewPath("metadata"))
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(&newQuota.Spec.Scope, &oldQuota.Spec.Scope, field.NewPath("spec").Child("scope"))...)
	allErrs = append(allErrs, ValidateQuota(newQuota)...)
	return allErrs
}

// ValidateQuotaSpec validates the specification of a Quota object.
func ValidateQuotaSpec(quotaSpec *core.QuotaSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	scopeRef := quotaSpec.Scope
	if _, err := helper.QuotaScope(scopeRef); err != nil {
		allErrs = append(allErrs, field.NotSupported(fldPath.Child("scope"), scopeRef, []string{"project", "secret", "workloadidentity"}))
	}

	metricsFldPath := fldPath.Child("metrics")
	for k, v := range quotaSpec.Metrics {
		keyPath := metricsFldPath.Key(string(k))
		if !isValidQuotaMetric(k) {
			allErrs = append(allErrs, field.Invalid(keyPath, v.String(), fmt.Sprintf("%s is no supported quota metric", string(k))))
		}
		allErrs = append(allErrs, kubernetescorevalidation.ValidateResourceQuantityValue(k.String(), v, keyPath)...)
	}

	return allErrs
}

func isValidQuotaMetric(metric corev1.ResourceName) bool {
	switch metric {
	case
		core.QuotaMetricCPU,
		core.QuotaMetricGPU,
		core.QuotaMetricMemory,
		core.QuotaMetricStorageStandard,
		core.QuotaMetricStoragePremium,
		core.QuotaMetricLoadbalancer:
		return true
	}
	return false
}
