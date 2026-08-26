// SPDX-FileCopyrightText: Copyright Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package util

import (
	"github.com/gardener/gardener/pkg/client/kubernetes"
)

// ApplyClientConnectionConfigurationToRESTConfig applies the given client connection configurations to the given
// REST config.
var ApplyClientConnectionConfigurationToRESTConfig = kubernetes.ApplyClientConnectionConfigurationToRESTConfig
