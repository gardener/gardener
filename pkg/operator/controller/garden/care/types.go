// SPDX-FileCopyrightText: Copyright Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package care

import (
	"context"
	"time"

	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health/checker"
)

// NewHealthCheckFunc is a function used to create a new instance for performing health checks.
type NewHealthCheckFunc func(*operatorv1alpha1.Garden, client.Client, kubernetes.Interface, clock.Clock, map[gardencorev1beta1.ConditionType]time.Duration, string, *checker.HealthChecker) HealthCheck

// defaultNewHealthCheck is the default function to create a new instance for performing health checks.
var defaultNewHealthCheck NewHealthCheckFunc = NewHealth

// HealthCheck is an interface used to perform health checks.
type HealthCheck interface {
	Check(ctx context.Context, conditions GardenConditions) []gardencorev1beta1.Condition
}

// ConstraintCheck is an interface used to perform constraint checks.
type ConstraintCheck interface {
	Check(ctx context.Context, constraints GardenConstraints) []gardencorev1beta1.Condition
}

// NewConstraintCheckFunc is a function used to create a new instance for performing constraint checks.
type NewConstraintCheckFunc func(client.Client, clock.Clock, string) ConstraintCheck

// defaultNewConstraintCheck is the default function to create a new instance for performing constraint checks.
var defaultNewConstraintCheck NewConstraintCheckFunc = func(runtimeClient client.Client, clock clock.Clock, gardenNamespace string) ConstraintCheck {
	return NewConstraint(runtimeClient, clock, gardenNamespace)
}

// NewConstraintCheck is used to create a new Constraint check instance.
var NewConstraintCheck = defaultNewConstraintCheck
