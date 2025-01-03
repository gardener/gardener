// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package care

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	"github.com/gardener/gardener/pkg/gardenlet/operation/seed"
	"github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
)

// HealthCheck is an interface used to perform health checks.
type HealthCheck interface {
	Check(ctx context.Context, threshold *metav1.Duration, conditions ShootConditions) []gardencorev1beta1.Condition
}

// NewHealthCheckFunc is a function used to create a new instance for performing health checks.
type NewHealthCheckFunc func(
	logger logr.Logger,
	shoot *shoot.Shoot,
	seed *seed.Seed,
	seedClient kubernetes.Interface,
	gardenClient client.Client,
	shootClientInit ShootClientInit,
	clock clock.Clock,
	gardenletConfig *gardenletconfigv1alpha1.GardenletConfiguration,
	conditionThresholds map[gardencorev1beta1.ConditionType]time.Duration,
) HealthCheck

// defaultNewHealthCheck is the default function to create a new instance for performing health checks.
var defaultNewHealthCheck NewHealthCheckFunc = func(
	log logr.Logger,
	shoot *shoot.Shoot,
	seed *seed.Seed,
	seedClientSet kubernetes.Interface,
	gardenClient client.Client,
	shootClientInit ShootClientInit,
	clock clock.Clock,
	gardenletConfig *gardenletconfigv1alpha1.GardenletConfiguration,
	conditionThresholds map[gardencorev1beta1.ConditionType]time.Duration,
) HealthCheck {
	return NewHealth(
		log,
		shoot,
		seed,
		seedClientSet,
		gardenClient,
		shootClientInit,
		clock,
		gardenletConfig,
		conditionThresholds,
	)
}

// ConstraintCheck is an interface used to perform constraint checks.
type ConstraintCheck interface {
	Check(context.Context, ShootConstraints) []gardencorev1beta1.Condition
}

// NewConstraintCheckFunc is a function used to create a new instance for performing constraint checks.
type NewConstraintCheckFunc func(
	log logr.Logger,
	shoot *shoot.Shoot,
	seedClient client.Client,
	shootClientInit ShootClientInit,
	clock clock.Clock,
) ConstraintCheck

// defaultNewConstraintCheck is the default function to create a new instance for performing constraint checks.
var defaultNewConstraintCheck = func(
	log logr.Logger,
	shoot *shoot.Shoot,
	seedClient client.Client,
	shootClientInit ShootClientInit,
	clock clock.Clock,
) ConstraintCheck {
	return NewConstraint(
		log,
		shoot,
		seedClient,
		shootClientInit,
		clock,
	)
}

// GarbageCollector is an interface used to perform garbage collection.
type GarbageCollector interface {
	Collect(ctx context.Context)
}

// NewGarbageCollectorFunc is a function used to create a new instance to perform garbage collection.
type NewGarbageCollectorFunc func(op *operation.Operation, init ShootClientInit) GarbageCollector

// defaultNewGarbageCollector is the default function to create a new instance to perform garbage collection.
var defaultNewGarbageCollector = func(op *operation.Operation, init ShootClientInit) GarbageCollector {
	return NewGarbageCollection(op, init)
}

// WebhookRemediator is an interface used to perform webhook remediation.
type WebhookRemediator interface {
	Remediate(ctx context.Context) error
}

// NewWebhookRemediatorFunc is a function used to create a new instance to perform webhook remediation.
type NewWebhookRemediatorFunc func(op *operation.Operation, init ShootClientInit) WebhookRemediator

// defaultNewWebhookRemediator is the default function to create a new instance to perform webhook remediation.
var defaultNewWebhookRemediator = func(log logr.Logger, shoot *gardencorev1beta1.Shoot, init ShootClientInit) WebhookRemediator {
	return NewWebhookRemediation(log, shoot, init)
}

// NewOperationFunc is a function used to create a new `operation.Operation` instance.
type NewOperationFunc func(
	ctx context.Context,
	log logr.Logger,
	gardenClient client.Client,
	seedClientSet kubernetes.Interface,
	shootClientMap clientmap.ClientMap,
	config *gardenletconfigv1alpha1.GardenletConfiguration,
	gardenerInfo *gardencorev1beta1.Gardener,
	gardenClusterIdentity string,
	secrets map[string]*corev1.Secret,
	shoot *gardencorev1beta1.Shoot,
) (
	*operation.Operation,
	error,
)

var defaultNewOperationFunc = func(
	ctx context.Context,
	log logr.Logger,
	gardenClient client.Client,
	seedClientSet kubernetes.Interface,
	shootClientMap clientmap.ClientMap,
	config *gardenletconfigv1alpha1.GardenletConfiguration,
	gardenerInfo *gardencorev1beta1.Gardener,
	gardenClusterIdentity string,
	secrets map[string]*corev1.Secret,
	shoot *gardencorev1beta1.Shoot,
) (
	*operation.Operation,
	error,
) {
	return operation.
		NewBuilder().
		WithLogger(log).
		WithConfig(config).
		WithGardenerInfo(gardenerInfo).
		WithGardenClusterIdentity(gardenClusterIdentity).
		WithSecrets(secrets).
		WithGardenFrom(gardenClient, shoot.Namespace).
		WithSeedFrom(gardenClient, *shoot.Spec.SeedName).
		WithShootFromCluster(seedClientSet, shoot).
		Build(ctx, gardenClient, seedClientSet, shootClientMap)
}
