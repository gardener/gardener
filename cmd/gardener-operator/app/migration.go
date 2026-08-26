// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

func runMigrations(_ context.Context, _ client.Client, _ logr.Logger) manager.RunnableFunc {
	return func(context.Context) error {
		return nil
	}
}
