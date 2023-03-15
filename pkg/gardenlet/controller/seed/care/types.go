// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	"github.com/gardener/gardener/pkg/operation/care"
	seedpkg "github.com/gardener/gardener/pkg/operation/seed"
)

var defaultNewSeedObjectFunc = func(ctx context.Context, seed *gardencorev1beta1.Seed) (*seedpkg.Seed, error) {
	return seedpkg.NewBuilder().WithSeedObject(seed).Build(ctx)
}

// NewHealthCheckFunc is a function used to create a new instance for performing health checks.
type NewHealthCheckFunc func(*gardencorev1beta1.Seed, client.Client, clock.Clock, *string, bool) HealthCheck

// defaultNewHealthCheck is the default function to create a new instance for performing health checks.
var defaultNewHealthCheck NewHealthCheckFunc = func(seed *gardencorev1beta1.Seed, client client.Client, gardenletConfig config.GardenletConfiguration, clock clock.Clock, namespace *string, seedIsGarden bool) HealthCheck {
	return care.NewHealthForSeed(seed, client, gardenletConfig, clock, namespace, seedIsGarden)
}

// HealthCheck is an interface used to perform health checks.
type HealthCheck interface {
	CheckSeed(ctx context.Context, condition []gardencorev1beta1.Condition, thresholdMappings map[gardencorev1beta1.ConditionType]time.Duration) []gardencorev1beta1.Condition
}
