// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package nodeagentauthorizer

import (
	"net/http"

	"github.com/go-logr/logr"

	"github.com/gardener/gardener/pkg/resourcemanager/apis/config"
)

// Webhook represents the webhook of Node-Agent Authorizer
type Webhook struct {
	Logger  logr.Logger
	Handler http.Handler
	Config  config.NodeAgentAuthorizerWebhookConfig
}
