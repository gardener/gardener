// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package genericactuator

import (
	"context"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/controller/worker"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
)

// WorkerDelegate is used for the Worker reconciliation.
type WorkerDelegate interface {
	// GetMachineControllerManagerChart should return the the chart and the values for the machine-controller-manager
	// deployment.
	GetMachineControllerManagerChartValues(context.Context) (map[string]interface{}, error)
	// GetMachineControllerManagerShootChart should return the values to render the chart containing resources
	// that are required by the machine-controller-manager inside the shoot cluster itself.
	GetMachineControllerManagerShootChartValues(context.Context) (map[string]interface{}, error)

	// MachineClassKind yields the name of the provider specific machine class.
	MachineClassKind() string
	// MachineClassList yields a newly initialized machine class list object.
	MachineClassList() runtime.Object
	// DeployMachineClasses generates and creates the provider specific machine classes.
	DeployMachineClasses(context.Context) error

	// GenerateMachineDeployments generates the configuration for the desired machine deployments.
	GenerateMachineDeployments(context.Context) (worker.MachineDeployments, error)

	// GetMachineImages returns the list of used machine images for this `Worker` resource. It will be stored in the
	// `.status.providerStatus` field of the `Worker` resource such that the controller can look up its provider-specific
	// machine image information in case the required version has been removed from its componentconfig.
	GetMachineImages(context.Context) (runtime.Object, error)
}

// DelegateFactory acts upon Worker resources.
type DelegateFactory interface {
	// WorkerDelegate returns a worker delegate interface that is used for the Worker reconciliation
	// based on this generic actuator.
	WorkerDelegate(context.Context, *extensionsv1alpha1.Worker, *extensionscontroller.Cluster) (WorkerDelegate, error)
}
