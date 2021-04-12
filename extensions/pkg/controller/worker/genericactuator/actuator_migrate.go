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

	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/pkg/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/extensions/pkg/controller"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"
)

// Migrate removes all machine related resources (e.g. MachineDeployments, MachineClasses, MachineClassSecrets, MachineSets and Machines)
// without waiting for machine-controller-manager to do that. Before removal it ensures that the MCM is deleted.
func (a *genericActuator) Migrate(ctx context.Context, worker *extensionsv1alpha1.Worker, cluster *controller.Cluster) error {
	logger := a.logger.WithValues("worker", client.ObjectKeyFromObject(worker), "operation", "migrate")

	workerDelegate, err := a.delegateFactory.WorkerDelegate(ctx, worker, cluster)
	if err != nil {
		return errors.Wrap(err, "could not instantiate actuator context")
	}

	// Keep objects for shoot managed resources so that they are not deleted from the shoot during the migration
	if err := managedresources.SetKeepObjects(ctx, a.client, worker.Namespace, McmShootResourceName, true); err != nil {
		return errors.Wrapf(err, "could not keep objects of managed resource containing mcm chart for worker '%s'", kutil.ObjectName(worker))
	}

	// Make sure machine-controller-manager is deleted before deleting the machines.
	if err := a.deleteMachineControllerManager(ctx, logger, worker); err != nil {
		return errors.Wrap(err, "failed deleting machine-controller-manager")
	}

	if err := a.waitUntilMachineControllerManagerIsDeleted(ctx, logger, worker.Namespace); err != nil {
		return errors.Wrap(err, "failed deleting machine-controller-manager")
	}

	if err := a.shallowDeleteAllObjects(ctx, logger, worker.Namespace, &machinev1alpha1.MachineList{}); err != nil {
		return errors.Wrap(err, "shallow deletion of all machine failed")
	}

	if err := a.shallowDeleteAllObjects(ctx, logger, worker.Namespace, &machinev1alpha1.MachineSetList{}); err != nil {
		return errors.Wrap(err, "shallow deletion of all machineSets failed")
	}

	if err := a.shallowDeleteAllObjects(ctx, logger, worker.Namespace, &machinev1alpha1.MachineDeploymentList{}); err != nil {
		return errors.Wrap(err, "shallow deletion of all machineDeployments failed")
	}

	if err := a.shallowDeleteAllObjects(ctx, logger, worker.Namespace, workerDelegate.MachineClassList()); err != nil {
		return errors.Wrap(err, "cleaning up machine classes failed")
	}

	if err := a.shallowDeleteMachineClassSecrets(ctx, logger, worker.Namespace, nil); err != nil {
		return errors.Wrap(err, "cleaning up machine class secrets failed")
	}

	if err := a.removeFinalizerFromWorkerSecretRef(ctx, logger, worker); err != nil {
		return errors.Wrap(err, "unable to remove the finalizers from worker`s secret")
	}

	// Wait until all machine resources have been properly deleted.
	if err := a.waitUntilMachineResourcesDeleted(ctx, logger, worker, workerDelegate); err != nil {
		return gardencorev1beta1helper.DetermineError(err, fmt.Sprintf("Failed while waiting for all machine resources to be deleted: '%s'", err.Error()))
	}

	return nil
}
