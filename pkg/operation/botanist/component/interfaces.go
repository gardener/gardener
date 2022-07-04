// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package component

import (
	"context"

	"github.com/gardener/gardener/pkg/apis/core/v1alpha1"
)

// Deployer is used to control the life-cycle of a component.
type Deployer interface {
	// Deploy a component.
	Deploy(ctx context.Context) error
	// Destroy already deployed component.
	Destroy(ctx context.Context) error
}

// Waiter waits for life-cycle operations of a component to finish.
type Waiter interface {
	// Wait for deployment to finish and component to report ready.
	Wait(ctx context.Context) error
	// WaitCleanup for destruction to finish and component to be fully removed.
	WaitCleanup(ctx context.Context) error
}

// Migrator is used to control the control-plane migration operations of a component.
type Migrator interface {
	Restore(ctx context.Context, shootState *v1alpha1.ShootState) error
	Migrate(ctx context.Context) error
}

// MigrateWaiter waits for the control-plane migration operations of a component to finish.
type MigrateWaiter interface {
	WaitMigrate(ctx context.Context) error
}

// MonitoringComponent exposes configuration for Prometheus as well as the AlertManager.
type MonitoringComponent interface {
	// ScrapeConfigs returns the scrape configurations for Prometheus.
	ScrapeConfigs() ([]string, error)
	// AlertingRules returns the alerting rules configs for AlertManager (mapping file name to rule config).
	AlertingRules() (map[string]string, error)
}

type (
	// AggregateMonitoringConfiguration is a function alias for returning configuration for the aggregate monitoring.
	AggregateMonitoringConfiguration func() (AggregateMonitoringConfig, error)
	// CentralMonitoringConfiguration is a function alias for returning configuration for the central monitoring.
	CentralMonitoringConfiguration func() (CentralMonitoringConfig, error)
	// CentralLoggingConfiguration is a function alias for returning configuration for the central logging.
	CentralLoggingConfiguration func() (CentralLoggingConfig, error)
)

// DeployWaiter controls and waits for life-cycle operations of a component.
type DeployWaiter interface {
	Deployer
	Waiter
}

// DeployMigrateWaiter controls and waits for the life-cycle and control-plane migration operations of a component.
type DeployMigrateWaiter interface {
	Deployer
	Migrator
	MigrateWaiter
	Waiter
}
