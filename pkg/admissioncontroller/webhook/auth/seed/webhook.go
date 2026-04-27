// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package seed

import (
	"net/http"

	"github.com/go-logr/logr"
	auth "k8s.io/apiserver/pkg/authorization/authorizer"

	"github.com/gardener/gardener/pkg/client/kubernetes"
)

// Webhook represents the webhook of Seed Authorizer.
type Webhook struct {
	Logger     logr.Logger
	ClientSet  kubernetes.Interface
	Handler    http.Handler
	Authorizer auth.Authorizer
}
