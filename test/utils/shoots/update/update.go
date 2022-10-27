// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package update

import (
	"context"
	"fmt"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/test/framework"
	"github.com/gardener/gardener/test/utils/shoots/access"
	"github.com/gardener/gardener/test/utils/shoots/update/highavailability"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/Masterminds/semver"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
)

func getKAPIAuthToken(ctx context.Context, seedClient kubernetes.Interface, namespace string) string {
	c := seedClient.Client()
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kube-apiserver",
			Namespace: namespace,
		},
	}
	Expect(c.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)).To(Succeed())
	return deployment.Spec.Template.Spec.Containers[0].ReadinessProbe.HTTPGet.HTTPHeaders[0].Value
}

// RunTest runs the update test for an existing shoot cluster. If provided, it updates .spec.kubernetes.version with the
// value of <newControlPlaneKubernetesVersion> and the .kubernetes.version fields of all worker pools which currently
// have the same value as .spec.kubernetes.version. For all worker pools specifying a different version,
// <newWorkerPoolKubernetesVersion> will be used.
// If <newControlPlaneKubernetesVersion> or <newWorkerPoolKubernetesVersion> are nil or empty then the next consecutive
// minor versions will be fetched from the CloudProfile referenced by the shoot.
func RunTest(
	ctx context.Context,
	f *framework.ShootFramework,
	newControlPlaneKubernetesVersion *string,
	newWorkerPoolKubernetesVersion *string,
) {
	By("creating shoot client")
	var (
		job *batchv1.Job
		err error
	)

	shootClient, err := access.CreateShootClientFromAdminKubeconfig(ctx, f.GardenClient, f.Shoot)
	Expect(err).NotTo(HaveOccurred())

	if gardencorev1beta1helper.IsHAControlPlaneConfigured(f.Shoot) {
		f.Seed, f.SeedClient, err = f.GetSeed(ctx, *f.Shoot.Spec.SeedName)
		Expect(err).NotTo(HaveOccurred())
		shootSeedNamespace := f.Shoot.Status.TechnicalID
		job, err = highavailability.DeployZeroDownTimeValidatorJob(ctx, f.SeedClient.Client(), "update", shootSeedNamespace, getKAPIAuthToken(ctx, f.SeedClient, shootSeedNamespace))
		Expect(err).NotTo(HaveOccurred())

		checkJobReady(ctx, f.SeedClient.Client(), job)
		defer func() {
			ExpectWithOffset(1,
				client.IgnoreNotFound(
					f.SeedClient.Client().Delete(ctx,
						job,
						client.PropagationPolicy(metav1.DeletePropagationForeground),
					),
				),
			).To(Succeed())
		}()
	}

	By("verifying the Kubernetes version for all existing nodes matches with the versions defined in the Shoot spec [before update]")
	Expect(verifyKubernetesVersions(ctx, shootClient, f.Shoot)).To(Succeed())

	By("reading CloudProfile")
	cloudProfile, err := f.GetCloudProfile(ctx)
	Expect(err).NotTo(HaveOccurred())

	By("computing new Kubernetes version for control plane and worker pools")
	controlPlaneVersion, poolNameToKubernetesVersion, err := computeNewKubernetesVersions(cloudProfile, f.Shoot, newControlPlaneKubernetesVersion, newWorkerPoolKubernetesVersion)
	Expect(err).NotTo(HaveOccurred())

	if len(controlPlaneVersion) == 0 && len(poolNameToKubernetesVersion) == 0 {
		Skip("shoot already has the desired kubernetes versions")
	}

	By("updating shoot")
	if controlPlaneVersion != "" {
		By("updating .spec.kubernetes.version to " + controlPlaneVersion)
	}
	for poolName, kubernetesVersion := range poolNameToKubernetesVersion {
		By("updating .kubernetes.version to " + kubernetesVersion + " for pool " + poolName)
	}

	Expect(f.UpdateShoot(ctx, func(shoot *gardencorev1beta1.Shoot) error {
		if controlPlaneVersion != "" {
			shoot.Spec.Kubernetes.Version = controlPlaneVersion
		}

		for i, worker := range shoot.Spec.Provider.Workers {
			if workerPoolVersion, ok := poolNameToKubernetesVersion[worker.Name]; ok {
				shoot.Spec.Provider.Workers[i].Kubernetes.Version = &workerPoolVersion
			}
		}

		return nil
	})).To(Succeed())

	By("re-creating shoot client")
	shootClient, err = access.CreateShootClientFromAdminKubeconfig(ctx, f.GardenClient, f.Shoot)
	Expect(err).NotTo(HaveOccurred())

	By("verifying the Kubernetes version for all existing nodes matches with the versions defined in the Shoot spec [after update]")
	Expect(verifyKubernetesVersions(ctx, shootClient, f.Shoot)).To(Succeed())

	if gardencorev1beta1helper.IsHAControlPlaneConfigured(f.Shoot) {
		By("ensuring there is no downtime while updating shoot")
		ExpectWithOffset(1, f.SeedClient.Client().Get(ctx, client.ObjectKeyFromObject(job), job)).To(Succeed())
		ExpectWithOffset(1, job.Status.Failed).Should(BeZero())
	}
}

func verifyKubernetesVersions(ctx context.Context, shootClient kubernetes.Interface, shoot *gardencorev1beta1.Shoot) error {
	controlPlaneKubernetesVersion, err := semver.NewVersion(shoot.Spec.Kubernetes.Version)
	if err != nil {
		return err
	}

	expectedControlPlaneKubernetesVersion := "v" + controlPlaneKubernetesVersion.String()
	if shootClient.Version() != expectedControlPlaneKubernetesVersion {
		return fmt.Errorf("control plane version is %q but expected %q", shootClient.Version(), expectedControlPlaneKubernetesVersion)
	}

	poolNameToKubernetesVersion := map[string]string{}
	for _, worker := range shoot.Spec.Provider.Workers {
		poolKubernetesVersion, err := gardencorev1beta1helper.CalculateEffectiveKubernetesVersion(controlPlaneKubernetesVersion, worker.Kubernetes)
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

		if kubeProxyVersion := node.Status.NodeInfo.KubeProxyVersion; kubeProxyVersion != kubernetesVersion {
			return fmt.Errorf("kube-proxy version of pool %q is %q but expected %q", poolName, kubeProxyVersion, kubernetesVersion)
		}
	}

	return nil
}

func computeNewKubernetesVersions(
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
	consecutiveMinorAvailable, newVersion, err := gardencorev1beta1helper.GetKubernetesVersionForMinorUpdate(cloudProfile, kubernetesVersion)
	if err != nil {
		return "", err
	}

	if !consecutiveMinorAvailable {
		return "", fmt.Errorf("no consecutive minor version available for %q", kubernetesVersion)
	}

	return newVersion, nil
}

// checkJobReady checks k8s job associated pod is ready, up and running.
func checkJobReady(ctx context.Context, cl client.Client, job *batchv1.Job) {
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
	}, time.Minute, time.Second).Should(BeNil())
}
