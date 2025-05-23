// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package rotation

import (
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ManagedByGardenerOperatorSecretsManager is the label selector for selecting secrets managed by gardener-operator's
// instance of secrets manager.
var ManagedByGardenerOperatorSecretsManager = client.MatchingLabels{
	"managed-by":       "secrets-manager",
	"manager-identity": "gardener-operator",
}
