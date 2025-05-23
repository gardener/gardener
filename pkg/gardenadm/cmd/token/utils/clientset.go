// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"context"

	"github.com/go-logr/logr"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/gardenadm/botanist"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	botanistpkg "github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
)

// CreateClientSet creates a new client set using the AutonomousBotanist to create the client set.
// Exposed for unit testing.
var CreateClientSet = func(ctx context.Context, log logr.Logger) (kubernetes.Interface, error) {
	return (&botanist.AutonomousBotanist{Botanist: &botanistpkg.Botanist{Operation: &operation.Operation{Logger: log}}}).CreateClientSet(ctx)
}
