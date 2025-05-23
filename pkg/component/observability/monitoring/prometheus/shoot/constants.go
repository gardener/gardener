// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shoot

import (
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

const (
	// Label is a constant for the label of the shoot prometheus instance.
	Label = "shoot"
	// ServiceAccountName is the name of the service account in the shoot cluster.
	ServiceAccountName = "prometheus-" + Label
	// AccessSecretName is the name of the secret containing a token for accessing the shoot cluster.
	AccessSecretName = gardenerutils.SecretNamePrefixShootAccess + ServiceAccountName
)
