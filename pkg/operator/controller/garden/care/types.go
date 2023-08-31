// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
type NewHealthCheckFunc func(*operatorv1alpha1.Garden, client.Client, kubernetes.Interface, clock.Clock, map[gardencorev1beta1.ConditionType]time.Duration, string) HealthCheck

// defaultNewHealthCheck is the default function to create a new instance for performing health checks.
var defaultNewHealthCheck NewHealthCheckFunc = func(garden *operatorv1alpha1.Garden, runtimeClient client.Client, gardenClientSet kubernetes.Interface, clock clock.Clock, conditionThresholds map[gardencorev1beta1.ConditionType]time.Duration, gardenNamespace string) HealthCheck {
	return NewHealth(garden, runtimeClient, gardenClientSet, clock, conditionThresholds, gardenNamespace)
}

// HealthCheck is an interface used to perform health checks.
type HealthCheck interface {
	Check(ctx context.Context, conditions GardenConditions) []gardencorev1beta1.Condition
}
