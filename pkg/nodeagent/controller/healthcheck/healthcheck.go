// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package healthcheck

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
)

const maxFailureDuration = 60 * time.Second

// HealthChecker can be implemented to run a healthcheck against a node component
// and repair if possible, otherwise fail and report bach.
type HealthChecker interface {
	// Name returns the name of the healthchecker.
	Name() string
	// Check executes the healthcheck.
	Check(ctx context.Context, node *corev1.Node) error
}
