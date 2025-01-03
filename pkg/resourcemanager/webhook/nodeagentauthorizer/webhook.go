// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package nodeagentauthorizer

import (
	"net/http"

	"github.com/go-logr/logr"

	resourcemanagerconfigv1alpha1 "github.com/gardener/gardener/pkg/resourcemanager/apis/config/v1alpha1"
)

// Webhook represents the webhook of node-agent authorizer.
type Webhook struct {
	Logger  logr.Logger
	Handler http.Handler
	Config  resourcemanagerconfigv1alpha1.NodeAgentAuthorizerWebhookConfig
}
