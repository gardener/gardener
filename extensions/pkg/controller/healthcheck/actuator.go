// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package healthcheck

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

/*
	Each extension can register multiple HealthCheckActuators with various health checks to check the API Objects it deploys.
	Each new actuator is responsible for a single extension resource (e.g Worker) - predicates can be defined for fine-grained control over which objects to watch.

    The HealthCheck reconciler triggers the registered actuator to execute the health checks.
	After, the reconciler writes conditions to the extension resource. Multiple checks that contribute to the HealthConditionType XYZ result in only one condition with .type XYZ).
	To contribute to the Shoot's health, the Gardener/Gardenlet checks each extension for conditions containing one of the following HealthConditionTypes:
      - SystemComponentsHealthy,
      - EveryNodeReady,
      - ControlPlaneHealthy.
	However, extensions are free to choose any healthCheckType.

	Generic HealthCheck functions for various API Objects are provided and can be reused.
	Many providers deploy helm charts via managed resources that are picked up by the resource-manager making sure that
	the helm chart is applied and all its components (Deployments, StatefulSets, DaemonSets, ...) are healthy.
	To integrate, the health check controller can also check the health of managed resources.

	More sophisticated checks should be implemented in the extension itself by using the HealthCheck interface.
*/

// GetExtensionObjectFunc returns the extension object that should be registered with the health check controller.
// For example: func() extensionsv1alpha1.Object {return &extensionsv1alpha1.Worker{}}
type GetExtensionObjectFunc = func() extensionsv1alpha1.Object

// GetExtensionObjectListFunc returns the extension object list that should be registered with the health check controller.
// For example: func() client.ObjectList { return &extensionsv1alpha1.WorkerList{} }
type GetExtensionObjectListFunc = func() client.ObjectList

// PreCheckFunc checks whether the health check shall be performed based on the given object and cluster.
type PreCheckFunc = func(context.Context, client.Client, client.Object, *extensionscontroller.Cluster) bool

// ErrorCodeCheckFunc checks if the given error is user specific and return respective Gardener ErrorCodes.
type ErrorCodeCheckFunc = func(error) []gardencorev1beta1.ErrorCode

// ConditionTypeToHealthCheck registers a HealthCheck for the given ConditionType. If the PreCheckFunc is not nil it will
// be executed with the given object before the health check if performed. Otherwise, the health check will always be
// performed.
type ConditionTypeToHealthCheck struct {
	ConditionType      string
	PreCheckFunc       PreCheckFunc
	HealthCheck        HealthCheck
	ErrorCodeCheckFunc ErrorCodeCheckFunc
}

// HealthCheckActuator acts upon registered resources.
type HealthCheckActuator interface {
	// ExecuteHealthCheckFunctions is regularly called by the health check controller
	// Executes all registered health checks and aggregates the results.
	// Returns
	//  - Result for each healthConditionTypes registered with the individual health checks.
	//  - an error if it could not execute the health checks.
	//    This results in a condition with with type "Unknown" with reason "ConditionCheckError".
	ExecuteHealthCheckFunctions(context.Context, logr.Logger, types.NamespacedName) (*[]Result, error)
}

// Result represents an aggregated health status for the health checks performed on the dependent API Objects of an extension resource.
// A Result refers to a single healthConditionType (e.g SystemComponentsHealthy) of an extension Resource.
type Result struct {
	// HealthConditionType is used as the .type field of the Condition that the HealthCheck controller writes to the extension Resource.
	// To contribute to the Shoot's health, the Gardener checks each extension for a Health Condition Type of SystemComponentsHealthy, EveryNodeReady, ControlPlaneHealthy.
	HealthConditionType string
	// Status contains the status for the health checks that have been performed for an extension resource
	Status gardencorev1beta1.ConditionStatus
	// Detail contains details to why the health checks are unsuccessful
	Detail *string
	// SuccessfulChecks is the amount of successful health checks
	SuccessfulChecks int
	// ProgressingChecks is the amount of progressing health checks
	ProgressingChecks int
	// UnsuccessfulChecks is the amount of unsuccessful health checks
	UnsuccessfulChecks int
	// FailedChecks is the amount of health checks that could not be performed (e.g client could not reach Api Server)
	// Results in a condition with with type "Unknown" with reason "ConditionCheckError" for this healthConditionType
	FailedChecks int
	// Codes is an optional list of error codes that were produced by the health checks.
	Codes []gardencorev1beta1.ErrorCode
	// ProgressingThreshold is the threshold duration after which a health check that reported the `Progressing` status
	// shall be transitioned to `False`
	ProgressingThreshold *time.Duration
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
	// SetLoggerSuffix injects the logger
	SetLoggerSuffix(string, string)
	// DeepCopy clones the healthCheck
	DeepCopy() HealthCheck
}

// SingleCheckResult is the result for a health check
type SingleCheckResult struct {
	// Status contains the status for the health check that has been performed for an extension resource
	Status gardencorev1beta1.ConditionStatus
	// Detail contains details for the health check being unsuccessful
	Detail string
	// Codes optionally contains a list of error codes related to the health check
	Codes []gardencorev1beta1.ErrorCode
	// ProgressingThreshold is the threshold duration after which a health check that reported the `Progressing` status
	// shall be transitioned to `False`
	ProgressingThreshold *time.Duration
}
