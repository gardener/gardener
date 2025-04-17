// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package care

import (
	"context"
	"time"

	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

// NewHealthCheckFunc is a function used to create a new instance for performing health checks.
type NewHealthCheckFunc func(*gardencorev1beta1.Seed, client.Client, clock.Clock, *string, map[gardencorev1beta1.ConditionType]time.Duration) HealthCheck

// defaultNewHealthCheck is the default function to create a new instance for performing health checks.
var defaultNewHealthCheck NewHealthCheckFunc = NewHealth

// HealthCheck is an interface used to perform health checks.
type HealthCheck interface {
	Check(ctx context.Context, condition SeedConditions) []gardencorev1beta1.Condition
}
