// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
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
