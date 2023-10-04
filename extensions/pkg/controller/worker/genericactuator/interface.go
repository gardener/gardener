// Copyright 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package genericactuator

import (
	"context"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/controller/worker"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

// WorkerDelegate is used for the Worker reconciliation.
type WorkerDelegate interface {
	// DeployMachineClasses generates and creates the provider specific machine classes.
	DeployMachineClasses(context.Context) error

	// GenerateMachineDeployments generates the configuration for the desired machine deployments.
	GenerateMachineDeployments(context.Context) (worker.MachineDeployments, error)

	// UpdateMachineImagesStatus will store a list of machine images used by the
	// machines associated with this Worker resource in its provider status.
	// The controller can look up its provider-specific machine image information
	// in case the required version has been removed from the `CloudProfile`.
	UpdateMachineImagesStatus(context.Context) error

	// PreReconcileHook is a hook called at the beginning of the worker reconciliation flow.
	PreReconcileHook(context.Context) error
	// PostReconcileHook is a hook called at the end of the worker reconciliation flow.
	PostReconcileHook(context.Context) error
	// PreDeleteHook is a hook called at the beginning of the worker deletion flow.
	PreDeleteHook(context.Context) error
	// PostDeleteHook is a hook called at the end of the worker deletion flow.
	PostDeleteHook(context.Context) error
}

// DelegateFactory acts upon Worker resources.
type DelegateFactory interface {
	// WorkerDelegate returns a worker delegate interface that is used for the Worker reconciliation
	// based on this generic actuator.
	WorkerDelegate(context.Context, *extensionsv1alpha1.Worker, *extensionscontroller.Cluster) (WorkerDelegate, error)
}
