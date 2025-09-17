// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shoot

import (
	"net/http"

	"github.com/go-logr/logr"

	"github.com/gardener/gardener/pkg/client/kubernetes"
)

// Webhook represents the webhook of Shoot Authorizer.
type Webhook struct {
	Logger    logr.Logger
	ClientSet kubernetes.Interface
	Handler   http.Handler
}
