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

// daemonSetHealthChecker contains all the information for the DaemonSet HealthCheck
type daemonSetHealthChecker struct {
	logger logr.Logger
	client client.Client
	name   string
}

// SeedDaemonSetHealthChecker is a healthCheck for DaemonSets in the Seed cluster
type SeedDaemonSetHealthChecker struct {
	daemonSetHealthChecker
}

// ShootDaemonSetHealthChecker is a healthCheck for DaemonSets in the Shoot cluster
type ShootDaemonSetHealthChecker struct {
	daemonSetHealthChecker
}

var (
	_ healthcheck.HealthCheck  = (*SeedDaemonSetHealthChecker)(nil)
	_ healthcheck.SourceClient = (*SeedDaemonSetHealthChecker)(nil)
	_ healthcheck.HealthCheck  = (*ShootDaemonSetHealthChecker)(nil)
	_ healthcheck.TargetClient = (*ShootDaemonSetHealthChecker)(nil)
)

// NewSeedDaemonSetHealthChecker is a healthCheck function to check DaemonSets in the Seed cluster
func NewSeedDaemonSetHealthChecker(name string) *SeedDaemonSetHealthChecker {
	return &SeedDaemonSetHealthChecker{
		daemonSetHealthChecker: daemonSetHealthChecker{
			name: name,
		},
	}
}

// NewShootDaemonSetHealthChecker is a healthCheck function to check DaemonSets in the Shoot cluster
func NewShootDaemonSetHealthChecker(name string) *ShootDaemonSetHealthChecker {
	return &ShootDaemonSetHealthChecker{
		daemonSetHealthChecker: daemonSetHealthChecker{
			name: name,
		},
	}
}

// InjectSourceClient injects the seed client
func (h *SeedDaemonSetHealthChecker) InjectSourceClient(sourceClient client.Client) {
	h.client = sourceClient
}

// InjectTargetClient injects the shoot client
func (h *ShootDaemonSetHealthChecker) InjectTargetClient(targetClient client.Client) {
	h.client = targetClient
}

// SetLoggerSuffix injects the logger
func (h *daemonSetHealthChecker) SetLoggerSuffix(provider, extension string) {
	h.logger = log.Log.WithName(fmt.Sprintf("%s-%s-healthcheck-daemonset", provider, extension))
}

// Check executes the health check
func (h *daemonSetHealthChecker) Check(ctx context.Context, request types.NamespacedName) (*healthcheck.SingleCheckResult, error) {
	daemonSet := &appsv1.DaemonSet{}

	if err := h.client.Get(ctx, client.ObjectKey{Namespace: request.Namespace, Name: h.name}, daemonSet); err != nil {
		if apierrors.IsNotFound(err) {
			return &healthcheck.SingleCheckResult{
				Status: gardencorev1beta1.ConditionFalse,
				Detail: fmt.Sprintf("DaemonSet %q in namespace %q not found", h.name, request.Namespace),
			}, nil
		}

		err := fmt.Errorf("failed to retrieve DaemonSet %q in namespace %q: %w", h.name, request.Namespace, err)
		h.logger.Error(err, "Health check failed")
		return nil, err
	}
	if isHealthy, err := DaemonSetIsHealthy(daemonSet); !isHealthy {
		h.logger.Error(err, "Health check failed")
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
