// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package garden

import (
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

const (
	// Label is a constant for the label of the garden prometheus instance.
	Label = "garden"
	// ServiceAccountName is the name of the service account in the virtual garden cluster.
	ServiceAccountName = "prometheus-" + Label
	// AccessSecretName is the name of the secret containing a token for accessing the virtual garden cluster.
	AccessSecretName = gardenerutils.SecretNamePrefixShootAccess + ServiceAccountName
)
