// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

/**
	Overview
		- Tests the update of a Shoot's Kubernetes version to the next minor version

	Prerequisites
		- A Shoot exists.

	Test: Update the Shoot's Kubernetes version to the next minor version
	Expected Output
		- Successful reconciliation of the Shoot after the Kubernetes Version update.
 **/

package shootupdate_test

import (
	"context"
	"flag"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/test/framework"
	"github.com/gardener/gardener/test/utils/access"
	shootupdatesuite "github.com/gardener/gardener/test/utils/shoots/update"
	"github.com/gardener/gardener/test/utils/shoots/update/highavailability"
)

var (
	newControlPlaneKubernetesVersion = flag.String("version", "", "the version to use for .spec.kubernetes.version and .spec.provider.workers[].kubernetes.version (only when nil or equal to .spec.kubernetes.version)")
	newWorkerPoolKubernetesVersion   = flag.String("version-worker-pools", "", "the version to use for .spec.provider.workers[].kubernetes.version (only when not equal to .spec.kubernetes.version)")
)

const UpdateKubernetesVersionTimeout = 45 * time.Minute

func init() {
	framework.RegisterShootFrameworkFlags()
}

var _ = Describe("Shoot update testing", func() {
	f := framework.NewShootFramework(nil)

	framework.CIt("should update the kubernetes version of the shoot and its worker pools to the respective next versions", func(ctx context.Context) {
		RunTest(ctx, f, newControlPlaneKubernetesVersion, newWorkerPoolKubernetesVersion)
	}, UpdateKubernetesVersionTimeout)
})

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
	By("Create shoot client")
	var (
		job *batchv1.Job
		err error
	)

	shootClient, err := access.CreateShootClientFromAdminKubeconfig(ctx, f.GardenClient, f.Shoot)
	Expect(err).NotTo(HaveOccurred())

	if v1beta1helper.IsHAControlPlaneConfigured(f.Shoot) {
		f.Seed, f.SeedClient, err = f.GetSeed(ctx, *f.Shoot.Spec.SeedName)
		Expect(err).NotTo(HaveOccurred())
		controlPlaneNamespace := f.Shoot.Status.TechnicalID

		By("Deploy zero-downtime validator job")
		job, err = highavailability.DeployZeroDownTimeValidatorJob(ctx,
			f.SeedClient.Client(), "update", controlPlaneNamespace, shootupdatesuite.GetKubeAPIServerAuthToken(ctx, f.SeedClient.Client(), controlPlaneNamespace))
		Expect(err).NotTo(HaveOccurred())
		shootupdatesuite.WaitForJobToBeReady(ctx, f.SeedClient.Client(), job)
	}

	By("Verify the Kubernetes version for all existing nodes matches with the versions defined in the Shoot spec [before update]")
	Expect(shootupdatesuite.VerifyKubernetesVersions(ctx, shootClient, f.Shoot)).To(Succeed())

	By("Read CloudProfile")
	cloudProfile, err := f.GetCloudProfile(ctx)
	Expect(err).NotTo(HaveOccurred())

	By("Compute new Kubernetes version for control plane and worker pools")
	controlPlaneVersion, poolNameToKubernetesVersion, err := shootupdatesuite.ComputeNewKubernetesVersions(cloudProfile, f.Shoot, newControlPlaneKubernetesVersion, newWorkerPoolKubernetesVersion)
	Expect(err).NotTo(HaveOccurred())

	if len(controlPlaneVersion) == 0 && len(poolNameToKubernetesVersion) == 0 {
		Skip("shoot already has the desired kubernetes versions")
	}

	By("Update shoot")
	if controlPlaneVersion != "" {
		By("Update .spec.kubernetes.version to " + controlPlaneVersion)
	}
	for poolName, kubernetesVersion := range poolNameToKubernetesVersion {
		By("Update .kubernetes.version to " + kubernetesVersion + " for pool " + poolName)
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

	By("Re-creating shoot client")
	shootClient, err = access.CreateShootClientFromAdminKubeconfig(ctx, f.GardenClient, f.Shoot)
	Expect(err).NotTo(HaveOccurred())

	By("Verify the Kubernetes version for all existing nodes matches with the versions defined in the Shoot spec [after update]")
	Expect(shootupdatesuite.VerifyKubernetesVersions(ctx, shootClient, f.Shoot)).To(Succeed())

	if v1beta1helper.IsHAControlPlaneConfigured(f.Shoot) {
		By("Ensure there was no downtime while upgrading shoot")
		Expect(f.SeedClient.Client().Get(ctx, client.ObjectKeyFromObject(job), job)).To(Succeed())
		Expect(job.Status.Failed).Should(BeZero())
		Expect(client.IgnoreNotFound(
			f.SeedClient.Client().Delete(ctx,
				job,
				client.PropagationPolicy(metav1.DeletePropagationForeground),
			),
		),
		).To(Succeed())
	}
}
