// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/go-logr/logr"
)

func (g *garden) runMigrations(_ context.Context, _ logr.Logger) error {
	return nil
}
