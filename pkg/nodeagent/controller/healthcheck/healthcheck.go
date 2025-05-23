// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package healthcheck

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
)

const maxFailureDuration = time.Minute

// HealthChecker can be implemented to run a health check against a node component
// and fix it if possible.
type HealthChecker interface {
	// Name returns the name of the healthchecker.
	Name() string
	// Check executes the health check.
	Check(ctx context.Context, node *corev1.Node) error
}
