// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package update

import (
	"context"
	"fmt"
	"time"

	"github.com/Masterminds/semver/v3"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllermanager/controller/shoot/maintenance/helper"
	nodeagentconfigv1alpha1 "github.com/gardener/gardener/pkg/nodeagent/apis/config/v1alpha1"
	versionutils "github.com/gardener/gardener/pkg/utils/version"
)

// GetKubeAPIServerAuthToken returns kube API server auth token for given shoot's control-plane namespace in seed cluster.
func GetKubeAPIServerAuthToken(ctx context.Context, seedClient client.Client, namespace string) string {
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      v1beta1constants.DeploymentNameKubeAPIServer,
			Namespace: namespace,
		},
	}
	Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)).To(Succeed())
	return deployment.Spec.Template.Spec.Containers[0].ReadinessProbe.HTTPGet.HTTPHeaders[0].Value
}

// VerifyKubernetesVersions verifies that the Kubernetes versions of the control plane and worker nodes
// match the versions defined in the shoot spec.
func VerifyKubernetesVersions(ctx context.Context, shootClient kubernetes.Interface, shoot *gardencorev1beta1.Shoot) error {
	controlPlaneKubernetesVersion, err := semver.NewVersion(shoot.Spec.Kubernetes.Version)
	if err != nil {
		return err
	}

	expectedControlPlaneKubernetesVersion := "v" + controlPlaneKubernetesVersion.String()
	if shootClient.Version() != expectedControlPlaneKubernetesVersion {
		return fmt.Errorf("control plane version is %q but expected %q", shootClient.Version(), expectedControlPlaneKubernetesVersion)
	}

	poolNameToKubernetesVersion := make(map[string]string, len(shoot.Spec.Provider.Workers))
	for _, worker := range shoot.Spec.Provider.Workers {
		poolKubernetesVersion, err := v1beta1helper.CalculateEffectiveKubernetesVersion(controlPlaneKubernetesVersion, worker.Kubernetes)
		if err != nil {
			return fmt.Errorf("error when calculating effective Kubernetes version of pool %q: %w", worker.Name, err)
		}
		poolNameToKubernetesVersion[worker.Name] = "v" + poolKubernetesVersion.String()
	}

	nodeList := &corev1.NodeList{}
	if err := shootClient.Client().List(ctx, nodeList); err != nil {
		return err
	}

	for _, node := range nodeList.Items {
		var (
			poolName          = node.Labels[v1beta1constants.LabelWorkerPool]
			kubernetesVersion = poolNameToKubernetesVersion[poolName]
		)

		if kubeletVersion := node.Status.NodeInfo.KubeletVersion; kubeletVersion != kubernetesVersion {
			return fmt.Errorf("kubelet version of pool %q is %q but expected %q", poolName, kubeletVersion, kubernetesVersion)
		}
	}

	return nil
}

// ComputeNewKubernetesVersions computes the new Kubernetes versions for the control plane and worker pools of the given shoot.
func ComputeNewKubernetesVersions(
	cloudProfile *gardencorev1beta1.CloudProfile,
	shoot *gardencorev1beta1.Shoot,
	newControlPlaneKubernetesVersion *string,
	newWorkerPoolKubernetesVersion *string,
) (
	controlPlaneKubernetesVersion string,
	poolNameToKubernetesVersion map[string]string,
	err error,
) {
	if newControlPlaneKubernetesVersion != nil && *newControlPlaneKubernetesVersion != "" {
		controlPlaneKubernetesVersion = *newControlPlaneKubernetesVersion
	} else {
		controlPlaneKubernetesVersion, err = getNextConsecutiveMinorVersion(cloudProfile, shoot.Spec.Kubernetes.Version)
		if err != nil {
			return "", nil, err
		}
	}

	// if current version is already the same as the new version then reset it
	if shoot.Spec.Kubernetes.Version == controlPlaneKubernetesVersion {
		controlPlaneKubernetesVersion = ""
	}

	poolNameToKubernetesVersion = make(map[string]string, len(shoot.Spec.Provider.Workers))
	for _, worker := range shoot.Spec.Provider.Workers {
		// worker does not override version
		if worker.Kubernetes == nil || worker.Kubernetes.Version == nil {
			continue
		}

		if *worker.Kubernetes.Version == shoot.Spec.Kubernetes.Version {
			// worker overrides version and it's equal to the control plane version
			poolNameToKubernetesVersion[worker.Name] = controlPlaneKubernetesVersion
			continue
		}

		// worker overrides version and it's not equal to the control plane version
		if newWorkerPoolKubernetesVersion != nil && *newWorkerPoolKubernetesVersion != "" {
			poolNameToKubernetesVersion[worker.Name] = *newWorkerPoolKubernetesVersion
		} else {
			poolNameToKubernetesVersion[worker.Name], err = getNextConsecutiveMinorVersion(cloudProfile, *worker.Kubernetes.Version)
			if err != nil {
				return "", nil, err
			}
		}

		// if current version is already the same as the new version then reset it
		if *worker.Kubernetes.Version == poolNameToKubernetesVersion[worker.Name] {
			delete(poolNameToKubernetesVersion, worker.Name)
		}
	}

	return
}

func getNextConsecutiveMinorVersion(cloudProfile *gardencorev1beta1.CloudProfile, kubernetesVersion string) (string, error) {
	consecutiveMinorAvailable, newVersion, err := v1beta1helper.GetVersionForForcefulUpdateToConsecutiveMinor(cloudProfile.Spec.Kubernetes.Versions, kubernetesVersion)
	if err != nil {
		return "", err
	}

	if !consecutiveMinorAvailable {
		return "", fmt.Errorf("no consecutive minor version available for %q", kubernetesVersion)
	}

	return newVersion, nil
}

// WaitForJobToBeReady waits until given job's associated pod is ready.
func WaitForJobToBeReady(ctx context.Context, cl client.Client, job *batchv1.Job) {
	r, _ := labels.NewRequirement("job-name", selection.Equals, []string{job.Name})
	opts := &client.ListOptions{
		LabelSelector: labels.NewSelector().Add(*r),
		Namespace:     job.Namespace,
	}

	EventuallyWithOffset(1, func() error {
		podList := &corev1.PodList{}
		if err := cl.List(ctx, podList, opts); err != nil {
			return fmt.Errorf("error occurred while getting pod object: %v ", err)
		}

		if len(podList.Items) == 0 {
			return fmt.Errorf("job %s associated pod is not scheduled", job.Name)
		}

		for _, c := range podList.Items[0].Status.Conditions {
			if c.Type == corev1.PodReady && c.Status == corev1.ConditionTrue {
				return nil
			}
		}
		return fmt.Errorf("waiting for pod %v to be ready", podList.Items[0].Name)
	}, time.Minute, time.Second).Should(Succeed())
}

// VerifyMachineImageVersions checks if the machine image versions of all worker nodes are up-to-date.
func VerifyMachineImageVersions(ctx context.Context, shootClient kubernetes.Interface, shoot *gardencorev1beta1.Shoot) error {
	poolNameToMachineImageVersion := make(map[string]string, len(shoot.Spec.Provider.Workers))
	for _, worker := range shoot.Spec.Provider.Workers {
		if v1beta1helper.IsUpdateStrategyInPlace(worker.UpdateStrategy) {
			poolNameToMachineImageVersion[worker.Name] = *worker.Machine.Image.Version
		}
	}

	nodeList := &corev1.NodeList{}
	if err := shootClient.Client().List(ctx, nodeList); err != nil {
		return err
	}

	for _, node := range nodeList.Items {
		var (
			poolName = node.Labels[v1beta1constants.LabelWorkerPool]
		)

		machineImageVersion, ok := poolNameToMachineImageVersion[poolName]
		if !ok {
			// not a in-place update worker pool, skip
			continue
		}

		version := nodeagentconfigv1alpha1.OSVersionRegex.FindString(node.Status.NodeInfo.OSImage)
		if version == "" {
			return fmt.Errorf("unable to find os version in %q with regex: %s for node %q of pool %q", node.Status.NodeInfo.OSImage, nodeagentconfigv1alpha1.OSVersionRegex.String(), node.Name, poolName)
		}

		if machineImageVersionUpToDate, err := versionutils.CompareVersions(machineImageVersion, "=", version); err != nil {
			return fmt.Errorf("failed comparing current machine image version %q with machine image version in the node status %q for node %q of pool %q: %w", machineImageVersion, version, node.Name, poolName, err)
		} else if !machineImageVersionUpToDate {
			return fmt.Errorf("machine image version of node %q of pool %q is %q but expected %q", node.Name, poolName, version, machineImageVersion)
		}
	}

	return nil
}

// ComputeNewMachineImageVersions computes the new machine image versions for the worker pools of the given shoot.
func ComputeNewMachineImageVersions(
	cloudProfile *gardencorev1beta1.CloudProfile,
	shoot *gardencorev1beta1.Shoot,
	newWorkerPoolMachineImageVersion *string,
) (
	poolNameToMachineImageVersion map[string]string,
	err error,
) {
	controlPlaneVersion, err := semver.NewVersion(shoot.Spec.Kubernetes.Version)
	if err != nil {
		return nil, err
	}

	poolNameToMachineImageVersion = make(map[string]string, len(shoot.Spec.Provider.Workers))
	for _, worker := range shoot.Spec.Provider.Workers {
		if !v1beta1helper.IsUpdateStrategyInPlace(worker.UpdateStrategy) {
			continue
		}

		if newWorkerPoolMachineImageVersion != nil && *newWorkerPoolMachineImageVersion != "" {
			poolNameToMachineImageVersion[worker.Name] = *newWorkerPoolMachineImageVersion
			continue
		}

		poolNameToMachineImageVersion[worker.Name], err = getNextConsecutiveMachineImageVersion(cloudProfile, worker, controlPlaneVersion)
		if err != nil {
			return nil, fmt.Errorf("error when getting next consecutive machine image version for worker pool %q: %w", worker.Name, err)
		}
	}

	return
}

func getNextConsecutiveMachineImageVersion(cloudProfile *gardencorev1beta1.CloudProfile, worker gardencorev1beta1.Worker, controlPlaneVersion *semver.Version) (string, error) {
	machineTypeFromCloudProfile := v1beta1helper.FindMachineTypeByName(cloudProfile.Spec.MachineTypes, worker.Machine.Type)
	if machineTypeFromCloudProfile == nil {
		return "", fmt.Errorf("machine type %q of worker %q does not exist in cloudprofile", worker.Machine.Type, worker.Name)
	}

	machineImageFromCloudProfile, err := helper.DetermineMachineImage(cloudProfile, worker.Machine.Image)
	if err != nil {
		return "", err
	}

	kubeletVersion, err := v1beta1helper.CalculateEffectiveKubernetesVersion(controlPlaneVersion, worker.Kubernetes)
	if err != nil {
		return "", err
	}

	filteredMachineImageVersionsFromCloudProfile := helper.FilterMachineImageVersions(&machineImageFromCloudProfile, worker, kubeletVersion, machineTypeFromCloudProfile, cloudProfile.Spec.Capabilities)

	// Always pass true for the value isExpired, because we want to get the next consecutive version regardless of whether the current version is expired or not.
	newMachineImageVersion, err := helper.DetermineMachineImageVersion(worker.Machine.Image, filteredMachineImageVersionsFromCloudProfile, true)
	if err != nil {
		return "", err
	}

	if newMachineImageVersion == "" {
		return "", fmt.Errorf("no consecutive OS version available for worker pool %q", worker.Name)
	}

	return newMachineImageVersion, nil
}
