// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

package care

import (
	"context"
	"time"

	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	seedpkg "github.com/gardener/gardener/pkg/operation/seed"
)

var defaultNewSeedObjectFunc = func(ctx context.Context, seed *gardencorev1beta1.Seed) (*seedpkg.Seed, error) {
	return seedpkg.NewBuilder().WithSeedObject(seed).Build(ctx)
}

// NewHealthCheckFunc is a function used to create a new instance for performing health checks.
type NewHealthCheckFunc func(*gardencorev1beta1.Seed, client.Client, clock.Clock, *string, bool, bool, bool, map[gardencorev1beta1.ConditionType]time.Duration) HealthCheck

// defaultNewHealthCheck is the default function to create a new instance for performing health checks.
var defaultNewHealthCheck NewHealthCheckFunc = func(seed *gardencorev1beta1.Seed, client client.Client, clock clock.Clock, namespace *string, seedIsGarden bool, loggingEnabled, valiEnabled bool, conditionThresholds map[gardencorev1beta1.ConditionType]time.Duration) HealthCheck {
	return NewHealth(seed, client, clock, namespace, seedIsGarden, loggingEnabled, valiEnabled, conditionThresholds)
}

// HealthCheck is an interface used to perform health checks.
type HealthCheck interface {
	Check(ctx context.Context, condition SeedConditions) []gardencorev1beta1.Condition
}
