// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package shoot

import (
	"strconv"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1helper "github.com/gardener/gardener/pkg/api/core/v1beta1/helper"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	. "github.com/gardener/gardener/test/e2e"
	. "github.com/gardener/gardener/test/e2e/gardener"
	"github.com/gardener/gardener/test/e2e/gardener/seed"
	"github.com/gardener/gardener/test/e2e/gardener/shoot/internal/bastion"
	"github.com/gardener/gardener/test/e2e/gardener/shoot/internal/exposureclass"
	"github.com/gardener/gardener/test/e2e/gardener/shoot/internal/inclusterclient"
	"github.com/gardener/gardener/test/e2e/gardener/shoot/internal/zerodowntimevalidator"
	"github.com/gardener/gardener/test/utils/access"
	shootupdatesuite "github.com/gardener/gardener/test/utils/shoots/update"
	"github.com/gardener/gardener/test/utils/shoots/update/inplace"
)

const (
	// Explicitly use one version below the latest supported minor version
	// so that Kubernetes version update test can be performed.
	kubernetesTargetVersion = "1.36"
	kubernetesSourceVersion = "1.35"
)

var _ = Describe("Shoot Tests", Label("Shoot", "default"), func() {
	Describe("Create, Update, Delete", Label("simple"), func() {
		test := func(tc *ShootContext, withInPlaceUpdatePools, testBastionAndExposureClass, testMaintenanceAnnotation bool) {
			tc.Shoot.Spec.Kubernetes.KubeAPIServer.EncryptionConfig = &gardencorev1beta1.EncryptionConfig{
				Resources: []string{"services", "clusterroles.rbac.authorization.k8s.io"},
			}

			tc.Shoot.Spec.Kubernetes.Version = kubernetesTargetVersion

			if !v1beta1helper.IsWorkerless(tc.Shoot) {
				// create worker pools which explicitly specify the kubernetes version and with different update strategies
				pool1 := tc.Shoot.Spec.Provider.Workers[0]
				pool2, pool3 := pool1.DeepCopy(), pool1.DeepCopy()
				pool2.Name += "2"
				pool2.Kubernetes = &gardencorev1beta1.WorkerKubernetes{Version: &tc.Shoot.Spec.Kubernetes.Version}
				pool3.Name += "3"
				pool3.Kubernetes = &gardencorev1beta1.WorkerKubernetes{Version: new(kubernetesSourceVersion)}
				tc.Shoot.Spec.Provider.Workers = append(tc.Shoot.Spec.Provider.Workers, *pool2, *pool3)
			}

			if withInPlaceUpdatePools {
				pool4 := DefaultWorker("auto", new(gardencorev1beta1.AutoInPlaceUpdate))
				pool4.Kubernetes = &gardencorev1beta1.WorkerKubernetes{Version: &tc.Shoot.Spec.Kubernetes.Version}
				pool4.Minimum = 2
				pool4.Maximum = 2
				pool4.MaxUnavailable = new(intstr.FromInt(1))
				pool4.MaxSurge = new(intstr.FromInt(0))

				pool5 := DefaultWorker("manual", new(gardencorev1beta1.ManualInPlaceUpdate))
				pool5.Kubernetes = &gardencorev1beta1.WorkerKubernetes{
					Version: new(kubernetesSourceVersion),
					Kubelet: &gardencorev1beta1.KubeletConfig{
						CPUManagerPolicy: new("none"),
						EvictionHard: &gardencorev1beta1.KubeletConfigEviction{
							MemoryAvailable: new("100Mi"),
							NodeFSAvailable: new("100Mi"),
						},
					},
				}

				pool6 := DefaultWorker("auto-surge", new(gardencorev1beta1.AutoInPlaceUpdate))
				pool6.MaxSurge = new(intstr.FromInt(1))
				pool6.MaxUnavailable = new(intstr.FromInt(0))

				tc.Shoot.Spec.Provider.Workers = []gardencorev1beta1.Worker{pool4, pool5, pool6}
			}

			BeforeAll(func() {
				tc.Init()
			})

			ItShouldCreateShoot(tc)
			ItShouldWaitForShootToBeReconciledAndHealthy(tc)
			ItShouldInitializeShootClient(tc)
			ItShouldGetResponsibleSeed(tc)
			seed.ItShouldInitializeSeedClient(tc.SeedContext)

			It("Verify shoot access using admin kubeconfig", func(ctx SpecContext) {
				Eventually(ctx, tc.ShootKomega.List(&corev1.NamespaceList{})).Should(Succeed())
			}, SpecTimeout(time.Minute))

			verifyViewerKubeconfigShootAccess(tc)

			if withInPlaceUpdatePools {
				inplace.ItShouldLabelManualInPlaceNodesWithSelectedForUpdate(tc)
			}

			if !v1beta1helper.IsWorkerless(tc.Shoot) {
				verifyWorkerNodeLabels(tc)

				It("Verify reported CIDRs", func(ctx SpecContext) {
					// For workerless shoots, the status.networking section is not reported. Skip its verification accordingly.
					Eventually(ctx, func(g Gomega) {
						g.Expect(tc.GardenKomega.Get(tc.Shoot)()).To(Succeed())

						networking := ptr.Deref(tc.Shoot.Status.Networking, gardencorev1beta1.NetworkingStatus{})
						if nodes := tc.Shoot.Spec.Networking.Nodes; nodes != nil {
							g.Expect(networking.Nodes).To(ConsistOf(*nodes))
							g.Expect(networking.EgressCIDRs).To(ConsistOf(*nodes))
						}
						if services := tc.Shoot.Spec.Networking.Services; services != nil {
							g.Expect(networking.Services).To(ConsistOf(*services))
						}
						if pods := tc.Shoot.Spec.Networking.Pods; pods != nil {
							g.Expect(networking.Pods).To(ConsistOf(*pods))
						}
					}).Should(Succeed())
				}, SpecTimeout(time.Minute))

				inclusterclient.VerifyInClusterAccessToAPIServer(tc)
				if testBastionAndExposureClass {
					bastion.VerifyBastion(tc)
				}
			}

			zeroDowntimeValidatorJob := &zerodowntimevalidator.Job{}
			if v1beta1helper.IsHAControlPlaneConfigured(tc.Shoot) {
				zeroDowntimeValidatorJob.ItShouldDeployJob(tc)
				zeroDowntimeValidatorJob.ItShouldWaitForJobToBeReady(tc)
				zeroDowntimeValidatorJob.AfterAllDeleteJob(tc)
			}

			verifyNodeKubernetesVersions(tc)

			var (
				nodesOfInPlaceWorkersBeforeTest sets.Set[string]
				cloudProfile                    *gardencorev1beta1.CloudProfile
				controlPlaneKubernetesVersion   string
				poolNameToKubernetesVersion     map[string]string
			)

			if withInPlaceUpdatePools {
				It("should get the nodes of worker with in-place update strategy", func(ctx SpecContext) {
					nodesOfInPlaceWorkersBeforeTest = inplace.FindNodesOfInPlaceWorkers(ctx, tc.Log, tc.ShootClient, tc.Shoot)
				}, SpecTimeout(2*time.Minute))
			}

			It("Get CloudProfile", func(ctx SpecContext) {
				Eventually(ctx, func() error {
					var err error
					cloudProfile, err = gardenerutils.GetCloudProfile(ctx, tc.GardenClient, tc.Shoot)
					return err
				}).Should(Succeed())
			}, SpecTimeout(time.Minute))

			It("Compute new Kubernetes version for control plane and worker pools", func() {
				var err error
				controlPlaneKubernetesVersion, poolNameToKubernetesVersion, err = shootupdatesuite.ComputeNewKubernetesVersions(cloudProfile, tc.Shoot, nil, nil)
				Expect(err).NotTo(HaveOccurred())
			})

			It("Update Shoot", func(ctx SpecContext) {
				patch := client.StrategicMergeFrom(tc.Shoot.DeepCopy())
				if controlPlaneKubernetesVersion != "" {
					tc.Log.Info("Updating control plane Kubernetes version", "version", controlPlaneKubernetesVersion)
					tc.Shoot.Spec.Kubernetes.Version = controlPlaneKubernetesVersion
				}
				for i, worker := range tc.Shoot.Spec.Provider.Workers {
					if workerPoolVersion, ok := poolNameToKubernetesVersion[worker.Name]; ok {
						tc.Log.Info("Updating worker pool Kubernetes version", "pool", worker.Name, "version", workerPoolVersion)
						tc.Shoot.Spec.Provider.Workers[i].Kubernetes.Version = &workerPoolVersion
					}

					if ptr.Deref(worker.UpdateStrategy, "") == gardencorev1beta1.AutoInPlaceUpdate {
						tc.Log.Info("Updating worker pool machine image version", "pool", worker.Name, "version", "2.0.0")
						tc.Shoot.Spec.Provider.Workers[i].Machine.Image.Version = new("2.0.0")
					} else if ptr.Deref(worker.UpdateStrategy, "") == gardencorev1beta1.ManualInPlaceUpdate {
						tc.Log.Info("Updating worker pool Kubelet config", "pool", worker.Name)
						tc.Shoot.Spec.Provider.Workers[i].Kubernetes.Kubelet = &gardencorev1beta1.KubeletConfig{
							CPUManagerPolicy: new("static"),
							EvictionHard: &gardencorev1beta1.KubeletConfigEviction{
								MemoryAvailable: new("200Mi"),
								NodeFSAvailable: new("200Mi"),
							},
						}
					}
				}

				Eventually(ctx, func() error {
					return tc.GardenClient.Patch(ctx, tc.Shoot, patch)
				}).Should(Succeed())
			}, SpecTimeout(time.Minute))

			if withInPlaceUpdatePools {
				inplace.ItShouldVerifyInPlaceUpdateStart(tc, true, true)
			}

			ItShouldWaitForShootToBeReconciledAndHealthy(tc)
			ItShouldInitializeShootClient(tc)
			verifyNodeKubernetesVersions(tc)

			if v1beta1helper.IsHAControlPlaneConfigured(tc.Shoot) {
				zeroDowntimeValidatorJob.ItShouldEnsureThereWasNoDowntime(tc)
			}

			if !v1beta1helper.IsWorkerless(tc.Shoot) {
				inclusterclient.VerifyInClusterAccessToAPIServer(tc)

			}

			if withInPlaceUpdatePools {
				It("should compare the node names after the test", func(ctx SpecContext) {
					totalInPlaceWorkersMaxSurge := inplace.GetTotalInPlaceWorkersMaxSurge(tc.Shoot)
					tc.Log.Info("Total in-place workers max surge", "maxSurge", totalInPlaceWorkersMaxSurge)

					nodesOfInPlaceWorkersAfterTest := inplace.FindNodesOfInPlaceWorkers(ctx, tc.Log, tc.ShootClient, tc.Shoot)
					tc.Log.Info("Nodes of in-place workers before test and after test", "beforeNodes", nodesOfInPlaceWorkersBeforeTest.UnsortedList(), "afterNodes", nodesOfInPlaceWorkersAfterTest.UnsortedList())

					Expect(nodesOfInPlaceWorkersAfterTest.Intersection(nodesOfInPlaceWorkersBeforeTest)).To(HaveLen(nodesOfInPlaceWorkersBeforeTest.Len() - totalInPlaceWorkersMaxSurge))
				}, SpecTimeout(2*time.Minute))

				inplace.ItShouldVerifyInPlaceUpdateCompletion(tc)
			}

			if testBastionAndExposureClass {
				exposureclass.VerifyExposureClassSwitch(tc, ItShouldWaitForShootToBeReconciledAndHealthy)
			}

			if testMaintenanceAnnotation {
				ItShouldAnnotateShoot(tc, map[string]string{
					"shoot.gardener.cloud/skip-readiness": "",
					"gardener.cloud/operation":            "maintain",
				})

				It("Wait for operation annotation to be gone (meaning controller picked up reconciliation request)", func(ctx SpecContext) {
					Eventually(ctx, tc.GardenKomega.Object(tc.Shoot)).Should(
						HaveField("Annotations", Not(HaveKey("gardener.cloud/operation"))),
					)
				}, SpecTimeout(time.Minute))

				ItShouldWaitForShootToBeReconciledAndHealthy(tc)

				It("Wait for skip-readiness annotation to be gone", func(ctx SpecContext) {
					Eventually(ctx, tc.GardenKomega.Object(tc.Shoot)).Should(
						HaveField("Annotations", Not(HaveKey("shoot.gardener.cloud/skip-readiness"))),
					)
				}, SpecTimeout(time.Minute))
			}

			ItShouldDeleteShoot(tc)
			ItShouldWaitForShootToBeDeleted(tc)
		}

		Context("Shoot with workers", Label("basic"), Ordered, PriorityLong, func() {
			test(NewShootContext(DefaultShoot("e2e-default")), false, true, true)
		})

		Context("Shoot with only in-place workers", Label("basic", "in-place"), Ordered, PriorityLong, func() {
			test(NewShootContext(DefaultShoot("e2e-inplace")), true, false, false)
		})

		Context("Shoot with workers and layer 4 load balancing", Ordered, Label("basic"), PriorityLong, func() {
			shoot := DefaultShoot("e2e-layer4-lb")
			metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, v1beta1constants.ShootDisableIstioTLSTermination, "true")
			test(NewShootContext(shoot), false, true, false)
		})

		Context("Workerless Shoot", Label("workerless"), Ordered, func() {
			test(NewShootContext(DefaultWorkerlessShoot("e2e-default")), false, false, true)
		})
	})
})

func verifyNodeKubernetesVersions(tc *ShootContext) {
	GinkgoHelper()

	It("Verify that the Kubernetes versions for all existing nodes match the versions defined in the Shoot spec", func(ctx SpecContext) {
		Eventually(ctx, func(g Gomega) {
			g.Expect(shootupdatesuite.VerifyKubernetesVersions(ctx, tc.ShootClientSet, tc.Shoot)).To(Succeed())
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))
}

func verifyWorkerNodeLabels(tc *ShootContext) {
	GinkgoHelper()

	It("Verify worker node labels", func(ctx SpecContext) {
		commonNodeLabels := utils.MergeStringMaps(tc.Shoot.Spec.Provider.Workers[0].Labels)
		commonNodeLabels["networking.gardener.cloud/node-local-dns-enabled"] = "false"
		commonNodeLabels["node.kubernetes.io/role"] = "node"

		Eventually(ctx, func(g Gomega) {
			for _, workerPool := range tc.Shoot.Spec.Provider.Workers {
				expectedNodeLabels := utils.MergeStringMaps(commonNodeLabels)
				expectedNodeLabels["worker.gardener.cloud/pool"] = workerPool.Name
				expectedNodeLabels["worker.gardener.cloud/cri-name"] = string(workerPool.CRI.Name)
				expectedNodeLabels["worker.gardener.cloud/system-components"] = strconv.FormatBool(workerPool.SystemComponents.Allow)

				kubernetesVersion := tc.Shoot.Spec.Kubernetes.Version
				if workerPool.Kubernetes != nil && workerPool.Kubernetes.Version != nil {
					kubernetesVersion = *workerPool.Kubernetes.Version
				}
				expectedNodeLabels["worker.gardener.cloud/kubernetes-version"] = kubernetesVersion

				nodeList := &corev1.NodeList{}
				g.Expect(tc.ShootClient.List(ctx, nodeList, client.MatchingLabels{
					"worker.gardener.cloud/pool": workerPool.Name,
				})).To(Succeed())
				g.Expect(len(nodeList.Items)).To(BeNumerically(">=", workerPool.Minimum), "worker pool %s should have at least %d nodes", workerPool.Name, workerPool.Minimum)
				g.Expect(len(nodeList.Items)).To(BeNumerically("<=", workerPool.Maximum), "worker pool %s should have at most %d nodes", workerPool.Name, workerPool.Maximum)

				for _, node := range nodeList.Items {
					for key, value := range expectedNodeLabels {
						g.Expect(node.Labels).To(HaveKeyWithValue(key, value), "worker pool %s node %s should have label %s=%s", workerPool.Name, node.Name, key, value)
					}
				}
			}
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))
}

func verifyViewerKubeconfigShootAccess(tc *ShootContext) {
	GinkgoHelper()

	It("Verify shoot access using viewer kubeconfig", func(ctx SpecContext) {
		Eventually(ctx, func(g Gomega) {
			readOnlyShootClient, err := access.CreateShootClientFromViewerKubeconfig(ctx, tc.GardenClientSet, tc.Shoot)
			g.Expect(err).NotTo(HaveOccurred())

			g.Expect(readOnlyShootClient.Client().List(ctx, &corev1.ConfigMapList{})).To(Succeed())
			g.Expect(readOnlyShootClient.Client().List(ctx, &corev1.SecretList{})).To(BeForbiddenError())
			g.Expect(readOnlyShootClient.Client().List(ctx, &corev1.ServiceList{})).To(BeForbiddenError())
			g.Expect(readOnlyShootClient.Client().List(ctx, &rbacv1.ClusterRoleList{})).To(BeForbiddenError())
			g.Expect(readOnlyShootClient.Client().Create(ctx, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{GenerateName: "test-", Namespace: metav1.NamespaceDefault}})).To(BeForbiddenError())
			g.Expect(readOnlyShootClient.Client().Update(ctx, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "kube-root-ca.crt", Namespace: metav1.NamespaceDefault}})).To(BeForbiddenError())
			g.Expect(readOnlyShootClient.Client().Patch(ctx, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "kube-root-ca.crt", Namespace: metav1.NamespaceDefault}}, client.RawPatch(types.MergePatchType, []byte("{}")))).To(BeForbiddenError())
			g.Expect(readOnlyShootClient.Client().Delete(ctx, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "kube-root-ca.crt", Namespace: metav1.NamespaceDefault}})).To(BeForbiddenError())
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))
}
