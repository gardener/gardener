// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package general

import (
	"context"
	"fmt"
	"regexp"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/gardener/gardener/extensions/pkg/controller/healthcheck"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
)

// ManagedResourceHealthChecker contains all the information for the ManagedResource HealthCheck
type ManagedResourceHealthChecker struct {
	logger              logr.Logger
	client              client.Client
	managedResourceName string
}

var (
	_ healthcheck.HealthCheck  = (*ManagedResourceHealthChecker)(nil)
	_ healthcheck.SourceClient = (*ManagedResourceHealthChecker)(nil)
)

// CheckManagedResource is a healthCheck function to check ManagedResources in the Seed/Source cluster
func CheckManagedResource(managedResourceName string) *ManagedResourceHealthChecker {
	return &ManagedResourceHealthChecker{
		managedResourceName: managedResourceName,
	}
}

// InjectSourceClient injects the seed/source client
func (healthChecker *ManagedResourceHealthChecker) InjectSourceClient(client client.Client) {
	healthChecker.client = client
}

// SetLoggerSuffix injects the logger
func (healthChecker *ManagedResourceHealthChecker) SetLoggerSuffix(provider, extension string) {
	healthChecker.logger = log.Log.WithName(fmt.Sprintf("%s-%s-healthcheck-managed-resource", provider, extension))
}

// configurationProblemRegex is used to check if a not healthy managed resource has a configuration problem.
var configurationProblemRegex = regexp.MustCompile(`(?i)(error during apply of object .* is invalid:)`)

// Check executes the health check
func (healthChecker *ManagedResourceHealthChecker) Check(ctx context.Context, request types.NamespacedName) (*healthcheck.SingleCheckResult, error) {
	managedResource := &resourcesv1alpha1.ManagedResource{}

	if err := healthChecker.client.Get(ctx, client.ObjectKey{Namespace: request.Namespace, Name: healthChecker.managedResourceName}, managedResource); err != nil {
		if apierrors.IsNotFound(err) {
			return &healthcheck.SingleCheckResult{
				Status: gardencorev1beta1.ConditionFalse,
				Detail: fmt.Sprintf("Managed Resource %q in namespace %q not found", healthChecker.managedResourceName, request.Namespace),
			}, nil
		}

		err := fmt.Errorf("check Managed Resource failed. Unable to retrieve managed resource %q in namespace %q: %w", healthChecker.managedResourceName, request.Namespace, err)
		healthChecker.logger.Error(err, "Health check failed")
		return nil, err
	}
	if isHealthy, err := managedResourceIsHealthy(managedResource); !isHealthy {
		healthChecker.logger.Error(err, "Health check failed")

		var errorCodes []gardencorev1beta1.ErrorCode
		if configurationProblemRegex.MatchString(err.Error()) {
			errorCodes = append(errorCodes, gardencorev1beta1.ErrorConfigurationProblem)
		}

		return &healthcheck.SingleCheckResult{
			Status: gardencorev1beta1.ConditionFalse,
			Detail: err.Error(),
			Codes:  errorCodes,
		}, nil
	}

	return &healthcheck.SingleCheckResult{
		Status: gardencorev1beta1.ConditionTrue,
	}, nil
}

func managedResourceIsHealthy(managedResource *resourcesv1alpha1.ManagedResource) (bool, error) {
	if err := health.CheckManagedResource(managedResource); err != nil {
		err := fmt.Errorf("managed resource %q in namespace %q is unhealthy: %w", managedResource.Name, managedResource.Namespace, err)
		return false, err
	}
	return true, nil
}
