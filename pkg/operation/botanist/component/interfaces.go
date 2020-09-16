// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

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
	// ScrapeConfigs returns the scrape configurationsv for Prometheus.
	ScrapeConfigs() ([]string, error)
	// AlertingRules returns the alerting rules configs for AlertManager (mapping file name to rule config).
	AlertingRules() (map[string]string, error)
}

// LoggingConfiguration is a function alias for returning logging parsers and filters.
type LoggingConfiguration func() (string, string, error)

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
