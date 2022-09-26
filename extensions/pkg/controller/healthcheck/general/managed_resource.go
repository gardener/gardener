// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package general

import (
	"context"
	"fmt"
	"regexp"

	"github.com/gardener/gardener/extensions/pkg/controller/healthcheck"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// ManagedResourceHealthChecker contains all the information for the ManagedResource HealthCheck
type ManagedResourceHealthChecker struct {
	logger              logr.Logger
	seedClient          client.Client
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
	if isHealthy, err := managedResourceIsHealthy(mcmDeployment); !isHealthy {
		healthChecker.logger.Error(err, "Health check failed")

		var (
			errorCodes                 []gardencorev1beta1.ErrorCode
			configurationProblemRegexp = regexp.MustCompile(`(?i)(error during apply of object .* is invalid:)`)
		)

		if configurationProblemRegexp.MatchString(err.Error()) {
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
