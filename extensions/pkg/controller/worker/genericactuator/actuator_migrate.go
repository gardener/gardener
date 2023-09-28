// Copyright 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/extensions/pkg/controller"
	extensionsworkercontroller "github.com/gardener/gardener/extensions/pkg/controller/worker"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"
)

// Migrate removes all machine related resources (e.g. MachineDeployments, MachineClasses, MachineClassSecrets, MachineSets and Machines)
// without waiting for machine-controller-manager to do that. Before removal it ensures that the MCM is deleted.
func (a *genericActuator) Migrate(ctx context.Context, log logr.Logger, worker *extensionsv1alpha1.Worker, cluster *controller.Cluster) error {
	log = log.WithValues("operation", "migrate")

	workerDelegate, err := a.delegateFactory.WorkerDelegate(ctx, worker, cluster)
	if err != nil {
		return fmt.Errorf("could not instantiate actuator context: %w", err)
	}

	// Keep objects for shoot managed resources so that they are not deleted from the shoot during the migration
	if err := managedresources.SetKeepObjects(ctx, a.client, worker.Namespace, McmShootResourceName, true); err != nil {
		return fmt.Errorf("could not keep objects of managed resource containing mcm chart for worker '%s': %w", kubernetesutils.ObjectName(worker), err)
	}

	// Make sure machine-controller-manager is deleted before deleting the machines.
	if err := a.deleteMachineControllerManager(ctx, log, worker); err != nil {
		return fmt.Errorf("failed deleting machine-controller-manager: %w", err)
	}

	if a.mcmManaged {
		if err := a.waitUntilMachineControllerManagerIsDeleted(ctx, log, worker.Namespace); err != nil {
			return fmt.Errorf("failed deleting machine-controller-manager: %w", err)
		}
	}

	// TODO(rfranzke): Instead of checking for machine objects, we could also only persist the state when it is nil.
	//  This is only to prevent that subsequent executions of Migrate don't overwrite/delete previously persisted state.
	//  We cannot do it this way yet since gardenlet does not persist the ShootState after all extension resources have
	//  been migrated. It is planned to do so after v1.79 has been released, hence we have to wait a bit longer.
	machineObjectsExist, err := kubernetesutils.ResourcesExist(ctx, a.client, machinev1alpha1.SchemeGroupVersion.WithKind("MachineList"), client.InNamespace(worker.Namespace))
	if err != nil {
		return fmt.Errorf("failed checking whether machine objects exist: %w", err)
	}
	if machineObjectsExist {
		if err := extensionsworkercontroller.PersistState(ctx, log, a.client, worker); err != nil {
			return fmt.Errorf("failed persisting worker state: %w", err)
		}
	}

	if err := a.shallowDeleteAllObjects(ctx, log, worker.Namespace, &machinev1alpha1.MachineList{}); err != nil {
		return fmt.Errorf("shallow deletion of all machine failed: %w", err)
	}

	if err := a.shallowDeleteAllObjects(ctx, log, worker.Namespace, &machinev1alpha1.MachineSetList{}); err != nil {
		return fmt.Errorf("shallow deletion of all machineSets failed: %w", err)
	}

	if err := a.shallowDeleteAllObjects(ctx, log, worker.Namespace, &machinev1alpha1.MachineDeploymentList{}); err != nil {
		return fmt.Errorf("shallow deletion of all machineDeployments failed: %w", err)
	}

	if err := a.shallowDeleteAllObjects(ctx, log, worker.Namespace, workerDelegate.MachineClassList()); err != nil {
		return fmt.Errorf("cleaning up machine classes failed: %w", err)
	}

	if err := a.shallowDeleteMachineClassSecrets(ctx, log, worker.Namespace, nil); err != nil {
		return fmt.Errorf("cleaning up machine class secrets failed: %w", err)
	}

	if err := a.removeFinalizerFromWorkerSecretRef(ctx, log, worker); err != nil {
		return fmt.Errorf("unable to remove the finalizers from worker`s secret: %w", err)
	}

	// Wait until all machine resources have been properly deleted.
	if err := a.waitUntilMachineResourcesDeleted(ctx, log, worker, workerDelegate); err != nil {
		return fmt.Errorf("failed while waiting for all machine resources to be deleted: %w", err)
	}

	return nil
}
