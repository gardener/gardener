// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package config

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

type HealthCheckConfig struct {
	// SyncPeriod is the duration how often the existing resources are reconciled (how
	// often the health check of Shoot clusters is performed (only if no operation is
	// already running on them).
	// defaults to 30 sec
	SyncPeriod metav1.Duration
}
