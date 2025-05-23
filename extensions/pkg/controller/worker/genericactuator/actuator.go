// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package genericactuator

import (
	"context"
	"fmt"
	"time"

	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener/extensions/pkg/controller/healthcheck"
	"github.com/gardener/gardener/extensions/pkg/controller/worker"
	extensionsworkerhelper "github.com/gardener/gardener/extensions/pkg/controller/worker/helper"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

type genericActuator struct {
	delegateFactory    DelegateFactory
	gardenReader       client.Reader
	seedClient         client.Client
	seedReader         client.Reader
	scheme             *runtime.Scheme
	errorCodeCheckFunc healthcheck.ErrorCodeCheckFunc
}

// NewActuator creates a new Actuator that reconciles
// Worker resources of Gardener's `extensions.gardener.cloud` API group.
// It provides a default implementation that allows easier integration of providers.
// If machine-controller-manager should not be managed then only the delegateFactory must be provided.
func NewActuator(mgr manager.Manager, gardenCluster cluster.Cluster, delegateFactory DelegateFactory, errorCodeCheckFunc healthcheck.ErrorCodeCheckFunc) worker.Actuator {
	actuator := &genericActuator{
		delegateFactory:    delegateFactory,
		seedClient:         mgr.GetClient(),
		seedReader:         mgr.GetAPIReader(),
		scheme:             mgr.GetScheme(),
		errorCodeCheckFunc: errorCodeCheckFunc,
	}

	if gardenCluster != nil {
		actuator.gardenReader = gardenCluster.GetAPIReader()
	}

	return actuator
}

func (a *genericActuator) cleanupMachineDeployments(ctx context.Context, logger logr.Logger, existingMachineDeployments *machinev1alpha1.MachineDeploymentList, wantedMachineDeployments worker.MachineDeployments) error {
	logger.Info("Cleaning up machine deployments")
	for _, existingMachineDeployment := range existingMachineDeployments.Items {
		if !wantedMachineDeployments.HasDeployment(existingMachineDeployment.Name) {
			if err := a.seedClient.Delete(ctx, &existingMachineDeployment); err != nil {
				return err
			}
		}
	}
	return nil
}

func (a *genericActuator) listMachineClassNames(ctx context.Context, namespace string) (sets.Set[string], error) {
	machineClassList := &machinev1alpha1.MachineClassList{}
	if err := a.seedClient.List(ctx, machineClassList, client.InNamespace(namespace)); err != nil {
		return nil, err
	}

	classNames := sets.New[string]()

	if err := meta.EachListItem(machineClassList, func(machineClass runtime.Object) error {
		accessor, err := meta.Accessor(machineClass)
		if err != nil {
			return err
		}

		classNames.Insert(accessor.GetName())
		return nil
	}); err != nil {
		return nil, err
	}

	return classNames, nil
}

func (a *genericActuator) cleanupMachineClasses(ctx context.Context, logger logr.Logger, namespace string, wantedMachineDeployments worker.MachineDeployments) error {
	logger.Info("Cleaning up machine classes")
	machineClassList := &machinev1alpha1.MachineClassList{}
	if err := a.seedClient.List(ctx, machineClassList, client.InNamespace(namespace)); err != nil {
		return err
	}

	return meta.EachListItem(machineClassList, func(obj runtime.Object) error {
		machineClass := obj.(client.Object)
		if !wantedMachineDeployments.HasClass(machineClass.GetName()) {
			logger.Info("Deleting machine class", "machineClass", machineClass)
			if err := a.seedClient.Delete(ctx, machineClass); err != nil {
				return err
			}
		}

		return nil
	})
}

func getMachineClassSecretLabels() map[string]string {
	return map[string]string{v1beta1constants.GardenerPurpose: v1beta1constants.GardenPurposeMachineClass}
}

func (a *genericActuator) listMachineClassSecrets(ctx context.Context, namespace string) (*corev1.SecretList, error) {
	secretList := &corev1.SecretList{}
	if err := a.seedClient.List(ctx, secretList, client.InNamespace(namespace), client.MatchingLabels(getMachineClassSecretLabels())); err != nil {
		return nil, err
	}

	return secretList, nil
}

// cleanupMachineClassSecrets deletes all unused machine class secrets (i.e., those which are not part
// of the provided list <usedSecrets>.
func (a *genericActuator) cleanupMachineClassSecrets(ctx context.Context, logger logr.Logger, namespace string, wantedMachineDeployments worker.MachineDeployments) error {
	logger.Info("Cleaning up machine class secrets")
	secretList, err := a.listMachineClassSecrets(ctx, namespace)
	if err != nil {
		return err
	}

	// Cleanup all secrets which were used for machine classes that do not exist anymore.
	for _, secret := range secretList.Items {
		if !wantedMachineDeployments.HasSecret(secret.Name) {
			if err := a.seedClient.Delete(ctx, &secret); err != nil {
				return err
			}
		}
	}

	return nil
}

// IsMachineControllerStuck determines if the machine controller pod is stuck.
func (a *genericActuator) IsMachineControllerStuck(ctx context.Context, worker *extensionsv1alpha1.Worker) (bool, *string, error) {
	machineDeployments := &machinev1alpha1.MachineDeploymentList{}
	if err := a.seedClient.List(ctx, machineDeployments, client.InNamespace(worker.Namespace)); err != nil {
		return false, nil, err
	}

	machineSets := &machinev1alpha1.MachineSetList{}
	if err := a.seedClient.List(ctx, machineSets, client.InNamespace(worker.Namespace)); err != nil {
		return false, nil, err
	}

	isStuck, msg := isMachineControllerStuck(machineSets.Items, machineDeployments.Items)
	return isStuck, msg, nil
}

const (
	// stuckMCMThreshold defines that the machine deployment set has to be created more than
	// this duration ago to be considered for the check whether a machine controller manager pod is stuck
	stuckMCMThreshold = 2 * time.Minute
	// mcmFinalizer is the finalizer used by the machine controller manager
	// not imported from the MCM to reduce dependencies
	mcmFinalizer = "machine.sapcloud.io/machine-controller-manager"
	// mcmProviderFinalizer is the finalizer used by the out-of-tree machine controller provider
	// not imported from the out-of-tree MCM provider to reduce dependencies
	mcmProviderFinalizer = "machine.sapcloud.io/machine-controller"
)

// isMachineControllerStuck determines if the machine controller pod is stuck.
// A pod is assumed to be stuck if
//   - a machine deployment exists that does not have a machine set with the correct machine class
//   - the machine set does not have a status that indicates (attempted) machine creation
func isMachineControllerStuck(machineSets []machinev1alpha1.MachineSet, machineDeployments []machinev1alpha1.MachineDeployment) (bool, *string) {
	// map the owner reference to the existing machine sets
	ownerReferenceToMachineSet := gardenerutils.BuildOwnerToMachineSetsMap(machineSets)

	for _, machineDeployment := range machineDeployments {
		if !controllerutil.ContainsFinalizer(&machineDeployment, mcmFinalizer) {
			continue
		}

		// do not consider machine deployments that have just recently been created
		if time.Now().UTC().Sub(machineDeployment.CreationTimestamp.UTC()) < stuckMCMThreshold {
			continue
		}

		machineSet := extensionsworkerhelper.GetMachineSetWithMachineClass(machineDeployment.Name, machineDeployment.Spec.Template.Spec.Class.Name, ownerReferenceToMachineSet)
		if machineSet == nil {
			msg := fmt.Sprintf("missing machine set for machine deployment (%s/%s)", machineDeployment.Namespace, machineDeployment.Name)
			return true, &msg
		}
	}
	return false, nil
}
