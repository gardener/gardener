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

// deploymentHealthChecker contains all the information for the Deployment HealthCheck
type deploymentHealthChecker struct {
	logger logr.Logger
	client client.Client
	name   string
}

// SeedDeploymentHealthChecker is a healthCheck for Deployments in the Seed cluster
type SeedDeploymentHealthChecker struct {
	deploymentHealthChecker
}

// ShootDeploymentHealthChecker is a healthCheck for Deployments in the Shoot cluster
type ShootDeploymentHealthChecker struct {
	deploymentHealthChecker
}

var (
	_ healthcheck.HealthCheck  = (*SeedDeploymentHealthChecker)(nil)
	_ healthcheck.SourceClient = (*SeedDeploymentHealthChecker)(nil)
	_ healthcheck.HealthCheck  = (*ShootDeploymentHealthChecker)(nil)
	_ healthcheck.TargetClient = (*ShootDeploymentHealthChecker)(nil)
)

// NewSeedDeploymentHealthChecker is a healthCheck function to check Deployments in the Seed cluster
func NewSeedDeploymentHealthChecker(deploymentName string) *SeedDeploymentHealthChecker {
	return &SeedDeploymentHealthChecker{
		deploymentHealthChecker: deploymentHealthChecker{
			name: deploymentName,
		},
	}
}

// NewShootDeploymentHealthChecker is a healthCheck function to check Deployments in the Shoot cluster
func NewShootDeploymentHealthChecker(deploymentName string) *ShootDeploymentHealthChecker {
	return &ShootDeploymentHealthChecker{
		deploymentHealthChecker: deploymentHealthChecker{
			name: deploymentName,
		},
	}
}

// InjectSourceClient injects the seed client
func (h *SeedDeploymentHealthChecker) InjectSourceClient(sourceClient client.Client) {
	h.client = sourceClient
}

// InjectTargetClient injects the shoot client
func (h *ShootDeploymentHealthChecker) InjectTargetClient(targetClient client.Client) {
	h.client = targetClient
}

// SetLoggerSuffix injects the logger
func (h *deploymentHealthChecker) SetLoggerSuffix(provider, extension string) {
	h.logger = log.Log.WithName(fmt.Sprintf("%s-%s-healthcheck-deployment", provider, extension))
}

// Check executes the health check
func (h *deploymentHealthChecker) Check(ctx context.Context, request types.NamespacedName) (*healthcheck.SingleCheckResult, error) {
	deployment := &appsv1.Deployment{}

	if err := h.client.Get(ctx, client.ObjectKey{Namespace: request.Namespace, Name: h.name}, deployment); err != nil {
		if apierrors.IsNotFound(err) {
			return &healthcheck.SingleCheckResult{
				Status: gardencorev1beta1.ConditionFalse,
				Detail: fmt.Sprintf("deployment %q in namespace %q not found", h.name, request.Namespace),
			}, nil
		}

		err := fmt.Errorf("failed to retrieve deployment %q in namespace %q: %w", h.name, request.Namespace, err)
		h.logger.Error(err, "Health check failed")
		return nil, err
	}

	if isHealthy, err := deploymentIsHealthy(deployment); !isHealthy {
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

func deploymentIsHealthy(deployment *appsv1.Deployment) (bool, error) {
	if err := health.CheckDeployment(deployment); err != nil {
		err := fmt.Errorf("deployment %q in namespace %q is unhealthy: %w", deployment.Name, deployment.Namespace, err)
		return false, err
	}
	return true, nil
}
