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

// DeploymentHealthChecker contains all the information for the Deployment HealthCheck
type DeploymentHealthChecker struct {
	logger      logr.Logger
	seedClient  client.Client
	shootClient client.Client
	name        string
	checkType   DeploymentCheckType
}

// DeploymentCheckType in which cluster the check will be executed
type DeploymentCheckType string

const (
	deploymentCheckTypeSeed  DeploymentCheckType = "Seed"
	deploymentCheckTypeShoot DeploymentCheckType = "Shoot"
)

// NewSeedDeploymentHealthChecker is a healthCheck function to check Deployments in the Seed cluster
func NewSeedDeploymentHealthChecker(deploymentName string) healthcheck.HealthCheck {
	return &DeploymentHealthChecker{
		name:      deploymentName,
		checkType: deploymentCheckTypeSeed,
	}
}

// NewShootDeploymentHealthChecker is a healthCheck function to check Deployments in the Shoot cluster
func NewShootDeploymentHealthChecker(deploymentName string) healthcheck.HealthCheck {
	return &DeploymentHealthChecker{
		name:      deploymentName,
		checkType: deploymentCheckTypeShoot,
	}
}

// InjectSeedClient injects the seed client
func (healthChecker *DeploymentHealthChecker) InjectSeedClient(seedClient client.Client) {
	healthChecker.seedClient = seedClient
}

// InjectShootClient injects the shoot client
func (healthChecker *DeploymentHealthChecker) InjectShootClient(shootClient client.Client) {
	healthChecker.shootClient = shootClient
}

// SetLoggerSuffix injects the logger
func (healthChecker *DeploymentHealthChecker) SetLoggerSuffix(provider, extension string) {
	healthChecker.logger = log.Log.WithName(fmt.Sprintf("%s-%s-healthcheck-deployment", provider, extension))
}

// DeepCopy clones the healthCheck struct by making a copy and returning the pointer to that new copy
// Actually, it does not perform a *deep* copy.
func (healthChecker *DeploymentHealthChecker) DeepCopy() healthcheck.HealthCheck {
	shallowCopy := *healthChecker
	return &shallowCopy
}

// Check executes the health check
func (healthChecker *DeploymentHealthChecker) Check(ctx context.Context, request types.NamespacedName) (*healthcheck.SingleCheckResult, error) {
	deployment := &appsv1.Deployment{}

	var err error
	if healthChecker.checkType == deploymentCheckTypeSeed {
		err = healthChecker.seedClient.Get(ctx, client.ObjectKey{Namespace: request.Namespace, Name: healthChecker.name}, deployment)
	} else {
		err = healthChecker.shootClient.Get(ctx, client.ObjectKey{Namespace: request.Namespace, Name: healthChecker.name}, deployment)
	}
	if err != nil {
		if apierrors.IsNotFound(err) {
			return &healthcheck.SingleCheckResult{
				Status: gardencorev1beta1.ConditionFalse,
				Detail: fmt.Sprintf("deployment %q in namespace %q not found", healthChecker.name, request.Namespace),
			}, nil
		}

		err := fmt.Errorf("failed to retrieve deployment %q in namespace %q: %w", healthChecker.name, request.Namespace, err)
		healthChecker.logger.Error(err, "Health check failed")
		return nil, err
	}

	if isHealthy, err := deploymentIsHealthy(deployment); !isHealthy {
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

func deploymentIsHealthy(deployment *appsv1.Deployment) (bool, error) {
	if err := health.CheckDeployment(deployment); err != nil {
		err := fmt.Errorf("deployment %q in namespace %q is unhealthy: %w", deployment.Name, deployment.Namespace, err)
		return false, err
	}
	return true, nil
}
