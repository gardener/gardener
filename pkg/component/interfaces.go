// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package component

import (
	"context"

	"github.com/gardener/gardener/pkg/apis/core/v1beta1"
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
	Restore(ctx context.Context, shootState *v1beta1.ShootState) error
	Migrate(ctx context.Context) error
}

// MigrateWaiter waits for the control-plane migration operations of a component to finish.
type MigrateWaiter interface {
	WaitMigrate(ctx context.Context) error
}

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

// IstioConfigInterface contains functions for retrieving data from the istio configuration.
type IstioConfigInterface interface {
	// ServiceName is the currently used name of the istio ingress service, which is responsible for the shoot cluster.
	ServiceName() string
	// Namespace is the currently used namespace of the istio ingress gateway, which is responsible for the shoot cluster.
	Namespace() string
	// LoadBalancerAnnotations contain the annotation to be used for the istio ingress service load balancer.
	LoadBalancerAnnotations() map[string]string
	// Labels contain the labels to be used for the istio ingress gateway entities.
	Labels() map[string]string
}

// CentralLoggingConfiguration is a function alias for returning configuration for the central logging.
type CentralLoggingConfiguration func() (CentralLoggingConfig, error)
