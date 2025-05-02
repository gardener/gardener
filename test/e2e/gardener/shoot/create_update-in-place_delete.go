// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shoot

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	. "github.com/gardener/gardener/test/e2e/gardener"
	"github.com/gardener/gardener/test/e2e/gardener/shoot/internal/inclusterclient"
	shootupdatesuite "github.com/gardener/gardener/test/utils/shoots/update"
	"github.com/gardener/gardener/test/utils/shoots/update/inplace"
)

var _ = Describe("Shoot Tests", Label("Shoot", "default"), func() {
	Describe("Create, Update, Delete", Label("simple"), func() {
		Context("Shoot with in-place workers", Label("basic", "in-place"), Ordered, func() {
			shoot := DefaultShoot("e2e-default-ip")

			s := NewTestContext().ForShoot(shoot)

			s.Shoot.Spec.Kubernetes.Version = kubernetesTargetVersion

			// create two worker pools which explicitly specify the kubernetes version
			pool1 := DefaultWorker("auto", ptr.To(gardencorev1beta1.AutoInPlaceUpdate))
			pool1.Kubernetes = &gardencorev1beta1.WorkerKubernetes{Version: &s.Shoot.Spec.Kubernetes.Version}
			pool1.Minimum = 2
			pool1.Maximum = 2
			pool1.MaxUnavailable = ptr.To(intstr.FromInt(1))
			pool1.MaxSurge = ptr.To(intstr.FromInt(0))

			pool2 := DefaultWorker("manual", ptr.To(gardencorev1beta1.ManualInPlaceUpdate))
			pool2.Kubernetes = &gardencorev1beta1.WorkerKubernetes{
				Version: ptr.To(kubernetesSourceVersion),
				Kubelet: &gardencorev1beta1.KubeletConfig{
					CPUManagerPolicy: ptr.To("none"),
					EvictionHard: &gardencorev1beta1.KubeletConfigEviction{
						MemoryAvailable: ptr.To("100Mi"),
						NodeFSAvailable: ptr.To("100Mi"),
					},
				},
			}

			s.Shoot.Spec.Provider.Workers = []gardencorev1beta1.Worker{pool1, pool2}

			ItShouldCreateShoot(s)
			ItShouldWaitForShootToBeReconciledAndHealthy(s)
			ItShouldInitializeShootClient(s)
			ItShouldGetResponsibleSeed(s)
			ItShouldInitializeSeedClient(s)

			inplace.ItShouldLabelManualInPlaceNodesWithSelectedForUpdate(s)
			verifyWorkerNodeLabels(s)
			inclusterclient.VerifyInClusterAccessToAPIServer(s)
			verifyNodeKubernetesVersions(s)

			var (
				nodesOfInPlaceWorkersBeforeTest = inplace.ItShouldFindAllNodesOfInPlaceWorker(s)
				cloudProfile                    *gardencorev1beta1.CloudProfile
				controlPlaneKubernetesVersion   string
				poolNameToKubernetesVersion     map[string]string
			)

			It("Get CloudProfile", func(ctx SpecContext) {
				Eventually(ctx, func() error {
					var err error
					cloudProfile, err = gardenerutils.GetCloudProfile(ctx, s.GardenClient, s.Shoot)
					return err
				}).Should(Succeed())
			}, SpecTimeout(time.Minute))

			It("Compute new Kubernetes version for control plane and worker pools", func() {
				var err error
				controlPlaneKubernetesVersion, poolNameToKubernetesVersion, err = shootupdatesuite.ComputeNewKubernetesVersions(cloudProfile, s.Shoot, nil, nil)
				Expect(err).NotTo(HaveOccurred())
			})

			It("Update Shoot", func(ctx SpecContext) {
				patch := client.StrategicMergeFrom(s.Shoot.DeepCopy())
				if controlPlaneKubernetesVersion != "" {
					s.Log.Info("Updating control plane Kubernetes version", "version", controlPlaneKubernetesVersion)
					s.Shoot.Spec.Kubernetes.Version = controlPlaneKubernetesVersion
				}
				for i, worker := range s.Shoot.Spec.Provider.Workers {
					if workerPoolVersion, ok := poolNameToKubernetesVersion[worker.Name]; ok {
						s.Log.Info("Updating worker pool Kubernetes version", "pool", worker.Name, "version", workerPoolVersion)
						s.Shoot.Spec.Provider.Workers[i].Kubernetes.Version = &workerPoolVersion
					}
				}

				// Update machine image version for pool1
				s.Log.Info("Updating worker pool machine image version", "pool", s.Shoot.Spec.Provider.Workers[0].Name, "version", "2.0.0")
				s.Shoot.Spec.Provider.Workers[0].Machine.Image.Version = ptr.To("2.0.0")

				// Update Kubelet config for pool2
				s.Log.Info("Updating worker pool Kubelet config", "pool", s.Shoot.Spec.Provider.Workers[1].Name)
				s.Shoot.Spec.Provider.Workers[1].Kubernetes.Kubelet = &gardencorev1beta1.KubeletConfig{
					CPUManagerPolicy: ptr.To("static"),
					EvictionHard: &gardencorev1beta1.KubeletConfigEviction{
						MemoryAvailable: ptr.To("200Mi"),
						NodeFSAvailable: ptr.To("200Mi"),
					},
				}

				Eventually(ctx, func() error {
					return s.GardenClient.Patch(ctx, s.Shoot, patch)
				}).Should(Succeed())
			}, SpecTimeout(time.Minute))

			inplace.ItShouldVerifyInPlaceUpdateStart(s.GardenClient, s.Shoot, true, true)

			ItShouldWaitForShootToBeReconciledAndHealthy(s)
			ItShouldInitializeShootClient(s)
			verifyNodeKubernetesVersions(s)
			inclusterclient.VerifyInClusterAccessToAPIServer(s)

			nodesOfInPlaceWorkersAfterTest := inplace.ItShouldFindAllNodesOfInPlaceWorker(s)
			Expect(nodesOfInPlaceWorkersBeforeTest.UnsortedList()).To(ConsistOf(nodesOfInPlaceWorkersAfterTest.UnsortedList()))
			inplace.ItShouldVerifyInPlaceUpdateCompletion(s.GardenClient, s.Shoot)

			ItShouldDeleteShoot(s)
			ItShouldWaitForShootToBeDeleted(s)
		})
	})
})
