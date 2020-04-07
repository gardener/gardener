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

package worker

import (
	"context"
	"fmt"

	"github.com/gardener/gardener/extensions/pkg/controller/healthcheck"

	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// DefaultHealthChecker all the information for the Worker HealthCheck
// This check assumes that the MachineControllerManager (https://github.com/gardener/machine-controller-manager) has been deployed by the Worker extension controller
type DefaultHealthChecker struct {
	logger logr.Logger
	// Needs to be set by actuator before calling the Check function
	seedClient client.Client
	// make sure shoot client is instantiated
	shootClient client.Client
}

// NewSufficientNodesChecker is a health check function which checks if there is a sufficient amount of nodes registered in the cluster.
// Checks if all machines created by the machine deployment joinend the cluster
func NewSufficientNodesChecker() healthcheck.HealthCheck {
	return &DefaultHealthChecker{}
}

// InjectSeedClient injects the seed client
func (healthChecker *DefaultHealthChecker) InjectSeedClient(seedClient client.Client) {
	healthChecker.seedClient = seedClient
}

// InjectShootClient injects the shoot client
func (healthChecker *DefaultHealthChecker) InjectShootClient(shootClient client.Client) {
	healthChecker.shootClient = shootClient
}

// SetLoggerSuffix injects the logger
func (healthChecker *DefaultHealthChecker) SetLoggerSuffix(provider, extension string) {
	healthChecker.logger = log.Log.WithName(fmt.Sprintf("%s-%s-healthcheck-sufficient-nodes", provider, extension))
}

// DeepCopy clones the healthCheck struct by making a copy and returning the pointer to that new copy
func (healthChecker *DefaultHealthChecker) DeepCopy() healthcheck.HealthCheck {
	copy := *healthChecker
	return &copy
}

// Check executes the health check
func (healthChecker *DefaultHealthChecker) Check(ctx context.Context, request types.NamespacedName) (*healthcheck.SingleCheckResult, error) {
	machineDeploymentList := &machinev1alpha1.MachineDeploymentList{}
	// use seed seedClient
	if err := healthChecker.seedClient.List(ctx, machineDeploymentList, client.InNamespace(request.Namespace)); err != nil {
		err := fmt.Errorf("check for sufficient nodes failed. Failed to list machine deployments in namespace %s: %v'", request.Namespace, err)
		healthChecker.logger.Error(err, "Health check failed")
		return nil, err
	}

	if isHealthy, reason, err := machineDeploymentsAreHealthy(machineDeploymentList.Items); !isHealthy {
		err := fmt.Errorf("check for sufficient nodes failed: %v'", err)
		healthChecker.logger.Error(err, "Health check failed")
		return &healthcheck.SingleCheckResult{
			IsHealthy: false,
			Detail:    err.Error(),
			Reason:    *reason,
		}, nil
	}

	nodeList := &corev1.NodeList{}
	if err := healthChecker.shootClient.List(ctx, nodeList); err != nil {
		err := fmt.Errorf("check for sufficient nodes failed. Failed to list shoot nodes: %v'", err)
		healthChecker.logger.Error(err, "Health check failed")
		return nil, err
	}

	if isHealthy, reason, err := checkSufficientNodesAvailable(nodeList, machineDeploymentList); !isHealthy {
		healthChecker.logger.Error(err, "Health check failed")
		return &healthcheck.SingleCheckResult{
			IsHealthy: false,
			Detail:    err.Error(),
			Reason:    *reason,
		}, nil
	}
	return &healthcheck.SingleCheckResult{
		IsHealthy: true,
	}, nil
}
