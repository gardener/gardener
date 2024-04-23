// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// HealthCheckConfig contains the health check controller configuration.
type HealthCheckConfig struct {
	// SyncPeriod is the duration how often the existing resources are reconciled (how
	// often the health check of Shoot clusters is performed (only if no operation is
	// already running on them).
	// defaults to 30 sec
	SyncPeriod metav1.Duration
	// ShootRESTOptions allow overwriting certain default settings of the shoot rest.Config.
	ShootRESTOptions *RESTOptions
}

// RESTOptions define a subset of optional parameters for a rest.Config.
// Default values when unset are those from https://github.com/kubernetes/client-go/blob/master/rest/config.go.
type RESTOptions struct {
	// QPS indicates the maximum QPS to the master from this client.
	QPS *float32
	// Maximum burst for throttle.
	Burst *int
	// The maximum length of time to wait before giving up on a server request. A value of zero means no timeout.
	Timeout *time.Duration
}
