// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
