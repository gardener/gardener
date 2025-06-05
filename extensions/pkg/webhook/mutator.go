// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package webhook

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Mutator validates and if needed mutates objects.
type Mutator interface {
	// Mutate validates and if needed mutates the given object.
	// "old" is optional, and it must always be checked for nil.
	Mutate(ctx context.Context, new, old client.Object) error
}
