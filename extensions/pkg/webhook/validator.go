// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package webhook

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Validator validates objects.
type Validator interface {
	Validate(ctx context.Context, new, old client.Object) error
}
