// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package rotation

import (
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ManagedByGardenletSecretsManager is the label selector for selecting secrets managed by gardenlet's instance of
// secrets manager.
var ManagedByGardenletSecretsManager = client.MatchingLabels{
	"managed-by":       "secrets-manager",
	"manager-identity": "gardenlet",
}
