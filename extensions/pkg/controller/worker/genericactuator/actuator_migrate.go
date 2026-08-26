// SPDX-FileCopyrightText: Copyright Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package genericactuator

import (
	"context"

	"github.com/go-logr/logr"

	"github.com/gardener/gardener/extensions/pkg/controller"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

// Migrate ensures that the MCM is deleted in case it is managed.
func (a *genericActuator) Migrate(_ context.Context, _ logr.Logger, _ *extensionsv1alpha1.Worker, _ *controller.Cluster) error {
	return nil
}
