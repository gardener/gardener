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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// StatefulSetHealthChecker contains all the information for the StatefulSet HealthCheck
type StatefulSetHealthChecker struct {
	logger      logr.Logger
	seedClient  client.Client
	shootClient client.Client
	name        string
	checkType   StatefulSetCheckType
}

// StatefulSetCheckType in which cluster the check will be executed
type StatefulSetCheckType string

const (
	statefulSetCheckTypeSeed  StatefulSetCheckType = "Seed"
	statefulSetCheckTypeShoot StatefulSetCheckType = "Shoot"
)

// NewSeedStatefulSetChecker is a healthCheck function to check StatefulSets
func NewSeedStatefulSetChecker(name string) healthcheck.HealthCheck {
	return &StatefulSetHealthChecker{
		name:      name,
		checkType: statefulSetCheckTypeSeed,
	}
}

// NewShootStatefulSetChecker is a healthCheck function to check StatefulSets
func NewShootStatefulSetChecker(name string) healthcheck.HealthCheck {
	return &StatefulSetHealthChecker{
		name:      name,
		checkType: statefulSetCheckTypeShoot,
	}
}

// InjectSeedClient injects the seed client
func (healthChecker *StatefulSetHealthChecker) InjectSeedClient(seedClient client.Client) {
	healthChecker.seedClient = seedClient
}

// InjectShootClient injects the shoot client
func (healthChecker *StatefulSetHealthChecker) InjectShootClient(shootClient client.Client) {
	healthChecker.shootClient = shootClient
}

// SetLoggerSuffix injects the logger
func (healthChecker *StatefulSetHealthChecker) SetLoggerSuffix(provider, extension string) {
	healthChecker.logger = log.Log.WithName(fmt.Sprintf("%s-%s-healthcheck-deployment", provider, extension))
}

// DeepCopy clones the healthCheck struct by making a copy and returning the pointer to that new copy
func (healthChecker *StatefulSetHealthChecker) DeepCopy() healthcheck.HealthCheck {
	copy := *healthChecker
	return &copy
}

// Check executes the health check
func (healthChecker *StatefulSetHealthChecker) Check(ctx context.Context, request types.NamespacedName) (*healthcheck.SingleCheckResult, error) {
	statefulSet := &appsv1.StatefulSet{}

	var err error
	if healthChecker.checkType == statefulSetCheckTypeSeed {
		err = healthChecker.seedClient.Get(ctx, client.ObjectKey{Namespace: request.Namespace, Name: healthChecker.name}, statefulSet)
	} else {
		err = healthChecker.shootClient.Get(ctx, client.ObjectKey{Namespace: request.Namespace, Name: healthChecker.name}, statefulSet)
	}
	if err != nil {
		if apierrors.IsNotFound(err) {
			return &healthcheck.SingleCheckResult{
				Status: gardencorev1beta1.ConditionFalse,
				Detail: fmt.Sprintf("StatefulSet %q in namespace %q not found", healthChecker.name, request.Namespace),
			}, nil
		}
		err := fmt.Errorf("failed to retrieve StatefulSet %q in namespace %q: %w", healthChecker.name, request.Namespace, err)
		healthChecker.logger.Error(err, "Health check failed")
		return nil, err
	}
	if isHealthy, err := statefulSetIsHealthy(statefulSet); !isHealthy {
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

func statefulSetIsHealthy(statefulSet *appsv1.StatefulSet) (bool, error) {
	if err := health.CheckStatefulSet(statefulSet); err != nil {
		err := fmt.Errorf("statefulSet %q in namespace %q is unhealthy: %w", statefulSet.Name, statefulSet.Namespace, err)
		return false, err
	}
	return true, nil
}
