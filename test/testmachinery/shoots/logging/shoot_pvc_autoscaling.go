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
	"k8s.io/apimachinery/pkg/util/uuid"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	victorialogsConstants "github.com/gardener/gardener/pkg/component/observability/logging/victorialogs/constants"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/test/framework"
)

const (
	pvcAutoscalerTestTimeout           = 60 * time.Minute
	pvcAutoscalerCleanupTimeout        = 5 * time.Minute
	pvcAutoscalerInitializationTimeout = 5 * time.Minute
	pvcGrowthWaitInterval              = 30 * time.Second
	pvcGrowthWaitTimeout               = 10 * time.Minute

	pvcFillUtilizationPercent = 75

	debugContainerNamePrefix = "pvc-autoscaler-test-debug"
	debugContainerImage      = "registry.k8s.io/e2e-test-images/busybox:1.36.1-1"
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

	var test = func(backend logBackend, fillFunc func(ctx context.Context, f *framework.ShootFramework, backend logBackend, shootNamespace, debugContainerName string, pvcSize resource.Quantity) error) {
		var debugContainerName string

		shootFramework.Beta().Serial().CIt(fmt.Sprintf("should scale %s PVC when capacity threshold is exceeded", backend.name), func(ctx context.Context) {
			seedClient := shootFramework.SeedClient.Client()
			shootNamespace := shootFramework.ShootSeedNamespace()

			debugContainerName = debugContainerNamePrefix + "-" + utils.ComputeSHA256Hex([]byte(uuid.NewUUID()))[:5]

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
				return fillFunc(ctx, shootFramework, backend, shootNamespace, debugContainerName, initialPVCSize)
			}).WithTimeout(time.Minute * 1).WithPolling(time.Second * 10).WithContext(ctx).Should(Succeed())

			By(fmt.Sprintf("Wait for pvc-autoscaler to grow the %s PVC", backend.name))
			waitUntilPVCGrows(ctx, seedClient, shootNamespace, backend.pvcLabels, initialPVCSize)

			By(fmt.Sprintf("Wait until %s pod's filesystem reflects the new PVC size", backend.name))
			waitUntilFilesystemExpands(ctx, shootFramework, backend, shootNamespace, debugContainerName, initialPVCSize)

			By("Clean up fill files")
			Eventually(func() error {
				return cleanupFillFiles(ctx, shootFramework, backend, shootNamespace, debugContainerName)
			}).WithTimeout(time.Minute * 1).WithPolling(time.Second * 10).WithContext(ctx).Should(Succeed())
		}, pvcAutoscalerTestTimeout, framework.WithCAfterTest(func(ctx context.Context) {
			if debugContainerName == "" {
				return
			}
			Eventually(func() error {
				return cleanupFillFiles(ctx, shootFramework, backend, shootFramework.ShootSeedNamespace(), debugContainerName)
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

// fillCapacity fills the backend's PVC past the autoscaler threshold to trigger pvc-autoscaler.
func fillCapacity(ctx context.Context, f *framework.ShootFramework, backend logBackend, shootNamespace, debugContainerName string, pvcSize resource.Quantity) error {
	// Ensure the debug container as vpa can restart the backend pod.
	podName, err := ensureDebugContainer(ctx, f, backend, shootNamespace, debugContainerName)
	if err != nil {
		return err
	}

	fillMiB := pvcSize.Value() * int64(pvcFillUtilizationPercent) / 100 / (1024 * 1024)
	f.Logger.Info("Filling PVC capacity via dd", "backend", backend.name, "pvcSize", pvcSize.String(), "fillMiB", fillMiB)
	script := fmt.Sprintf(
		"mkdir -p %s/fill-capacity && dd if=/dev/zero of=%s/fill-capacity/fill bs=1M count=%d",
		backend.dataDir, backend.dataDir, fillMiB,
	)

	_, _, err = f.SeedClient.PodExecutor().Execute(ctx, shootNamespace, podName, debugContainerName, "sh", "-c", script)
	return err
}

// cleanupFillFiles removes the files written by fillCapacity from the backend's PVC.
func cleanupFillFiles(ctx context.Context, f *framework.ShootFramework, backend logBackend, shootNamespace, debugContainerName string) error {
	// Ensure the debug container as vpa can restart the backend pod.
	podName, err := ensureDebugContainer(ctx, f, backend, shootNamespace, debugContainerName)
	if err != nil {
		return err
	}
	f.Logger.Info("Cleaning up test files that were used to fill capacity", "backend", backend.name)
	_, _, err = f.SeedClient.PodExecutor().Execute(ctx, shootNamespace, podName, debugContainerName,
		"sh", "-c", fmt.Sprintf("rm -rf %s/fill-capacity", backend.dataDir),
	)
	return err
}

// ensureDebugContainer makes sure an ephemeral debug container with the given name is running in the backend
// pod, mounting the same data volume as the backend container.
func ensureDebugContainer(ctx context.Context, f *framework.ShootFramework, backend logBackend, shootNamespace, debugContainerName string) (string, error) {
	pod, err := framework.GetFirstRunningPodWithLabels(ctx, labels.SelectorFromSet(backend.podLabels), shootNamespace, f.SeedClient)
	if err != nil {
		return "", fmt.Errorf("failed to find running pod for backend %q in namespace %q: %w", backend.name, shootNamespace, err)
	}

	dataVolumeName, err := findDataVolumeName(pod, backend)
	if err != nil {
		return "", err
	}

	f.Logger.Info("Ensuring ephemeral debug container in backend pod", "backend", backend.name, "pod", pod.Name, "container", debugContainerName, "dataVolume", dataVolumeName)

	patch := client.StrategicMergeFrom(pod.DeepCopy())
	pod.Spec.EphemeralContainers = append(pod.Spec.EphemeralContainers, corev1.EphemeralContainer{
		EphemeralContainerCommon: corev1.EphemeralContainerCommon{
			Name:            debugContainerName,
			Image:           debugContainerImage,
			ImagePullPolicy: corev1.PullIfNotPresent,
			Command:         []string{"sleep", "3600"},
			VolumeMounts: []corev1.VolumeMount{{
				Name:      dataVolumeName,
				MountPath: backend.dataDir,
			}},
		},
	})
	if err := f.SeedClient.Client().SubResource("ephemeralcontainers").Patch(ctx, pod, patch); err != nil {
		return "", fmt.Errorf("failed to add ephemeral debug container to pod %s/%s: %w", shootNamespace, pod.Name, err)
	}

	waitUntilEphemeralContainerRunning(ctx, f, shootNamespace, pod.Name, debugContainerName)

	return pod.Name, nil
}

// findDataVolumeName returns the name of the pod volume that the backend container mounts at backend.dataDir, i.e. the
// data PVC that pvc-autoscaler watches.
func findDataVolumeName(pod *corev1.Pod, backend logBackend) (string, error) {
	for _, container := range pod.Spec.Containers {
		if container.Name != backend.containerName {
			continue
		}
		for _, volumeMount := range container.VolumeMounts {
			if volumeMount.MountPath == backend.dataDir {
				return volumeMount.Name, nil
			}
		}
	}
	return "", fmt.Errorf("could not find a volume mounted at %q in container %q of pod %s/%s", backend.dataDir, backend.containerName, pod.Namespace, pod.Name)
}

// waitUntilEphemeralContainerRunning waits until the ephemeral container with the given name reports a running state.
func waitUntilEphemeralContainerRunning(ctx context.Context, f *framework.ShootFramework, namespace, podName, containerName string) {
	Eventually(ctx, func() error {
		pod := &corev1.Pod{}
		if err := f.SeedClient.Client().Get(ctx, types.NamespacedName{Namespace: namespace, Name: podName}, pod); err != nil {
			return err
		}
		for _, status := range pod.Status.EphemeralContainerStatuses {
			if status.Name == containerName && status.State.Running != nil {
				return nil
			}
		}
		return fmt.Errorf("ephemeral container %q in pod %s/%s is not running yet", containerName, namespace, podName)
	}).WithPolling(2 * time.Second).WithTimeout(pvcAutoscalerInitializationTimeout).Should(Succeed())
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

func waitUntilFilesystemExpands(ctx context.Context, f *framework.ShootFramework, backend logBackend, shootNamespace, debugContainerName string, initialPVCSize resource.Quantity) {
	Eventually(func(g Gomega) {
		// Ensure the debug container as vpa can restart the backend pod.
		podName, err := ensureDebugContainer(ctx, f, backend, shootNamespace, debugContainerName)
		g.Expect(err).NotTo(HaveOccurred(), "ensuring debug container failed")

		stdout, _, err := f.SeedClient.PodExecutor().Execute(ctx, shootNamespace, podName, debugContainerName,
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
