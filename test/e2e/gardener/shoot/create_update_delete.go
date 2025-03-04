// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shoot

import (
	"strconv"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/gardener"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	. "github.com/gardener/gardener/test/e2e"
	. "github.com/gardener/gardener/test/e2e/gardener"
	. "github.com/gardener/gardener/test/e2e/gardener/shoot/internal"
	"github.com/gardener/gardener/test/e2e/gardener/shoot/internal/inclusterclient"
	"github.com/gardener/gardener/test/utils/access"
	shootupdatesuite "github.com/gardener/gardener/test/utils/shoots/update"
	"github.com/gardener/gardener/test/utils/shoots/update/highavailability"
)

const (
	// Explicitly use one version below the latest supported minor version
	// so that Kubernetes version update test can be performed.
	kubernetesTargetVersion = "1.31.1"
	kubernetesSourceVersion = "1.30.0"
)

var _ = Describe("Shoot Tests", Label("Shoot", "default"), func() {
	Describe("Create, Update, Delete", Label("simple"), func() {
		test := func(s *ShootContext) {
			BeforeTestSetup(func() {
				s.Shoot.Spec.Kubernetes.KubeAPIServer.EncryptionConfig = &gardencorev1beta1.EncryptionConfig{
					Resources: []string{"services", "clusterroles.rbac.authorization.k8s.io"},
				}

				s.Shoot.Spec.Kubernetes.Version = kubernetesTargetVersion

				if !v1beta1helper.IsWorkerless(s.Shoot) {
					// create two additional worker pools which explicitly specify the kubernetes version
					pool1 := s.Shoot.Spec.Provider.Workers[0]
					pool2, pool3 := pool1.DeepCopy(), pool1.DeepCopy()
					pool2.Name += "2"
					pool2.Kubernetes = &gardencorev1beta1.WorkerKubernetes{Version: &s.Shoot.Spec.Kubernetes.Version}
					pool3.Name += "3"
					pool3.Kubernetes = &gardencorev1beta1.WorkerKubernetes{Version: ptr.To(kubernetesSourceVersion)}
					s.Shoot.Spec.Provider.Workers = append(s.Shoot.Spec.Provider.Workers, *pool2, *pool3)
				}
			})

			ItShouldCreateShoot(s)
			ItShouldWaitForShootToBeReconciledAndHealthy(s)
			ItShouldInitializeShootClient(s)
			ItShouldGetResponsibleSeed(s)
			ItShouldInitializeSeedClient(s)

			It("Verify shoot access using admin kubeconfig", func(ctx SpecContext) {
				Eventually(ctx, s.ShootKomega.List(&corev1.NamespaceList{})).Should(Succeed())
			}, SpecTimeout(time.Minute))

			verifyViewerKubeconfigShootAccess(s)

			if !v1beta1helper.IsWorkerless(s.Shoot) {
				It("Verify worker node labels", func(ctx SpecContext) {
					commonNodeLabels := utils.MergeStringMaps(s.Shoot.Spec.Provider.Workers[0].Labels)
					commonNodeLabels["networking.gardener.cloud/node-local-dns-enabled"] = "false"
					commonNodeLabels["node.kubernetes.io/role"] = "node"

					Eventually(ctx, func(g Gomega) {
						for _, workerPool := range s.Shoot.Spec.Provider.Workers {
							expectedNodeLabels := utils.MergeStringMaps(commonNodeLabels)
							expectedNodeLabels["worker.gardener.cloud/pool"] = workerPool.Name
							expectedNodeLabels["worker.gardener.cloud/cri-name"] = string(workerPool.CRI.Name)
							expectedNodeLabels["worker.gardener.cloud/system-components"] = strconv.FormatBool(workerPool.SystemComponents.Allow)

							kubernetesVersion := s.Shoot.Spec.Kubernetes.Version
							if workerPool.Kubernetes != nil && workerPool.Kubernetes.Version != nil {
								kubernetesVersion = *workerPool.Kubernetes.Version
							}
							expectedNodeLabels["worker.gardener.cloud/kubernetes-version"] = kubernetesVersion

							nodeList := &corev1.NodeList{}
							g.Expect(s.ShootClient.List(ctx, nodeList, client.MatchingLabels{
								"worker.gardener.cloud/pool": workerPool.Name,
							})).To(Succeed())
							g.Expect(nodeList.Items).To(HaveLen(1), "worker pool %s should have exactly one Node", workerPool.Name)

							for key, value := range expectedNodeLabels {
								g.Expect(nodeList.Items[0].Labels).To(HaveKeyWithValue(key, value), "worker pool %s should have expected labels", workerPool.Name)
							}
						}
					}).Should(Succeed())
				}, SpecTimeout(time.Minute))

				It("Verify reported CIDRs", func(ctx SpecContext) {
					// For workerless shoots, the status.networking section is not reported. Skip its verification accordingly.
					Eventually(ctx, func(g Gomega) {
						g.Expect(s.GardenKomega.Get(s.Shoot)()).To(Succeed())

						networking := ptr.Deref(s.Shoot.Status.Networking, gardencorev1beta1.NetworkingStatus{})
						if nodes := s.Shoot.Spec.Networking.Nodes; nodes != nil {
							g.Expect(networking.Nodes).To(ConsistOf(*nodes))
							g.Expect(networking.EgressCIDRs).To(ConsistOf(*nodes))
						}
						if services := s.Shoot.Spec.Networking.Services; services != nil {
							g.Expect(networking.Services).To(ConsistOf(*services))
						}
						if pods := s.Shoot.Spec.Networking.Pods; pods != nil {
							g.Expect(networking.Pods).To(ConsistOf(*pods))
						}
					}).Should(Succeed())
				}, SpecTimeout(time.Minute))

				inclusterclient.VerifyInClusterAccessToAPIServer(s)
			}

			var (
				zeroDowntimeJob *batchv1.Job

				cloudProfile *gardencorev1beta1.CloudProfile

				controlPlaneKubernetesVersion string
				poolNameToKubernetesVersion   map[string]string
			)

			if v1beta1helper.IsHAControlPlaneConfigured(s.Shoot) {
				It("Deploy zero-downtime validator job to ensure no kube-apiserver downtime while running subsequent operations", func(ctx SpecContext) {
					Eventually(ctx, func(g Gomega) {
						var err error
						controlPlaneNamespace := s.Shoot.Status.TechnicalID
						zeroDowntimeJob, err = highavailability.DeployZeroDownTimeValidatorJob(
							ctx,
							s.SeedClient, "update", controlPlaneNamespace,
							shootupdatesuite.GetKubeAPIServerAuthToken(
								ctx,
								s.SeedClientSet,
								controlPlaneNamespace,
							),
						)
						g.Expect(err).NotTo(HaveOccurred())
					}).Should(Succeed())
				}, SpecTimeout(time.Minute))

				It("Wait for zero-downtime validator job to be ready", func(ctx SpecContext) {
					shootupdatesuite.WaitForJobToBeReady(ctx, s.SeedClient, zeroDowntimeJob)
				}, SpecTimeout(time.Minute))

				AfterAll(func(ctx SpecContext) {
					Expect(s.SeedClient.Delete(ctx, zeroDowntimeJob, client.PropagationPolicy(metav1.DeletePropagationForeground))).
						To(Or(Succeed(), BeNotFoundError()))
				}, NodeTimeout(time.Minute))
			}

			verifyNodeKubernetesVersions(s)

			It("Get CloudProfile", func(ctx SpecContext) {
				Eventually(ctx, func() error {
					var err error
					cloudProfile, err = gardener.GetCloudProfile(ctx, s.GardenClient, s.Shoot)
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
				Eventually(ctx, func() error {
					return s.GardenClient.Patch(ctx, s.Shoot, patch)
				}).Should(Succeed())
			}, SpecTimeout(time.Minute))

			ItShouldWaitForShootToBeReconciledAndHealthy(s)
			ItShouldInitializeShootClient(s)
			verifyNodeKubernetesVersions(s)

			if zeroDowntimeJob != nil {
				It("Ensure there was no downtime while upgrading shoot", func(ctx SpecContext) {
					Eventually(ctx, s.SeedKomega.Get(zeroDowntimeJob)).Should(Succeed())
					Expect(zeroDowntimeJob.Status.Failed).To(BeZero())
				}, SpecTimeout(time.Minute))
			}

			if !v1beta1helper.IsWorkerless(s.Shoot) {
				inclusterclient.VerifyInClusterAccessToAPIServer(s)
			}

			ItShouldAnnotateShoot(s, map[string]string{
				"shoot.gardener.cloud/skip-readiness": "",
				"gardener.cloud/operation":            "maintain",
			})

			It("Wait for operation annotation to be gone (meaning controller picked up reconciliation request)", func(ctx SpecContext) {
				Eventually(ctx, s.GardenKomega.Object(s.Shoot)).Should(
					HaveField("Annotations", Not(HaveKey("gardener.cloud/operation"))),
				)
			}, SpecTimeout(time.Minute))

			ItShouldWaitForShootToBeReconciledAndHealthy(s)

			It("Wait for skip-readiness annotation to be gone", func(ctx SpecContext) {
				Eventually(ctx, s.GardenKomega.Object(s.Shoot)).Should(
					HaveField("Annotations", Not(HaveKey("shoot.gardener.cloud/skip-readiness"))),
				)
			}, SpecTimeout(time.Minute))

			ItShouldDeleteShoot(s)
			ItShouldWaitForShootToBeDeleted(s)
		}

		Context("Shoot with workers", Label("basic"), Ordered, func() {
			test(NewTestContext().ForShoot(DefaultShoot("e2e-default")))
		})

		Context("Shoot with workers and layer 4 load balancing", Ordered, Label("basic"), func() {
			shoot := DefaultShoot("e2e-layer4-lb")
			metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, v1beta1constants.ShootDisableIstioTLSTermination, "true")
			test(NewTestContext().ForShoot(shoot))
		})

		Context("Workerless Shoot", Label("workerless"), Ordered, func() {
			test(NewTestContext().ForShoot(DefaultWorkerlessShoot("e2e-default")))
		})
	})
})

func verifyNodeKubernetesVersions(s *ShootContext) {
	GinkgoHelper()

	It("Verify that the Kubernetes versions for all existing nodes match the versions defined in the Shoot spec", func(ctx SpecContext) {
		Eventually(ctx, func(g Gomega) {
			g.Expect(shootupdatesuite.VerifyKubernetesVersions(ctx, s.ShootClientSet, s.Shoot)).To(Succeed())
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))
}

func verifyViewerKubeconfigShootAccess(s *ShootContext) {
	GinkgoHelper()

	It("Verify shoot access using viewer kubeconfig", func(ctx SpecContext) {
		Eventually(ctx, func(g Gomega) {
			readOnlyShootClient, err := access.CreateShootClientFromViewerKubeconfig(ctx, s.GardenClientSet, s.Shoot)
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
