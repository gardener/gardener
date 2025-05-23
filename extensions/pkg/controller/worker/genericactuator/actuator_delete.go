// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package genericactuator

import (
	"context"
	"errors"
	"fmt"
	"time"

	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/utils/flow"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	retryutils "github.com/gardener/gardener/pkg/utils/retry"
)

const (
	forceDeletionLabelKey   = "force-deletion"
	forceDeletionLabelValue = "True"
)

func (a *genericActuator) Delete(ctx context.Context, log logr.Logger, worker *extensionsv1alpha1.Worker, cluster *extensionscontroller.Cluster) error {
	log = log.WithValues("operation", "delete")

	workerDelegate, err := a.delegateFactory.WorkerDelegate(ctx, worker, cluster)
	if err != nil {
		newError := fmt.Errorf("could not instantiate actuator: %w", err)
		if a.errorCodeCheckFunc != nil {
			return v1beta1helper.NewErrorWithCodes(newError, a.errorCodeCheckFunc(err)...)
		}
		return newError
	}

	// Call pre deletion hook to prepare Worker deletion.
	if err := workerDelegate.PreDeleteHook(ctx); err != nil {
		return fmt.Errorf("pre worker deletion hook failed: %w", err)
	}

	// Redeploy generated machine classes to update credentials machine-controller-manager used.
	log.Info("Deploying the machine classes")
	if err := workerDelegate.DeployMachineClasses(ctx); err != nil {
		return fmt.Errorf("failed to deploy the machine classes: %w", err)
	}

	// Wait until the machine class credentials secret has been acquired.
	log.Info("Waiting until the machine class credentials secret has been acquired")
	if err := a.waitUntilCredentialsSecretAcquiredOrReleased(ctx, true, worker); err != nil {
		return fmt.Errorf("failed while waiting for the machine class credentials secret to be acquired: %w", err)
	}

	// Mark all existing machines to become forcefully deleted.
	log.Info("Marking all machines to become forcefully deleted")
	if err := markAllMachinesForcefulDeletion(ctx, log, a.seedClient, worker.Namespace); err != nil {
		return fmt.Errorf("marking all machines for forceful deletion failed: %w", err)
	}

	// Delete all machine deployments.
	log.Info("Deleting all machine deployments")
	if err := a.seedClient.DeleteAllOf(ctx, &machinev1alpha1.MachineDeployment{}, client.InNamespace(worker.Namespace)); err != nil {
		return fmt.Errorf("cleaning up all machine deployments failed: %w", err)
	}

	// Delete all machine classes.
	log.Info("Deleting all machine classes")
	if err := a.seedClient.DeleteAllOf(ctx, &machinev1alpha1.MachineClass{}, client.InNamespace(worker.Namespace)); err != nil {
		return fmt.Errorf("cleaning up all machine classes failed: %w", err)
	}

	// Delete all machine class secrets.
	log.Info("Deleting all machine class secrets")
	if err := a.seedClient.DeleteAllOf(ctx, &corev1.Secret{}, client.InNamespace(worker.Namespace), client.MatchingLabels(getMachineClassSecretLabels())); err != nil {
		return fmt.Errorf("cleaning up all machine class secrets failed: %w", err)
	}

	// Wait until all machine resources have been properly deleted.
	if err := gardenerutils.WaitUntilMachineResourcesDeleted(ctx, log, a.seedClient, worker.Namespace); err != nil {
		newError := fmt.Errorf("failed while waiting for all machine resources to be deleted: %w", err)
		if a.errorCodeCheckFunc != nil {
			return v1beta1helper.NewErrorWithCodes(newError, a.errorCodeCheckFunc(err)...)
		}
		return newError
	}

	// Wait until the machine class credentials secret has been released.
	log.Info("Waiting until the machine class credentials secret has been released")
	if err := a.waitUntilCredentialsSecretAcquiredOrReleased(ctx, false, worker); err != nil {
		return fmt.Errorf("failed while waiting for the machine class credentials secret to be released: %w", err)
	}

	// Call post deletion hook after Worker deletion has happened.
	if err := workerDelegate.PostDeleteHook(ctx); err != nil {
		return fmt.Errorf("post worker deletion hook failed: %w", err)
	}

	return nil
}

// ForceDelete simply returns nil in case of forceful deletion since cleaning up the machines would never succeed in this case.
// So we proceed to remove the finalizer without any action.
func (a *genericActuator) ForceDelete(_ context.Context, _ logr.Logger, _ *extensionsv1alpha1.Worker, _ *extensionscontroller.Cluster) error {
	return nil
}

// Mark all existing machines to become forcefully deleted.
func markAllMachinesForcefulDeletion(ctx context.Context, log logr.Logger, cl client.Client, namespace string) error {
	log.Info("Marking all machines for forceful deletion")
	// Mark all existing machines to become forcefully deleted.
	existingMachines := &machinev1alpha1.MachineList{}
	if err := cl.List(ctx, existingMachines, client.InNamespace(namespace)); err != nil {
		return err
	}

	var tasks []flow.TaskFn
	for _, machine := range existingMachines.Items {
		m := machine
		tasks = append(tasks, func(ctx context.Context) error {
			return markMachineForcefulDeletion(ctx, cl, &m)
		})
	}

	if err := flow.Parallel(tasks...)(ctx); err != nil {
		return fmt.Errorf("failed labelling machines for forceful deletion: %w", err)
	}

	return nil
}

// markMachineForcefulDeletion labels a machine object to become forcefully deleted.
func markMachineForcefulDeletion(ctx context.Context, cl client.Client, machine *machinev1alpha1.Machine) error {
	if machine.Labels == nil {
		machine.Labels = map[string]string{}
	}

	if val, ok := machine.Labels[forceDeletionLabelKey]; ok && val == forceDeletionLabelValue {
		return nil
	}

	machine.Labels[forceDeletionLabelKey] = forceDeletionLabelValue
	return cl.Update(ctx, machine)
}

func (a *genericActuator) waitUntilCredentialsSecretAcquiredOrReleased(ctx context.Context, acquired bool, worker *extensionsv1alpha1.Worker) error {
	acquiredOrReleased := false
	return retryutils.UntilTimeout(ctx, 5*time.Second, 5*time.Minute, func(ctx context.Context) (bool, error) {
		// Check whether the finalizer of the machine class credentials secret has been added or removed.
		if !acquiredOrReleased {
			secret, err := kubernetesutils.GetSecretByReference(ctx, a.seedClient, &worker.Spec.SecretRef)
			if err != nil {
				return retryutils.SevereError(fmt.Errorf("could not get the secret referenced by worker: %+v", err))
			}

			// We need to check for both mcmFinalizer and mcmProviderFinalizer:
			// - mcmFinalizer is the finalizer used by machine controller manager and its in-tree providers
			// - mcmProviderFinalizer is the finalizer used by out-of-tree machine controller providers
			if (controllerutil.ContainsFinalizer(secret, mcmFinalizer) || controllerutil.ContainsFinalizer(secret, mcmProviderFinalizer)) == acquired {
				acquiredOrReleased = true
			}
		}

		if !acquiredOrReleased {
			return retryutils.MinorError(errors.New("machine class credentials secret has not yet been acquired or released"))
		}
		return retryutils.Ok()
	})
}
