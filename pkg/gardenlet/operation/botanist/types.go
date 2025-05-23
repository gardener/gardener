// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"github.com/gardener/gardener/pkg/gardenlet/operation"
)

// Botanist is a struct which has methods that perform cloud-independent operations for a Shoot cluster.
type Botanist struct {
	*operation.Operation
}
