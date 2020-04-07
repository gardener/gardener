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

package healthcheck

import (
	"context"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

/*
	Each extension can register multiple HealthCheckActuator with various HealthChecks for checking the API Objects it deploys.
	Each NewActuator is responsible for a single extension resource (e.g Worker) - predicates can be defined for fine-grained control over which objects to watch.

    The HealthCheck Reconciler triggers the registered NewActuator to execute the health checks.
	After, the Reconciler writes Conditions to the extension resource. One condition per HealthConditionType (e.g multiple checks that contribute to the HealthConditionType XYZ result in one Condition with .type XYZ).
	To contribute to the Shoot's health, the Gardener/Gardenlet checks each extension for Conditions containing one of the following HealthConditionTypes: SystemComponentsHealthy, EveryNodeReady, ControlPlaneHealthy.
	However extensions are free to choose any healthCheckType.

	Generic HealthCheck functions for various API Objects are provided and can be reused.
	Many providers deploy helm charts via managed resources that are picked up by the resource-manager making sure that
	the helm chart is applied and all its components (Deployments, StatefulSets, DeamonSets, ...) are healthy.
	To integrate, the health check controller can also check the health of managed resources.

	More sophisticated checks should be implemented in the extension itself by using the HealthCheck interface.
*/

// GetExtensionObjectFunc returns the extension object that should be registered with the health check controller
type GetExtensionObjectFunc = func() runtime.Object

// PreCheckFunc checks whether the health check shall be performed based on the given object and cluster.
type PreCheckFunc = func(runtime.Object, *extensionscontroller.Cluster) bool

// ConditionTypeToHealthCheck registers a HealthCheck for the given ConditionType. If the PreCheckFunc is not nil it will
// be executed with the given object before the health check if performed. Otherwise, the health check will always be
// performed.
type ConditionTypeToHealthCheck struct {
	ConditionType string
	PreCheckFunc  PreCheckFunc
	HealthCheck   HealthCheck
}

// HealthCheckActuator acts upon registered resources.
type HealthCheckActuator interface {
	// ExecuteHealthCheckFunctions is regularly called by the health check controller
	// Executes all registered Health Checks and aggregates the result
	// Returns Result for each healthConditionType registered with the individual health checks.
	// returns an error if it could not execute the health checks
	// returning an error results in a condition with with type "Unknown" with reason "ConditionCheckError"
	ExecuteHealthCheckFunctions(context.Context, types.NamespacedName) (*[]Result, error)
}

// Result represents an aggregated health status for the health checks performed on the dependent API Objects of an extension resource.
// An Result refers to a single healthConditionType (e.g SystemComponentsHealthy) of an extension Resource.
type Result struct {
	// HealthConditionType is being used as the .type field of the Condition that the HealthCheck controller writes to the extension Resource.
	// To contribute to the Shoot's health, the Gardener checks each extension for a Health Condition Type of SystemComponentsHealthy, EveryNodeReady, ControlPlaneHealthy.
	HealthConditionType string
	// IsHealthy indicates if all the health checks for an extension resource have been successful
	IsHealthy bool
	// Detail contains details for health checks being unsuccessful
	Detail *string
	// SuccessfulChecks is the amount of successful health checks
	SuccessfulChecks int
	// UnsuccessfulChecks is the amount of health checks that were not successful
	UnsuccessfulChecks int
	// FailedChecks is the amount of health checks that could not be performed (e.g client could not reach Api Server)
	// Results in a condition with with type "Unknown" with reason "ConditionCheckError" for this healthConditionType
	FailedChecks int
}

// GetDetails returns the details of the health check result
func (h *Result) GetDetails() string {
	if h.Detail == nil {
		return ""
	}
	return *h.Detail
}

// HealthCheck represents a single health check
// Each health check gets the shoot and seed clients injected
// returns isHealthy, conditionReason, conditionDetail and error
// returning an error means the health check could not be conducted and will result in a condition with with type "Unknown" and reason "ConditionCheckError"
type HealthCheck interface {
	// Check is the function that executes the actual health check
	Check(context.Context, types.NamespacedName) (*SingleCheckResult, error)
	// InjectSeedClient injects the seed client
	InjectSeedClient(client.Client)
	// InjectShootClient injects the shoot client
	InjectShootClient(client.Client)
	// SetLoggerSuffix injects the logger
	SetLoggerSuffix(string, string)
	// DeepCopy clones the healthCheck
	DeepCopy() HealthCheck
}

// SingleCheckResult is the result for a health check
type SingleCheckResult struct {
	// IsHealthy indicates if all the health checks for an extension resource have been successful
	IsHealthy bool
	// Detail contains details for the health check being unsuccessful
	Detail string
	// Reason contains the reason for the health check being unsuccessful
	Reason string
}
