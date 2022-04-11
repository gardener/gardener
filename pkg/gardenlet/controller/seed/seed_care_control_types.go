// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
package seed

import (
	"context"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation/care"
	seedpkg "github.com/gardener/gardener/pkg/operation/seed"
	"github.com/go-logr/logr"
)

var defaultNewSeedObjectFunc = func(ctx context.Context, seed *gardencorev1beta1.Seed) (*seedpkg.Seed, error) {
	seedObj, err := seedpkg.NewBuilder().WithSeedObject(seed).Build(ctx)
	if err != nil {
		return nil, err
	}
	return seedObj, nil
}

// NewHealthCheckFunc is a function used to create a new instance for performing health checks.
type NewHealthCheckFunc func(seed *gardencorev1beta1.Seed, client kubernetes.Interface, l logr.Logger) HealthCheck

// defaultNewHealthCheck is the default function to create a new instance for performing health checks.
var defaultNewHealthCheck NewHealthCheckFunc = func(seed *gardencorev1beta1.Seed, client kubernetes.Interface, l logr.Logger) HealthCheck {
	return care.NewHealthForSeed(seed, client, l)
}

// HealthCheck is an interface used to perform health checks.
type HealthCheck interface {
	CheckSeed(ctx context.Context, seed *seedpkg.Seed, condition []gardencorev1beta1.Condition, thresholdMappings map[gardencorev1beta1.ConditionType]time.Duration) []gardencorev1beta1.Condition
}
