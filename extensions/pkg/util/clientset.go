// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package util

import (
	"github.com/gardener/gardener/pkg/client/kubernetes"
)

// ApplyClientConnectionConfigurationToRESTConfig applies the given client connection configurations to the given
// REST config.
var ApplyClientConnectionConfigurationToRESTConfig = kubernetes.ApplyClientConnectionConfigurationToRESTConfig
