// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package care

import (
	"context"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

// HealthCheck is an interface used to perform health checks.
type HealthCheck interface {
	Check(ctx context.Context, conditions ExtensionConditions) []gardencorev1beta1.Condition
}
