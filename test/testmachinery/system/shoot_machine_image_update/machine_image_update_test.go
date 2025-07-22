// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

/**
	Overview
		- Tests the update of a Shoot's Worker Pool Machine Image version to the next supported version

	Prerequisites
		- A Shoot exists.

	Test: Update the Shoot's Worker Pool Machine Image version to the next supported version
	Expected Output
		- Successful reconciliation of the Shoot after the Worker Pool Machine Image Version update.
 **/

package shootmachineimageupdate_test

import (
	"context"
	"flag"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/test/framework"
	"github.com/gardener/gardener/test/utils/access"
	shootupdatesuite "github.com/gardener/gardener/test/utils/shoots/update"
	"github.com/gardener/gardener/test/utils/shoots/update/highavailability"
	"github.com/gardener/gardener/test/utils/shoots/update/inplace"
)

var newWorkerPoolMachineImageVersion = flag.String("version-worker-pools", "", "the version to use for .spec.provider.workers[].machine.image.version")

const UpdateMachineImageVersionTimeout = 45 * time.Minute

func init() {
	framework.RegisterShootFrameworkFlags()
}

var _ = Describe("Shoot machine image update testing", func() {
	f := framework.NewShootFramework(nil)

	framework.CIt("should update machine image version for worker pools with in-place update strategy", func(ctx context.Context) {
		RunTest(ctx, f, newWorkerPoolMachineImageVersion)
	}, UpdateMachineImageVersionTimeout)
})

// RunTest runs the update test for an existing shoot cluster. If provided, it updates the worker pools with the specified machine image version.
// It verifies that the machine image version is updated without rolling the nodes for in-place update workers.
func RunTest(
	ctx context.Context,
	f *framework.ShootFramework,
	newWorkerPoolMachineImageVersion *string,
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

	var hasAutoInPlaceUpdateWorkers, hasManualInPlaceUpdateWorkers, hasInPlaceUpdateWorkers bool

	for _, worker := range f.Shoot.Spec.Provider.Workers {
		switch ptr.Deref(worker.UpdateStrategy, "") {
		case gardencorev1beta1.AutoInPlaceUpdate:
			hasAutoInPlaceUpdateWorkers = true
		case gardencorev1beta1.ManualInPlaceUpdate:
			hasManualInPlaceUpdateWorkers = true
		}
	}

	hasInPlaceUpdateWorkers = hasAutoInPlaceUpdateWorkers || hasManualInPlaceUpdateWorkers

	// Verify that the test has at least one worker pool with in-place update strategy
	// OS update test is only relevant for in-place update workers to ensure that the OS version is updated
	// without rolling the nodes
	Expect(hasInPlaceUpdateWorkers).To(BeTrue(), "the test requires at least one worker pool with in-place update strategy")

	By("Verify the machine image version for all existing nodes matches with the versions defined in the Shoot spec [before update]")
	Expect(shootupdatesuite.VerifyMachineImageVersions(ctx, shootClient, f.Shoot)).To(Succeed())

	By("Read CloudProfile")
	cloudProfile, err := f.GetCloudProfile(ctx)
	Expect(err).NotTo(HaveOccurred())

	poolNameToMachineImageVersion, err := shootupdatesuite.ComputeNewMachineImageVersions(cloudProfile, f.Shoot, newWorkerPoolMachineImageVersion)
	Expect(err).NotTo(HaveOccurred())

	By("Update shoot")
	for poolName, machineImageVersion := range poolNameToMachineImageVersion {
		By("Update .spec.provider.workers[].machine.image.version to " + machineImageVersion + " for pool " + poolName)
	}

	var nodesOfInPlaceWorkersBeforeTest sets.Set[string]
	if hasInPlaceUpdateWorkers {
		nodesOfInPlaceWorkersBeforeTest = inplace.FindNodesOfInPlaceWorkers(ctx, f.Logger, f.ShootClient.Client(), f.Shoot)
	}

	Expect(f.UpdateShootSpec(ctx, f.Shoot, func(shoot *gardencorev1beta1.Shoot) error {
		for i, worker := range shoot.Spec.Provider.Workers {
			if workerMachineImageVersion, ok := poolNameToMachineImageVersion[worker.Name]; ok {
				shoot.Spec.Provider.Workers[i].Machine.Image.Version = &workerMachineImageVersion
			}
		}

		return nil
	})).To(Succeed())

	if hasInPlaceUpdateWorkers {
		inplace.VerifyInPlaceUpdateStart(ctx, f.Logger, f.GardenClient.Client(), f.Shoot, hasAutoInPlaceUpdateWorkers, hasManualInPlaceUpdateWorkers)
		if hasManualInPlaceUpdateWorkers {
			inplace.LabelManualInPlaceNodesWithSelectedForUpdate(ctx, f.Logger, f.ShootClient.Client(), f.Shoot)
		}
	}

	err = f.WaitForShootToBeReconciled(ctx, f.Shoot)
	Expect(err).NotTo(HaveOccurred())

	By("Re-creating shoot client")
	shootClient, err = access.CreateShootClientFromAdminKubeconfig(ctx, f.GardenClient, f.Shoot)
	Expect(err).NotTo(HaveOccurred())

	if hasInPlaceUpdateWorkers {
		nodesOfInPlaceWorkersAfterTest := inplace.FindNodesOfInPlaceWorkers(ctx, f.Logger, f.ShootClient.Client(), f.Shoot)
		Expect(nodesOfInPlaceWorkersBeforeTest.UnsortedList()).To(ConsistOf(nodesOfInPlaceWorkersAfterTest.UnsortedList()))

		inplace.VerifyInPlaceUpdateCompletion(ctx, f.Logger, f.GardenClient.Client(), f.Shoot)
	}

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
