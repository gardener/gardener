// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package hybridbotanist

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	gardencorev1alpha1helper "github.com/gardener/gardener/pkg/apis/core/v1alpha1/helper"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation"
	"github.com/gardener/gardener/pkg/operation/common"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"

	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
)

var chartPathMachines = filepath.Join(common.ChartPath, "seed-machines", "charts", "machines")

// ReconcileMachines asks the CloudBotanist to provide the specific configuration for MachineClasses and MachineDeployments.
// It deploys the machine specifications, waits until it is ready and cleans old specifications.
func (b *HybridBotanist) ReconcileMachines() error {
	machineClassKind, _, machineClassChartName := b.ShootCloudBotanist.GetMachineClassInfo()

	// Generate machine classes configuration and list of corresponding machine deployments.
	machineClassChartValues, wantedMachineDeployments, err := b.ShootCloudBotanist.GenerateMachineConfig()
	if err != nil {
		return fmt.Errorf("The CloudBotanist failed to generate the machine config: '%s'", err.Error())
	}
	b.MachineDeployments = wantedMachineDeployments

	// Get list of existing machine class names and list of used machine class secrets.
	existingMachineClassNames, usedSecrets, err := b.ShootCloudBotanist.ListMachineClasses()
	if err != nil {
		return err
	}

	// Merge the list of used secrets with the list of those which are wanted. The machine class secret names
	// always match the machine class name itself, hence, we check against the class name.
	for _, wantedMachineDeployment := range wantedMachineDeployments {
		usedSecrets.Insert(wantedMachineDeployment.ClassName)
	}

	// During the time a rolling update happens we do not want the cluster autoscaler to interfer, hence it
	// is removed (and later, at the end of the flow, deployed again).
	if b.Shoot.WantsClusterAutoscaler {
		rollingUpdate := false
		// Check whether new machine classes have been computed (resulting in a rolling update of the nodes).
		for _, machineDeployment := range wantedMachineDeployments {
			if !existingMachineClassNames.Has(machineDeployment.ClassName) {
				rollingUpdate = true
				break
			}
		}

		// When the Shoot gets hibernated we want to remove the cluster auto scaler so that it does not interfer
		// with Gardeners modifications on the machine deployment's replicas fields.
		if b.Shoot.IsHibernated || rollingUpdate {
			if err := b.Botanist.DeleteClusterAutoscaler(); err != nil {
				return err
			}
			if err := b.Botanist.WaitUntilClusterAutoscalerDeleted(); err != nil {
				return err
			}
		}
	}

	// Deploy generated machine classes.
	values := map[string]interface{}{
		"machineClasses": machineClassChartValues,
	}
	if err := b.ApplyChartSeed(filepath.Join(common.ChartPath, "seed-machines", "charts", machineClassChartName), b.Shoot.SeedNamespace, machineClassChartName, values, nil); err != nil {
		return fmt.Errorf("Failed to deploy the generated machine classes: '%s'", err.Error())
	}

	// Get the list of all existing machine deployments
	existingMachineDeployments, err := b.K8sSeedClient.Machine().MachineV1alpha1().MachineDeployments(b.Shoot.SeedNamespace).List(metav1.ListOptions{})
	if err != nil {
		return err
	}

	// Generate machine deployment configuration based on previously computed list of deployments.
	machineDeploymentChartValues, err := b.generateMachineDeploymentConfig(existingMachineDeployments, wantedMachineDeployments, machineClassKind)
	if err != nil {
		return fmt.Errorf("Failed to generate the machine deployment config: '%s'", err.Error())
	}

	// Deploy generated machine deployments.
	if err := b.ApplyChartSeed(filepath.Join(chartPathMachines), b.Shoot.SeedNamespace, "machines", machineDeploymentChartValues, nil); err != nil {
		return fmt.Errorf("Failed to deploy the generated machine deployments: '%s'", err.Error())
	}

	// Wait until all generated machine deployments are healthy/available.
	if err := b.waitUntilMachineDeploymentsAvailable(wantedMachineDeployments); err != nil {
		return gardencorev1alpha1helper.DetermineError(fmt.Sprintf("Failed while waiting for all machine deployments to be ready: '%s'", err.Error()))
	}

	// Delete all old machine deployments (i.e. those which were not previously computed but exist in the cluster).
	if err := b.cleanupMachineDeployments(existingMachineDeployments, wantedMachineDeployments); err != nil {
		return fmt.Errorf("Failed to cleanup the machine deployments: '%s'", err.Error())
	}

	// Delete all old machine classes (i.e. those which were not previously computed but exist in the cluster).
	if err := b.ShootCloudBotanist.CleanupMachineClasses(wantedMachineDeployments); err != nil {
		return fmt.Errorf("The CloudBotanist failed to cleanup the machine classes: '%s'", err.Error())
	}

	// Delete all old machine class secrets (i.e. those which were not previously computed but exist in the cluster).
	if err := b.cleanupMachineClassSecrets(usedSecrets); err != nil {
		return fmt.Errorf("The CloudBotanist failed to cleanup the orphaned machine class secrets: '%s'", err.Error())
	}

	// Scale down machine-controller-manager if shoot is hibernated.
	if b.Shoot.IsHibernated {
		if err := kubernetes.ScaleDeployment(context.TODO(), b.K8sSeedClient.Client(), kutil.Key(b.Shoot.SeedNamespace, common.MachineControllerManagerDeploymentName), 0); err != nil {
			return err
		}
	}

	return nil
}

// DestroyMachines deletes all existing MachineDeployments. As it won't trigger the drain of nodes it needs to label
// the existing machines. In case an errors occurs, it will return it.
func (b *HybridBotanist) DestroyMachines() error {
	var (
		errorList                []error
		wg                       sync.WaitGroup
		_, machineClassPlural, _ = b.ShootCloudBotanist.GetMachineClassInfo()
		emptyMachineDeployments  = operation.MachineDeployments{}
	)

	// Mark all existing machines to become forcefully deleted.
	existingMachines, err := b.K8sSeedClient.Machine().MachineV1alpha1().Machines(b.Shoot.SeedNamespace).List(metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, machine := range existingMachines.Items {
		wg.Add(1)
		go func(machine machinev1alpha1.Machine) {
			defer wg.Done()
			if err := b.markMachineForcefulDeletion(machine); err != nil {
				errorList = append(errorList, err)
			}
		}(machine)
	}

	wg.Wait()
	if len(errorList) > 0 {
		return fmt.Errorf("Labelling machines (to become forcefully deleted) failed: %v", errorList)
	}

	// Get the list of all existing machine deployments.
	existingMachineDeployments, err := b.K8sSeedClient.Machine().MachineV1alpha1().MachineDeployments(b.Shoot.SeedNamespace).List(metav1.ListOptions{})
	if err != nil {
		return err
	}

	if err := b.cleanupMachineDeployments(existingMachineDeployments, emptyMachineDeployments); err != nil {
		return fmt.Errorf("Cleaning up machine deployments failed: %s", err.Error())
	}
	if err := b.ShootCloudBotanist.CleanupMachineClasses(emptyMachineDeployments); err != nil {
		return fmt.Errorf("Cleaning up machine classes failed: %s", err.Error())
	}

	// Wait until all machine resources have been properly deleted.
	if err := b.waitUntilMachineResourcesDeleted(machineClassPlural); err != nil {
		return gardencorev1alpha1helper.DetermineError(fmt.Sprintf("Failed while waiting for all machine resources to be deleted: '%s'", err.Error()))
	}

	return nil
}

// RefreshMachineClassSecrets updates all existing machine class secrets to reflect the latest
// cloud provider credentials.
func (b *HybridBotanist) RefreshMachineClassSecrets() error {
	secretList, err := b.listMachineClassSecrets()
	if err != nil {
		return err
	}

	// Refresh all secrets by updating the cloud provider credentials to the latest known values.
	for _, secret := range secretList.Items {
		var newSecret = secret

		newSecret.Data = b.ShootCloudBotanist.GenerateMachineClassSecretData()
		newSecret.Data["userData"] = secret.Data["userData"]

		if _, err := b.K8sSeedClient.UpdateSecretObject(&newSecret); err != nil {
			return err
		}
	}

	return nil
}

// generateMachineDeploymentConfig generates the configuration values for the machine deployment Helm chart. It
// does that based on the provided list of to-be-deployed <wantedMachineDeployments>.
func (b *HybridBotanist) generateMachineDeploymentConfig(existingMachineDeployments *machinev1alpha1.MachineDeploymentList, wantedMachineDeployments operation.MachineDeployments, classKind string) (map[string]interface{}, error) {
	var (
		values   = []map[string]interface{}{}
		replicas int
	)

	for _, deployment := range wantedMachineDeployments {
		config := map[string]interface{}{
			"name":            deployment.Name,
			"minReadySeconds": 500,
			"rollingUpdate": map[string]interface{}{
				"maxSurge":       deployment.MaxSurge.String(),
				"maxUnavailable": deployment.MaxUnavailable.String(),
			},
			"labels": map[string]interface{}{
				"name": deployment.Name,
			},
			"class": map[string]interface{}{
				"kind": classKind,
				"name": deployment.ClassName,
			},
		}
		existingMachineDeployment := getExistingMachineDeployment(existingMachineDeployments, deployment.Name)

		switch {
		// If the Shoot is hibernated then the machine deployment's replicas should be zero.
		case b.Shoot.IsHibernated:
			replicas = 0
		// If the cluster autoscaler is not enabled then min=max (as per API validation), hence
		// we can use either min or max.
		case !b.Shoot.WantsClusterAutoscaler:
			replicas = deployment.Minimum
		// If the machine deployment does not yet exist we set replicas to min so that the cluster
		// autoscaler can scale them as required.
		case existingMachineDeployment == nil:
			replicas = deployment.Minimum
		// If the Shoot was hibernated and is now woken up we set replicas to min so that the cluster
		// autoscaler can scale them as required.
		case shootIsWokenUp(b.Shoot.IsHibernated, existingMachineDeployments):
			replicas = deployment.Minimum
		// If the shoot worker pool minimum was updated and if the current machine deployment replica
		// count is less than minimum, we update the machine deployment replica count to updated minimum.
		case int(existingMachineDeployment.Spec.Replicas) < deployment.Minimum:
			replicas = deployment.Minimum
		// If the shoot worker pool maximum was updated and if the current machine deployment replica
		// count is greater than maximum, we update the machine deployment replica count to updated maximum.
		case int(existingMachineDeployment.Spec.Replicas) > deployment.Maximum:
			replicas = deployment.Maximum
		// In this case the machine deployment must exist (otherwise the above case was already true),
		// and the cluster autoscaler must be enabled. We do not want to override the machine deployment's
		// replicas as the cluster autoscaler is responsible for setting appropriate values.
		default:
			replicas = getDeploymentSpecReplicas(existingMachineDeployments, deployment.Name)
			if replicas == -1 {
				replicas = deployment.Minimum
			}
		}

		config["replicas"] = replicas
		values = append(values, config)
	}

	return map[string]interface{}{
		"machineDeployments": values,
	}, nil
}

// markMachineForcefulDeletion labels a machine object to become forcefully deleted.
func (b *HybridBotanist) markMachineForcefulDeletion(machine machinev1alpha1.Machine) error {
	labels := machine.Labels
	if labels == nil {
		labels = map[string]string{}
	}

	if val, ok := labels["force-deletion"]; ok && val == "True" {
		return nil
	}

	labels["force-deletion"] = "True"
	machine.Labels = labels

	_, err := b.K8sSeedClient.Machine().MachineV1alpha1().Machines(b.Shoot.SeedNamespace).Update(&machine)
	return err
}

// waitUntilMachineDeploymentsAvailable waits for a maximum of 30 minutes until all the desired <machineDeployments>
// were marked as healthy/available by the machine-controller-manager. It polls the status every 5 seconds.
func (b *HybridBotanist) waitUntilMachineDeploymentsAvailable(wantedMachineDeployments operation.MachineDeployments) error {
	return wait.Poll(5*time.Second, 30*time.Minute, func() (bool, error) {
		var numHealthyDeployments, numUpdated, numDesired, numberOfAwakeMachines int32

		// Get the list of all existing machine deployments
		existingMachineDeployments, err := b.K8sSeedClient.Machine().MachineV1alpha1().MachineDeployments(b.Shoot.SeedNamespace).List(metav1.ListOptions{})
		if err != nil {
			return false, err
		}

		// Collect the numbers of ready and desired replicas.
		for _, existingMachineDeployment := range existingMachineDeployments.Items {
			// If the Shoots get hibernated we want to wait until all machine deployments have been deleted entirely.
			if b.Shoot.IsHibernated {
				numberOfAwakeMachines += existingMachineDeployment.Status.Replicas
				continue
			}

			// If the Shoot is not hibernated we want to wait until all machine deployments have been as many ready
			// replicas as desired (specified in the .spec.replicas). However, if we see any error in the status of
			// the deployment then we return it.
			for _, failedMachine := range existingMachineDeployment.Status.FailedMachines {
				return false, fmt.Errorf("Machine %s failed: %s", failedMachine.Name, failedMachine.LastOperation.Description)
			}

			// If the Shoot is not hibernated we want to wait until all machine deployments have been as many ready
			// replicas as desired (specified in the .spec.replicas).
			for _, machineDeployment := range wantedMachineDeployments {
				if machineDeployment.Name == existingMachineDeployment.Name {
					if health.CheckMachineDeployment(&existingMachineDeployment) == nil {
						numHealthyDeployments++
					}
					numDesired += existingMachineDeployment.Spec.Replicas
					numUpdated += existingMachineDeployment.Status.UpdatedReplicas
				}
			}
		}

		switch {
		case !b.Shoot.IsHibernated:
			b.Logger.Infof("Waiting until as many as desired machines are ready (%d/%d machine objects up-to-date, %d/%d machinedeployments available)...", numUpdated, numDesired, numHealthyDeployments, len(wantedMachineDeployments))
			if numUpdated >= numDesired && int(numHealthyDeployments) == len(wantedMachineDeployments) {
				return true, nil
			}
		default:
			if numberOfAwakeMachines == 0 {
				return true, nil
			}
			b.Logger.Infof("Waiting until all machines have been hibernated (%d still awake)...", numberOfAwakeMachines)
		}

		return false, nil
	})
}

// waitUntilMachineResourcesDeleted waits for a maximum of 30 minutes until all machine resources have been properly
// deleted by the machine-controller-manager. It polls the status every 5 seconds.
func (b *HybridBotanist) waitUntilMachineResourcesDeleted(classKind string) error {
	var (
		countMachines            = -1
		countMachineSets         = -1
		countMachineDeployments  = -1
		countMachineClasses      = -1
		countMachineClassSecrets = -1

		listOptions = metav1.ListOptions{}
	)

	return wait.Poll(5*time.Second, 30*time.Minute, func() (bool, error) {
		msg := ""

		// Check whether all machines have been deleted.
		if countMachines != 0 {
			existingMachines, err := b.K8sSeedClient.Machine().MachineV1alpha1().Machines(b.Shoot.SeedNamespace).List(listOptions)
			if err != nil {
				return false, err
			}
			countMachines = len(existingMachines.Items)
			msg += fmt.Sprintf("%d machines, ", countMachines)
		}

		// Check whether all machine sets have been deleted.
		if countMachineSets != 0 {
			existingMachineSets, err := b.K8sSeedClient.Machine().MachineV1alpha1().MachineSets(b.Shoot.SeedNamespace).List(listOptions)
			if err != nil {
				return false, err
			}
			countMachineSets = len(existingMachineSets.Items)
			msg += fmt.Sprintf("%d machine sets, ", countMachineSets)
		}

		// Check whether all machine deployments have been deleted.
		if countMachineDeployments != 0 {
			existingMachineDeployments, err := b.K8sSeedClient.Machine().MachineV1alpha1().MachineDeployments(b.Shoot.SeedNamespace).List(listOptions)
			if err != nil {
				return false, err
			}
			countMachineDeployments = len(existingMachineDeployments.Items)
			msg += fmt.Sprintf("%d machine deployments, ", countMachineDeployments)

			// Check whether an operation failed during the deletion process.
			for _, existingMachineDeployment := range existingMachineDeployments.Items {
				for _, failedMachine := range existingMachineDeployment.Status.FailedMachines {
					return false, fmt.Errorf("Machine %s failed: %s", failedMachine.Name, failedMachine.LastOperation.Description)
				}
			}
		}

		// Check whether all machine classes have been deleted.
		if countMachineClasses != 0 {
			existingMachineClasses, _, err := b.ShootCloudBotanist.ListMachineClasses()
			if err != nil {
				return false, err
			}
			countMachineClasses = existingMachineClasses.Len()
			msg += fmt.Sprintf("%d machine classes, ", countMachineClasses)
		}

		// Check whether all machine class secrets have been deleted.
		if countMachineClassSecrets != 0 {
			count := 0
			existingMachineClassSecrets, err := b.listMachineClassSecrets()
			if err != nil {
				return false, err
			}
			for _, machineClassSecret := range existingMachineClassSecrets.Items {
				if len(machineClassSecret.Finalizers) != 0 {
					count++
				}
			}
			countMachineClassSecrets = count
			msg += fmt.Sprintf("%d machine class secrets, ", countMachineClassSecrets)
		}

		if countMachines != 0 || countMachineSets != 0 || countMachineDeployments != 0 || countMachineClasses != 0 || countMachineClassSecrets != 0 {
			b.Logger.Infof("Waiting until the following machine resources have been processed: %s", strings.TrimSuffix(msg, ", "))
			return false, nil
		}
		return true, nil
	})
}

// cleanupMachineDeployments deletes all machine deployments which are not part of the provided list
// <wantedMachineDeployments>.
func (b *HybridBotanist) cleanupMachineDeployments(existingMachineDeployments *machinev1alpha1.MachineDeploymentList, wantedMachineDeployments operation.MachineDeployments) error {
	for _, existingMachineDeployment := range existingMachineDeployments.Items {
		if !wantedMachineDeployments.ContainsName(existingMachineDeployment.Name) {
			if err := b.K8sSeedClient.Machine().MachineV1alpha1().MachineDeployments(b.Shoot.SeedNamespace).Delete(existingMachineDeployment.Name, &metav1.DeleteOptions{}); err != nil {
				return err
			}
		}
	}
	return nil
}

func (b *HybridBotanist) listMachineClassSecrets() (*corev1.SecretList, error) {
	return b.K8sSeedClient.ListSecrets(b.Shoot.SeedNamespace, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", gardencorev1alpha1.GardenPurpose, gardencorev1alpha1.GardenPurposeMachineClass),
	})
}

// cleanupMachineClassSecrets deletes all unused machine class secrets (i.e., those which are not part
// of the provided list <usedSecrets>.
func (b *HybridBotanist) cleanupMachineClassSecrets(usedSecrets sets.String) error {
	secretList, err := b.listMachineClassSecrets()
	if err != nil {
		return err
	}

	// Cleanup all secrets which were used for machine classes that do not exist anymore.
	for _, secret := range secretList.Items {
		if !usedSecrets.Has(secret.Name) {
			if err := b.K8sSeedClient.DeleteSecret(secret.Namespace, secret.Name); err != nil {
				return err
			}
		}
	}

	return nil
}

// Helper functions

func shootIsWokenUp(isHibernated bool, existingMachineDeployments *machinev1alpha1.MachineDeploymentList) bool {
	if isHibernated {
		return false
	}

	for _, existingMachineDeployment := range existingMachineDeployments.Items {
		if existingMachineDeployment.Spec.Replicas != 0 {
			return false
		}
	}
	return true
}

func getDeploymentSpecReplicas(existingMachineDeployments *machinev1alpha1.MachineDeploymentList, name string) int {
	for _, existingMachineDeployment := range existingMachineDeployments.Items {
		if existingMachineDeployment.Name == name {
			return int(existingMachineDeployment.Spec.Replicas)
		}
	}
	return -1
}

func getExistingMachineDeployment(existingMachineDeployments *machinev1alpha1.MachineDeploymentList, name string) *machinev1alpha1.MachineDeployment {
	for _, machineDeployment := range existingMachineDeployments.Items {
		if machineDeployment.Name == name {
			return &machineDeployment
		}
	}
	return nil
}
