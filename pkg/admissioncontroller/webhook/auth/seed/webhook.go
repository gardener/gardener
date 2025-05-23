// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package seed

import (
	"net/http"

	"github.com/go-logr/logr"
)

// Webhook represents the webhook of Seed Authorizer.
type Webhook struct {
	Logger  logr.Logger
	Handler http.Handler
}
