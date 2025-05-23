// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package infrastructure

import (
	"context"

	"github.com/go-logr/logr"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

// Actuator acts upon Infrastructure resources.
type Actuator interface {
	// Reconcile the Infrastructure config.
	Reconcile(context.Context, logr.Logger, *extensionsv1alpha1.Infrastructure, *extensionscontroller.Cluster) error
	// Delete the Infrastructure config.
	Delete(context.Context, logr.Logger, *extensionsv1alpha1.Infrastructure, *extensionscontroller.Cluster) error
	// ForceDelete forcefully deletes the Infrastructure config.
	ForceDelete(context.Context, logr.Logger, *extensionsv1alpha1.Infrastructure, *extensionscontroller.Cluster) error
	// Restore takes the state of the Infrastructure resource and applies it to the terraform pod's output state
	Restore(context.Context, logr.Logger, *extensionsv1alpha1.Infrastructure, *extensionscontroller.Cluster) error
	// Migrate deletes the terraform k8s resources without deleting the corresponding resources in the IaaS provider
	Migrate(context.Context, logr.Logger, *extensionsv1alpha1.Infrastructure, *extensionscontroller.Cluster) error
}
