// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package bastion

import (
	"context"

	"github.com/go-logr/logr"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

// Actuator acts upon Bastion resources.
type Actuator interface {
	// Reconcile reconciles the Bastion.
	Reconcile(context.Context, logr.Logger, *extensionsv1alpha1.Bastion, *extensionscontroller.Cluster) error
	// Delete deletes the Bastion.
	Delete(context.Context, logr.Logger, *extensionsv1alpha1.Bastion, *extensionscontroller.Cluster) error
	// ForceDelete forcefully deletes the Bastion.
	ForceDelete(context.Context, logr.Logger, *extensionsv1alpha1.Bastion, *extensionscontroller.Cluster) error
}
