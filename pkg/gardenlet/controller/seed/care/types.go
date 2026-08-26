// SPDX-FileCopyrightText: Copyright Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package care

import (
	"context"

	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health/checker"
)

// NewHealthCheckFunc is a function used to create a new instance for performing health checks.
type NewHealthCheckFunc func(*gardencorev1beta1.Seed, client.Client, clock.Clock, *string, *checker.HealthChecker) HealthCheck

// defaultNewHealthCheck is the default function to create a new instance for performing health checks.
var defaultNewHealthCheck NewHealthCheckFunc = NewHealth

// HealthCheck is an interface used to perform health checks.
type HealthCheck interface {
	Check(ctx context.Context, conditions SeedConditions) []gardencorev1beta1.Condition
}

// ConstraintCheck is an interface used to perform constraint checks.
type ConstraintCheck interface {
	Check(ctx context.Context, constraints SeedConstraints) []gardencorev1beta1.Condition
}

// NewConstraintCheckFunc is a function used to create a new instance for performing constraint checks.
type NewConstraintCheckFunc func(client.Client, clock.Clock, *string) ConstraintCheck

// defaultNewConstraintCheck is the default function to create a new instance for performing constraint checks.
var defaultNewConstraintCheck NewConstraintCheckFunc = func(seedClient client.Client, clock clock.Clock, namespace *string) ConstraintCheck {
	return NewConstraint(seedClient, clock, namespace)
}

// NewConstraintCheck is used to create a new Constraint check instance.
var NewConstraintCheck = defaultNewConstraintCheck
