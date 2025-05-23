// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package general

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/gardener/gardener/extensions/pkg/controller/healthcheck"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
)

// DaemonSetHealthChecker contains all the information for the DaemonSet HealthCheck
type DaemonSetHealthChecker struct {
	logger      logr.Logger
	seedClient  client.Client
	shootClient client.Client
	name        string
	checkType   DaemonSetCheckType
}

// DaemonSetCheckType in which cluster the check will be executed
type DaemonSetCheckType string

const (
	daemonSetCheckTypeSeed  DaemonSetCheckType = "Seed"
	daemonSetCheckTypeShoot DaemonSetCheckType = "Shoot"
)

// NewSeedDaemonSetHealthChecker is a healthCheck function to check DaemonSets
func NewSeedDaemonSetHealthChecker(name string) healthcheck.HealthCheck {
	return &DaemonSetHealthChecker{
		name:      name,
		checkType: daemonSetCheckTypeSeed,
	}
}

// NewShootDaemonSetHealthChecker is a healthCheck function to check DaemonSets
func NewShootDaemonSetHealthChecker(name string) healthcheck.HealthCheck {
	return &DaemonSetHealthChecker{
		name:      name,
		checkType: daemonSetCheckTypeShoot,
	}
}

// InjectSeedClient injects the seed client
func (healthChecker *DaemonSetHealthChecker) InjectSeedClient(seedClient client.Client) {
	healthChecker.seedClient = seedClient
}

// InjectShootClient injects the shoot client
func (healthChecker *DaemonSetHealthChecker) InjectShootClient(shootClient client.Client) {
	healthChecker.shootClient = shootClient
}

// SetLoggerSuffix injects the logger
func (healthChecker *DaemonSetHealthChecker) SetLoggerSuffix(provider, extension string) {
	healthChecker.logger = log.Log.WithName(fmt.Sprintf("%s-%s-healthcheck-deployment", provider, extension))
}

// DeepCopy clones the healthCheck struct by making a copy and returning the pointer to that new copy
// Actually, it does not perform a *deep* copy.
func (healthChecker *DaemonSetHealthChecker) DeepCopy() healthcheck.HealthCheck {
	shallowCopy := *healthChecker
	return &shallowCopy
}

// Check executes the health check
func (healthChecker *DaemonSetHealthChecker) Check(ctx context.Context, request types.NamespacedName) (*healthcheck.SingleCheckResult, error) {
	daemonSet := &appsv1.DaemonSet{}
	var err error
	if healthChecker.checkType == daemonSetCheckTypeSeed {
		err = healthChecker.seedClient.Get(ctx, client.ObjectKey{Namespace: request.Namespace, Name: healthChecker.name}, daemonSet)
	} else {
		err = healthChecker.shootClient.Get(ctx, client.ObjectKey{Namespace: request.Namespace, Name: healthChecker.name}, daemonSet)
	}
	if err != nil {
		if apierrors.IsNotFound(err) {
			return &healthcheck.SingleCheckResult{
				Status: gardencorev1beta1.ConditionFalse,
				Detail: fmt.Sprintf("DaemonSet %q in namespace %q not found", healthChecker.name, request.Namespace),
			}, nil
		}

		err := fmt.Errorf("failed to retrieve DaemonSet %q in namespace %q: %w", healthChecker.name, request.Namespace, err)
		healthChecker.logger.Error(err, "Health check failed")
		return nil, err
	}
	if isHealthy, err := DaemonSetIsHealthy(daemonSet); !isHealthy {
		healthChecker.logger.Error(err, "Health check failed")
		return &healthcheck.SingleCheckResult{
			Status: gardencorev1beta1.ConditionFalse,
			Detail: err.Error(),
		}, nil
	}

	return &healthcheck.SingleCheckResult{
		Status: gardencorev1beta1.ConditionTrue,
	}, nil
}

// DaemonSetIsHealthy takes a daemon set resource and returns
// if it is healthy or not or an error
func DaemonSetIsHealthy(daemonSet *appsv1.DaemonSet) (bool, error) {
	if err := health.CheckDaemonSet(daemonSet); err != nil {
		err := fmt.Errorf("daemonSet %q in namespace %q is unhealthy: %w", daemonSet.Name, daemonSet.Namespace, err)
		return false, err
	}
	return true, nil
}
