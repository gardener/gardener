// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package bastion

import (
	"context"

	"k8s.io/apimachinery/pkg/util/validation/field"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/extensions"
)

// ConfigValidator validates the provider config of bastion resource with the cloud provider.
type ConfigValidator interface {
	// Validate validates the provider config of the given bastion and cluster resources used by Bastion.
	// If the returned error list is non-empty, the reconciliation will fail with an error.
	// This error will have the error code ERR_CONFIGURATION_PROBLEM, unless there is at least one error in the list
	// that has its ErrorType field set to field.ErrorTypeInternal.
	Validate(ctx context.Context, bastion *extensionsv1alpha1.Bastion, cluster *extensions.Cluster) field.ErrorList
}
