// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

/**
	Overview
		- Tests PVC autoscaling for Vali and/or VictoriaLogs in the Shoot control plane namespace.
		- Only runs if the seed has PersistentVolumeClaimAutoscaler enabled; otherwise skipped.

	Notes
		- Which backends are active is determined at runtime by checking for the presence of the
		  Vali StatefulSet and the VLSingle resource in the shoot namespace.

	Test steps:
		1. Verify that pvc-autoscaler is running in the seed's garden namespace.
		2. For each active logging backend:
		   a. Record initial PVC size.
		   b. Fill the PVC past the autoscaler threshold.
		   c. Wait for pvc-autoscaler to resize the PVC.
		   d. Clean up the fill files.
**/

package logging

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	victoriametricsv1 "github.com/VictoriaMetrics/operator/api/operator/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	victorialogsConstants "github.com/gardener/gardener/pkg/component/observability/logging/victorialogs/constants"
	"github.com/gardener/gardener/test/framework"
)

const (
	pvcAutoscalerTestTimeout           = 60 * time.Minute
	pvcAutoscalerCleanupTimeout        = 5 * time.Minute
	pvcAutoscalerInitializationTimeout = 5 * time.Minute
	pvcGrowthWaitInterval              = 30 * time.Second
	pvcGrowthWaitTimeout               = 10 * time.Minute

	pvcFillUtilizationPercent = 75
)

type logBackend struct {
	name          string
	pvcLabels     map[string]string
	podLabels     map[string]string
	containerName string
	dataDir       string
	isDeployed    func(ctx context.Context, c client.Client, shootNamespace string) (bool, error)
	waitForReady  func(ctx context.Context, f *framework.ShootFramework) error
}

var _ = Describe("Shoot PVC autoscaling for logging testing", func() {
	shootFramework := framework.NewShootFramework(nil)

	var test = func(backend logBackend, fillFunc func(ctx context.Context, f *framework.ShootFramework, backend logBackend, shootNamespace string, pvcSize resource.Quantity) error) {
		shootFramework.Beta().Serial().CIt(fmt.Sprintf("should scale %s PVC when capacity threshold is exceeded", backend.name), func(ctx context.Context) {
			seedClient := shootFramework.SeedClient.Client()
			shootNamespace := shootFramework.ShootSeedNamespace()

			By(fmt.Sprintf("Check whether %s is deployed", backend.name))
			deployed, err := backend.isDeployed(ctx, seedClient, shootNamespace)
			framework.ExpectNoError(err)
			if !deployed {
				Skip(fmt.Sprintf("%s is not deployed in namespace %s — skipping", backend.name, shootNamespace))
			}

			By(fmt.Sprintf("Wait until %s is ready", backend.name))
			framework.ExpectNoError(backend.waitForReady(ctx, shootFramework))

			By(fmt.Sprintf("Record initial %s PVC storage capacity", backend.name))
			initialPVCSize, pvcName, err := getSizeOfFirstPVC(ctx, seedClient, shootNamespace, backend.pvcLabels)
			framework.ExpectNoError(err)
			shootFramework.Logger.Info("Initial PVC size", "backend", backend.name, "pvc", pvcName, "size", initialPVCSize.String())

			By(fmt.Sprintf("Fill %s PVC to %d%% utilization to trigger pvc-autoscaler", backend.name, pvcFillUtilizationPercent))
			Eventually(func() error {
				return fillFunc(ctx, shootFramework, backend, shootNamespace, initialPVCSize)
			}).WithTimeout(time.Minute * 1).WithPolling(time.Second * 10).WithContext(ctx).Should(Succeed())

			By(fmt.Sprintf("Wait for pvc-autoscaler to grow the %s PVC", backend.name))
			waitUntilPVCGrows(ctx, seedClient, shootNamespace, backend.pvcLabels, initialPVCSize)

			By(fmt.Sprintf("Wait until %s pod's filesystem reflects the new PVC size", backend.name))
			waitUntilFilesystemExpands(ctx, shootFramework, backend, shootNamespace, initialPVCSize)

			By("Clean up fill files")
			Eventually(func() error {
				_, _, err = framework.PodExecByLabel(ctx, shootFramework.SeedClient, shootNamespace, labels.SelectorFromSet(backend.podLabels), backend.containerName,
					"sh", "-c", fmt.Sprintf("rm -rf %s/fill-capacity", backend.dataDir),
				)
				return err
			}).WithTimeout(time.Minute * 1).WithPolling(time.Second * 10).WithContext(ctx).Should(Succeed())
		}, pvcAutoscalerTestTimeout, framework.WithCAfterTest(func(ctx context.Context) {
			Eventually(func() error {
				_, _, err := framework.PodExecByLabel(ctx, shootFramework.SeedClient, shootFramework.ShootSeedNamespace(), labels.SelectorFromSet(backend.podLabels), backend.containerName,
					"sh", "-c", fmt.Sprintf("rm -rf %s/fill-capacity", backend.dataDir),
				)
				return err
			}).WithTimeout(time.Minute * 1).WithPolling(time.Second * 10).WithContext(ctx).Should(Succeed())
		}, pvcAutoscalerCleanupTimeout))
	}

	framework.CBeforeEach(func(ctx context.Context) {
		settings := shootFramework.Seed.Spec.Settings
		if settings == nil || settings.PersistentVolumeClaimAutoscaler == nil || !settings.PersistentVolumeClaimAutoscaler.Enabled {
			Skip("PVC autoscaler is not enabled for this seed — skipping PVC autoscaler logging test")
		}

		checkRequiredResources(ctx, shootFramework.SeedClient)

		By("Verify pvc-autoscaler is running in the seed garden namespace")
		framework.ExpectNoError(shootFramework.WaitUntilDeploymentIsReady(ctx, v1beta1constants.DeploymentNamePVCAutoscaler, v1beta1constants.GardenNamespace, shootFramework.SeedClient))
	}, pvcAutoscalerInitializationTimeout)

	// TODO(plkokanov): Remove the Vali backend test once the `RemoveVali` featuregate has been promoted to GA.
	test(valiBackend(), fillCapacity)
	test(victoriaLogsBackend(), fillCapacity)
})

func fillCapacity(ctx context.Context, f *framework.ShootFramework, backend logBackend, shootNamespace string, pvcSize resource.Quantity) error {
	fillMiB := pvcSize.Value() * int64(pvcFillUtilizationPercent) / 100 / (1024 * 1024)
	f.Logger.Info("Filling PVC capacity via dd", "backend", backend.name, "pvcSize", pvcSize.String(), "fillMiB", fillMiB)
	script := fmt.Sprintf(
		"mkdir -p %s/fill-capacity && dd if=/dev/zero of=%s/fill-capacity/fill bs=1M count=%d",
		backend.dataDir, backend.dataDir, fillMiB,
	)

	_, _, err := framework.PodExecByLabel(ctx, f.SeedClient, shootNamespace,
		labels.SelectorFromSet(backend.podLabels), backend.containerName,
		"sh", "-c", script,
	)
	return err
}

func valiBackend() logBackend {
	return logBackend{
		name:          valiName,
		pvcLabels:     map[string]string{v1beta1constants.LabelApp: valiName},
		podLabels:     valiLabels,
		containerName: valiName,
		dataDir:       "/data",
		isDeployed: func(ctx context.Context, c client.Client, shootNamespace string) (bool, error) {
			sts := &appsv1.StatefulSet{}
			if err := c.Get(ctx, types.NamespacedName{Namespace: shootNamespace, Name: valiName}, sts); err != nil {
				return false, client.IgnoreNotFound(err)
			}
			return true, nil
		},
		waitForReady: func(ctx context.Context, f *framework.ShootFramework) error {
			return f.WaitUntilStatefulSetIsRunning(ctx, valiName, f.ShootSeedNamespace(), f.SeedClient)
		},
	}
}

func victoriaLogsBackend() logBackend {
	return logBackend{
		name: victorialogsConstants.VLSingleResourceName,
		pvcLabels: map[string]string{
			"app.kubernetes.io/name":     "vlsingle",
			"app.kubernetes.io/instance": victorialogsConstants.VLSingleResourceName,
		},
		podLabels: map[string]string{
			v1beta1constants.LabelApp:  victorialogsConstants.VLSingleResourceName,
			v1beta1constants.LabelRole: v1beta1constants.LabelObservability,
		},
		containerName: "vlsingle",
		dataDir:       "/victoria-logs-data",
		isDeployed: func(ctx context.Context, c client.Client, shootNamespace string) (bool, error) {
			vlSingle := &victoriametricsv1.VLSingle{}
			if err := c.Get(ctx, types.NamespacedName{Namespace: shootNamespace, Name: victorialogsConstants.VLSingleResourceName}, vlSingle); err != nil {
				return false, client.IgnoreNotFound(err)
			}
			return true, nil
		},
		waitForReady: func(ctx context.Context, f *framework.ShootFramework) error {
			return f.WaitUntilDeploymentIsReady(ctx, "vlsingle-"+victorialogsConstants.VLSingleResourceName, f.ShootSeedNamespace(), f.SeedClient)
		},
	}
}

func getSizeOfFirstPVC(ctx context.Context, c client.Client, namespace string, matchLabels map[string]string) (resource.Quantity, string, error) {
	pvcList := &corev1.PersistentVolumeClaimList{}
	if err := c.List(ctx, pvcList, client.InNamespace(namespace), client.MatchingLabels(matchLabels)); err != nil {
		return resource.Quantity{}, "", fmt.Errorf("failed to list PVCs in namespace %q with labels %v: %w", namespace, matchLabels, err)
	}
	if len(pvcList.Items) == 0 {
		return resource.Quantity{}, "", fmt.Errorf("no PVC found in namespace %q matching labels %v", namespace, matchLabels)
	}

	pvc := pvcList.Items[0]
	storageRequest, ok := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
	if !ok {
		return resource.Quantity{}, "", fmt.Errorf("PVC %s/%s has no storage request", pvc.Namespace, pvc.Name)
	}
	return storageRequest, pvc.Name, nil
}

func waitUntilFilesystemExpands(ctx context.Context, f *framework.ShootFramework, backend logBackend, shootNamespace string, initialPVCSize resource.Quantity) {
	Eventually(func(g Gomega) {
		stdout, _, err := framework.PodExecByLabel(ctx, f.SeedClient, shootNamespace, labels.SelectorFromSet(backend.podLabels), backend.containerName,
			"df", "-B1", backend.dataDir,
		)
		g.Expect(err).NotTo(HaveOccurred(), "df failed")

		var buf bytes.Buffer
		_, err = buf.ReadFrom(stdout)
		g.Expect(err).NotTo(HaveOccurred())

		scanner := bufio.NewScanner(&buf)
		scanner.Scan() // skip df header line
		g.Expect(scanner.Scan()).To(BeTrue(), "df output missing data line")

		fields := strings.Fields(scanner.Text())
		g.Expect(len(fields)).To(BeNumerically(">=", 2), "unexpected df fields: %v", fields)

		fsBytes, err := strconv.ParseInt(fields[1], 10, 64)
		g.Expect(err).NotTo(HaveOccurred(), "parsing df size %q", fields[1])

		initialBytes := initialPVCSize.Value()
		f.Logger.Info("Filesystem size observed in pod", "backend", backend.name, "fsBytes", fsBytes, "initialBytes", initialBytes)
		g.Expect(fsBytes).To(BeNumerically(">", initialBytes), "filesystem not yet expanded (observed: %d B, initial: %d B)", fsBytes, initialBytes)
	}).WithPolling(pvcGrowthWaitInterval).WithTimeout(pvcGrowthWaitTimeout).WithContext(ctx).Should(Succeed())
}

func waitUntilPVCGrows(ctx context.Context, c client.Client, namespace string, matchLabels map[string]string, initialSize resource.Quantity) {
	Eventually(func(g Gomega) {
		currentSize, _, err := getSizeOfFirstPVC(ctx, c, namespace, matchLabels)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(currentSize.Value()).To(BeNumerically(">", initialSize.Value()), "PVC has not grown yet (current: %s, initial: %s)", currentSize.String(), initialSize.String())
	}).WithPolling(pvcGrowthWaitInterval).WithTimeout(pvcGrowthWaitTimeout).WithContext(ctx).Should(Succeed())
}
