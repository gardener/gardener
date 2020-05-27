// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"fmt"
	"time"

	"github.com/gardener/gardener/extensions/pkg/controller"
	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/pkg/errors"
)

// Migrate removes all machine related resources (e.g. MachineDeployments, MachineClasses, MachineClassSecrets, MachineSets and Machines)
// without waiting for machine-controller-manager to do that. Before removal it ensures that the MCM is deleted.
func (a *genericActuator) Migrate(ctx context.Context, worker *extensionsv1alpha1.Worker, cluster *controller.Cluster) error {
	workerDelegate, err := a.delegateFactory.WorkerDelegate(ctx, worker, cluster)
	if err != nil {
		return errors.Wrap(err, "could not instantiate actuator context")
	}

	// Make sure machine-controller-manager is deleted before deleting the machines.
	if err := a.deleteMachineControllerManager(ctx, worker); err != nil {
		return errors.Wrap(err, "failed deleting machine-controller-manager")
	}

	if err := a.waitUntilMachineControllerManagerIsDeleted(ctx, worker.Namespace); err != nil {
		return errors.Wrap(err, "failed deleting machine-controller-manager")
	}

	if err := a.shallowDeleteAllObjects(ctx, worker.Namespace, &machinev1alpha1.MachineList{}); err != nil {
		return errors.Wrap(err, "shallow deletion of all machine failed")
	}

	if err := a.shallowDeleteAllObjects(ctx, worker.Namespace, &machinev1alpha1.MachineSetList{}); err != nil {
		return errors.Wrap(err, "shallow deletion of all machineSets failed")
	}

	if err := a.shallowDeleteAllObjects(ctx, worker.Namespace, &machinev1alpha1.MachineDeploymentList{}); err != nil {
		return errors.Wrap(err, "shallow deletion of all machineDeployments failed")
	}

	if err := a.shallowDeleteAllObjects(ctx, worker.Namespace, workerDelegate.MachineClassList()); err != nil {
		return errors.Wrap(err, "cleaning up machine classes failed")
	}

	if err := a.shallowDeleteMachineClassSecrets(ctx, worker.Namespace, nil); err != nil {
		return errors.Wrap(err, "cleaning up machine class secrets failed")
	}

	// Wait until all machine resources have been properly deleted.
	if err := a.waitUntilMachineResourcesDeleted(ctx, worker, workerDelegate); err != nil {
		return gardencorev1beta1helper.DetermineError(err, fmt.Sprintf("Failed while waiting for all machine resources to be deleted: '%s'", err.Error()))
	}

	return nil
}

func (a *genericActuator) waitUntilMachineControllerManagerIsDeleted(ctx context.Context, namespace string) error {
	return wait.PollUntil(5*time.Second, func() (bool, error) {
		machineControllerManagerDeployment := &appsv1.Deployment{}
		if err := a.client.Get(ctx, client.ObjectKey{Namespace: namespace, Name: McmDeploymentName}, machineControllerManagerDeployment); err != nil {
			if apierrors.IsNotFound(err) {
				return true, nil
			}
			return false, err
		}

		return false, nil
	}, ctx.Done())
}
