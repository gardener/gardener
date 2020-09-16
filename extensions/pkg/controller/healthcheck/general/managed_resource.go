// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package general

import (
	"context"
	"fmt"

	"github.com/gardener/gardener/extensions/pkg/controller/healthcheck"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"

	resourcesv1alpha1 "github.com/gardener/gardener-resource-manager/pkg/apis/resources/v1alpha1"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// ManagedResourceHealthChecker contains all the information for the ManagedResource HealthCheck
type ManagedResourceHealthChecker struct {
	logger              logr.Logger
	seedClient          client.Client
	shootClient         client.Client
	managedResourceName string
}

// CheckManagedResource is a healthCheck function to check ManagedResources
func CheckManagedResource(managedResourceName string) healthcheck.HealthCheck {
	return &ManagedResourceHealthChecker{
		managedResourceName: managedResourceName,
	}
}

// InjectSeedClient injects the seed client
func (healthChecker *ManagedResourceHealthChecker) InjectSeedClient(seedClient client.Client) {
	healthChecker.seedClient = seedClient
}

// InjectShootClient injects the shoot client
func (healthChecker *ManagedResourceHealthChecker) InjectShootClient(shootClient client.Client) {
	healthChecker.shootClient = shootClient
}

// SetLoggerSuffix injects the logger
func (healthChecker *ManagedResourceHealthChecker) SetLoggerSuffix(provider, extension string) {
	healthChecker.logger = log.Log.WithName(fmt.Sprintf("%s-%s-healthcheck-managed-resource", provider, extension))
}

// DeepCopy clones the healthCheck struct by making a copy and returning the pointer to that new copy
func (healthChecker *ManagedResourceHealthChecker) DeepCopy() healthcheck.HealthCheck {
	copy := *healthChecker
	return &copy
}

// Check executes the health check
func (healthChecker *ManagedResourceHealthChecker) Check(ctx context.Context, request types.NamespacedName) (*healthcheck.SingleCheckResult, error) {
	mcmDeployment := &resourcesv1alpha1.ManagedResource{}

	if err := healthChecker.seedClient.Get(ctx, client.ObjectKey{Namespace: request.Namespace, Name: healthChecker.managedResourceName}, mcmDeployment); err != nil {
		err := fmt.Errorf("check Managed Resource failed. Unable to retrieve managed resource '%s' in namespace '%s': %v", healthChecker.managedResourceName, request.Namespace, err)
		healthChecker.logger.Error(err, "Health check failed")
		return nil, err
	}
	if isHealthy, reason, err := managedResourceIsHealthy(mcmDeployment); !isHealthy {
		healthChecker.logger.Error(err, "Health check failed")
		return &healthcheck.SingleCheckResult{
			Status: gardencorev1beta1.ConditionFalse,
			Detail: err.Error(),
			Reason: *reason,
		}, nil
	}

	return &healthcheck.SingleCheckResult{
		Status: gardencorev1beta1.ConditionTrue,
	}, nil
}

func managedResourceIsHealthy(resource *resourcesv1alpha1.ManagedResource) (bool, *string, error) {
	if err := checkManagedResourceIsHealthy(resource); err != nil {
		reason := "ManagedResourceUnhealthy"
		err := fmt.Errorf("managed resource %s in namespace %s is unhealthy: %v", resource.Name, resource.Namespace, err)
		return false, &reason, err
	}
	return true, nil, nil
}

func checkManagedResourceIsHealthy(deployment *resourcesv1alpha1.ManagedResource) error {
	if deployment.Status.ObservedGeneration < deployment.Generation {
		return fmt.Errorf("observed generation outdated (%d/%d)", deployment.Status.ObservedGeneration, deployment.Generation)
	}

	for _, trueConditionType := range trueManagedResourceConditionTypes {
		conditionType := string(trueConditionType)
		condition := getManagedResourceCondition(deployment.Status.Conditions, trueConditionType)
		if condition == nil {
			return requiredConditionMissing(conditionType)
		}
		if err := checkConditionState(conditionType, string(corev1.ConditionTrue), string(condition.Status), condition.Reason, condition.Message); err != nil {
			return err
		}
	}
	return nil
}
