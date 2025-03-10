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
	healthchecker "github.com/gardener/gardener/pkg/utils/kubernetes/health/checker"
)

// health contains information needed to execute health checks for the extension.
type health struct {
	extension           *operatorv1alpha1.Extension
	gardenNamespace     string
	runtimeClient       client.Client
	gardenClientSet     kubernetes.Interface
	clock               clock.Clock
	conditionThresholds map[gardencorev1beta1.ConditionType]time.Duration
	healthChecker       *healthchecker.HealthChecker
}

// NewHealth creates a new Health instance with the given parameters.
func NewHealth(
	extension *operatorv1alpha1.Extension,
	runtimeClient client.Client,
	gardenClientSet kubernetes.Interface,
	clock clock.Clock,
	conditionThresholds map[gardencorev1beta1.ConditionType]time.Duration,
	gardenNamespace string,
) HealthCheck {
	return &health{
		extension:           extension,
		gardenNamespace:     gardenNamespace,
		runtimeClient:       runtimeClient,
		gardenClientSet:     gardenClientSet,
		clock:               clock,
		conditionThresholds: conditionThresholds,
		healthChecker:       healthchecker.NewHealthChecker(runtimeClient, clock, conditionThresholds, nil),
	}
}

// Check conducts the health checks on all the given conditions.
func (h *health) Check(_ context.Context, conditions ExtensionConditions) []gardencorev1beta1.Condition {
	return conditions.ConvertToSlice()
}

// ExtensionConditions contains all conditions of the extension status subresource.
type ExtensionConditions struct{}

// ConvertToSlice returns the extension conditions as a slice.
func (e ExtensionConditions) ConvertToSlice() []gardencorev1beta1.Condition {
	return []gardencorev1beta1.Condition{}
}

// ConditionTypes returns all extension condition types.
func (e ExtensionConditions) ConditionTypes() []gardencorev1beta1.ConditionType {
	return []gardencorev1beta1.ConditionType{}
}

// NewExtensionConditions returns a new instance of ExtensionConditions.
// All conditions are retrieved from the given 'status' or newly initialized.
func NewExtensionConditions(_ clock.Clock, _ *operatorv1alpha1.Extension) ExtensionConditions {
	return ExtensionConditions{}
}
