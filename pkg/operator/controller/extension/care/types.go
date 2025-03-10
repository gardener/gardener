// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
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
)

// NewHealthCheckFunc is a function used to create a new instance for performing health checks.
type NewHealthCheckFunc func(*operatorv1alpha1.Extension, client.Client, kubernetes.Interface, clock.Clock, map[gardencorev1beta1.ConditionType]time.Duration, string) HealthCheck

// defaultNewHealthCheck is the default function to create a new instance for performing health checks.
var defaultNewHealthCheck NewHealthCheckFunc = func(extension *operatorv1alpha1.Extension, runtimeClient client.Client, gardenClientSet kubernetes.Interface, clock clock.Clock, conditionThresholds map[gardencorev1beta1.ConditionType]time.Duration, gardenNamespace string) HealthCheck {
	return NewHealth(extension, runtimeClient, gardenClientSet, clock, conditionThresholds, gardenNamespace)
}

// HealthCheck is an interface used to perform health checks.
type HealthCheck interface {
	Check(ctx context.Context, conditions ExtensionConditions) []gardencorev1beta1.Condition
}
