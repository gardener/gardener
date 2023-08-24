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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

// HealthCheck is an interface used to perform health checks.
type HealthCheck interface {
	Check(ctx context.Context, thresholdMapping map[gardencorev1beta1.ConditionType]time.Duration, threshold *metav1.Duration, conditions []gardencorev1beta1.Condition) []gardencorev1beta1.Condition
}

// ConstraintCheck is an interface used to perform constraint checks.
type ConstraintCheck interface {
	Check(ctx context.Context, constraints []gardencorev1beta1.Condition) []gardencorev1beta1.Condition
}

// GarbageCollector is an interface used to perform garbage collection.
type GarbageCollector interface {
	Collect(ctx context.Context)
}

// WebhookRemediator is an interface used to perform webhook remediation.
type WebhookRemediator interface {
	Remediate(ctx context.Context) error
}
