// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package apiserver

import (
	"k8s.io/apimachinery/pkg/runtime/schema"

	gardencorev1 "github.com/gardener/gardener/pkg/apis/core/v1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	operationsv1alpha1 "github.com/gardener/gardener/pkg/apis/operations/v1alpha1"
	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	settingsv1alpha1 "github.com/gardener/gardener/pkg/apis/settings/v1alpha1"
)

// AllGardenerAPIGroupVersions is the list of all GroupVersions that are served by gardener-apiserver.
var AllGardenerAPIGroupVersions = []schema.GroupVersion{
	gardencorev1.SchemeGroupVersion,
	gardencorev1beta1.SchemeGroupVersion,
	settingsv1alpha1.SchemeGroupVersion,
	seedmanagementv1alpha1.SchemeGroupVersion,
	operationsv1alpha1.SchemeGroupVersion,
	securityv1alpha1.SchemeGroupVersion,
}
