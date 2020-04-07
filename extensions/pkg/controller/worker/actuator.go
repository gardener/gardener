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

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

// Actuator acts upon Worker resources.
type Actuator interface {
	// Reconcile reconciles the Worker.
	Reconcile(context.Context, *extensionsv1alpha1.Worker, *extensionscontroller.Cluster) error
	// Delete deletes the Worker.
	Delete(context.Context, *extensionsv1alpha1.Worker, *extensionscontroller.Cluster) error
	// Restore reads from the worker.status.state field and deploys the machines and machineSet
	Restore(context.Context, *extensionsv1alpha1.Worker, *extensionscontroller.Cluster) error
	// Migrate deletes the MCM, machineDeployments, machineClasses, machineClassSecrets,
	// machineSets and the machines. The underlying VMs representing the Shoot nodes are not deleted
	Migrate(context.Context, *extensionsv1alpha1.Worker, *extensionscontroller.Cluster) error
}

// StateActuator acts upon Worker's State resources.
type StateActuator interface {
	// Reconcile reconciles the Worker State.
	Reconcile(context.Context, *extensionsv1alpha1.Worker) error
}
