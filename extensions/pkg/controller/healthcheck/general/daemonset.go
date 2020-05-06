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

	"github.com/gardener/gardener/extensions/pkg/controller/healthcheck"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"

	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
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
	DaemonSetCheckTypeSeed  DaemonSetCheckType = "Seed"
	DaemonSetCheckTypeShoot DaemonSetCheckType = "Shoot"
)

// NewSeedDaemonSetHealthChecker is a healthCheck function to check DaemonSets
func NewSeedDaemonSetHealthChecker(name string) healthcheck.HealthCheck {
	return &DaemonSetHealthChecker{
		name:      name,
		checkType: DaemonSetCheckTypeSeed,
	}
}

// NewShootDaemonSetHealthChecker is a healthCheck function to check DaemonSets
func NewShootDaemonSetHealthChecker(name string) healthcheck.HealthCheck {
	return &DaemonSetHealthChecker{
		name:      name,
		checkType: DaemonSetCheckTypeShoot,
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
func (healthChecker *DaemonSetHealthChecker) DeepCopy() healthcheck.HealthCheck {
	copy := *healthChecker
	return &copy
}

// Check executes the health check
func (healthChecker *DaemonSetHealthChecker) Check(ctx context.Context, request types.NamespacedName) (*healthcheck.SingleCheckResult, error) {
	daemonSet := &appsv1.DaemonSet{}
	var err error
	if healthChecker.checkType == DaemonSetCheckTypeSeed {
		err = healthChecker.seedClient.Get(ctx, client.ObjectKey{Namespace: request.Namespace, Name: healthChecker.name}, daemonSet)
	} else {
		err = healthChecker.shootClient.Get(ctx, client.ObjectKey{Namespace: request.Namespace, Name: healthChecker.name}, daemonSet)
	}
	if err != nil {
		err := fmt.Errorf("failed to retrieve DaemonSet '%s' in namespace '%s': %v", healthChecker.name, request.Namespace, err)
		healthChecker.logger.Error(err, "Health check failed")
		return nil, err
	}
	if isHealthy, reason, err := DaemonSetIsHealthy(daemonSet); !isHealthy {
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

func DaemonSetIsHealthy(daemonSet *appsv1.DaemonSet) (bool, *string, error) {
	if err := health.CheckDaemonSet(daemonSet); err != nil {
		reason := "DaemonSetUnhealthy"
		err := fmt.Errorf("daemonSet %s in namespace %s is unhealthy: %v", daemonSet.Name, daemonSet.Namespace, err)
		return false, &reason, err
	}
	return true, nil, nil
}
