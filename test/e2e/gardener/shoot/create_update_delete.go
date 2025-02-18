// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shoot

import (
	"context"
	"strconv"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/utils"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	e2e "github.com/gardener/gardener/test/e2e/gardener"
	"github.com/gardener/gardener/test/e2e/gardener/shoot/internal/inclusterclient"
	"github.com/gardener/gardener/test/framework"
	"github.com/gardener/gardener/test/utils/access"
	shootupdatesuite "github.com/gardener/gardener/test/utils/shoots/update"
)

var _ = Describe("Shoot Tests", Label("Shoot", "default"), func() {
	test := func(shoot *gardencorev1beta1.Shoot) {
		f := defaultShootCreationFramework()
		f.Shoot = shoot
		f.Shoot.Spec.Kubernetes.KubeAPIServer.EncryptionConfig = &gardencorev1beta1.EncryptionConfig{
			Resources: []string{"services", "endpointslices.discovery.k8s.io"},
		}

		// explicitly use one version below the latest supported minor version so that Kubernetes version update test can be
		// performed
		f.Shoot.Spec.Kubernetes.Version = "1.30.0"

		if !v1beta1helper.IsWorkerless(f.Shoot) {
			// create two additional worker pools which explicitly specify the kubernetes version
			pool1 := f.Shoot.Spec.Provider.Workers[0]
			pool2, pool3 := pool1.DeepCopy(), pool1.DeepCopy()
			pool2.Name += "2"
			pool2.Kubernetes = &gardencorev1beta1.WorkerKubernetes{Version: &f.Shoot.Spec.Kubernetes.Version}
			pool3.Name += "3"
			pool3.Kubernetes = &gardencorev1beta1.WorkerKubernetes{Version: ptr.To("1.29.0")}
			f.Shoot.Spec.Provider.Workers = append(f.Shoot.Spec.Provider.Workers, *pool2, *pool3)
		}

		It("Create, Update, Delete", Label("simple"), Offset(1), func() {
			By("Create Shoot")
			ctx, cancel := context.WithTimeout(parentCtx, 30*time.Minute)
			defer cancel()

			Expect(f.CreateShootAndWaitForCreation(ctx, false)).To(Succeed())
			f.Verify()

			var (
				shootClient kubernetes.Interface
				err         error
			)

			By("Verify shoot access using admin kubeconfig")
			Eventually(func(g Gomega) {
				shootClient, err = access.CreateShootClientFromAdminKubeconfig(ctx, f.GardenClient, f.Shoot)
				g.Expect(err).NotTo(HaveOccurred())

				g.Expect(shootClient.Client().List(ctx, &corev1.NamespaceList{})).To(Succeed())
			}).Should(Succeed())

			By("Verify shoot access using viewer kubeconfig")
			Eventually(func(g Gomega) {
				readOnlyShootClient, err := access.CreateShootClientFromViewerKubeconfig(ctx, f.GardenClient, f.Shoot)
				g.Expect(err).NotTo(HaveOccurred())

				g.Expect(readOnlyShootClient.Client().List(ctx, &corev1.ConfigMapList{})).To(Succeed())
				g.Expect(readOnlyShootClient.Client().List(ctx, &corev1.SecretList{})).To(BeForbiddenError())
				g.Expect(readOnlyShootClient.Client().List(ctx, &corev1.ServiceList{})).To(BeForbiddenError())
				g.Expect(readOnlyShootClient.Client().List(ctx, &discoveryv1.EndpointSliceList{})).To(BeForbiddenError())
				g.Expect(readOnlyShootClient.Client().Create(ctx, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{GenerateName: "test-", Namespace: metav1.NamespaceDefault}})).To(BeForbiddenError())
				g.Expect(readOnlyShootClient.Client().Update(ctx, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "kube-root-ca.crt", Namespace: metav1.NamespaceDefault}})).To(BeForbiddenError())
				g.Expect(readOnlyShootClient.Client().Patch(ctx, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "kube-root-ca.crt", Namespace: metav1.NamespaceDefault}}, client.RawPatch(types.MergePatchType, []byte("{}")))).To(BeForbiddenError())
				g.Expect(readOnlyShootClient.Client().Delete(ctx, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "kube-root-ca.crt", Namespace: metav1.NamespaceDefault}})).To(BeForbiddenError())
			}).Should(Succeed())

			if !v1beta1helper.IsWorkerless(f.Shoot) {
				By("Verify worker node labels")
				commonNodeLabels := utils.MergeStringMaps(f.Shoot.Spec.Provider.Workers[0].Labels)
				commonNodeLabels["networking.gardener.cloud/node-local-dns-enabled"] = "false"
				commonNodeLabels["node.kubernetes.io/role"] = "node"

				Eventually(func(g Gomega) {
					for _, workerPool := range f.Shoot.Spec.Provider.Workers {
						expectedNodeLabels := utils.MergeStringMaps(commonNodeLabels)
						expectedNodeLabels["worker.gardener.cloud/pool"] = workerPool.Name
						expectedNodeLabels["worker.gardener.cloud/cri-name"] = string(workerPool.CRI.Name)
						expectedNodeLabels["worker.gardener.cloud/system-components"] = strconv.FormatBool(workerPool.SystemComponents.Allow)

						kubernetesVersion := f.Shoot.Spec.Kubernetes.Version
						if workerPool.Kubernetes != nil && workerPool.Kubernetes.Version != nil {
							kubernetesVersion = *workerPool.Kubernetes.Version
						}
						expectedNodeLabels["worker.gardener.cloud/kubernetes-version"] = kubernetesVersion

						nodeList := &corev1.NodeList{}
						g.Expect(shootClient.Client().List(ctx, nodeList, client.MatchingLabels{
							"worker.gardener.cloud/pool": workerPool.Name,
						})).To(Succeed())
						g.Expect(nodeList.Items).To(HaveLen(1), "worker pool %s should have exactly one Node", workerPool.Name)

						for key, value := range expectedNodeLabels {
							g.Expect(nodeList.Items[0].Labels).To(HaveKeyWithValue(key, value), "worker pool %s should have expected labels", workerPool.Name)
						}
					}
				}).Should(Succeed())

				By("Verify reported CIDRs")
				// For workerless shoots, the status.networking section is not reported. Skip its verification accordingly.
				Eventually(func(g Gomega) {
					g.Expect(f.GardenClient.Client().Get(ctx, client.ObjectKeyFromObject(f.Shoot), f.Shoot)).To(Succeed())

					networking := ptr.Deref(f.Shoot.Status.Networking, gardencorev1beta1.NetworkingStatus{})
					if nodes := f.Shoot.Spec.Networking.Nodes; nodes != nil {
						g.Expect(networking.Nodes).To(ConsistOf(*nodes))
						g.Expect(networking.EgressCIDRs).To(ConsistOf(*nodes))
					}
					if services := f.Shoot.Spec.Networking.Services; services != nil {
						g.Expect(networking.Services).To(ConsistOf(*services))
					}
					if pods := f.Shoot.Spec.Networking.Pods; pods != nil {
						g.Expect(networking.Pods).To(ConsistOf(*pods))
					}
				}).Should(Succeed())

				inclusterclient.VerifyInClusterAccessToAPIServer(parentCtx, f.ShootFramework)
			}

			By("Update Shoot")
			ctx, cancel = context.WithTimeout(parentCtx, 20*time.Minute)
			defer cancel()
			shootupdatesuite.RunTest(ctx, &framework.ShootFramework{
				GardenerFramework: f.GardenerFramework,
				Shoot:             f.Shoot,
			}, nil, nil)

			if !v1beta1helper.IsWorkerless(f.Shoot) {
				inclusterclient.VerifyInClusterAccessToAPIServer(parentCtx, f.ShootFramework)
			}

			By("Add skip readiness annotation")
			ctx, cancel = context.WithTimeout(parentCtx, 10*time.Minute)
			defer cancel()
			Expect(f.ShootFramework.UpdateShoot(ctx, func(shoot *gardencorev1beta1.Shoot) error {
				metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, "shoot.gardener.cloud/skip-readiness", "")
				// Use maintain operation to also execute tasks in the reconcile flow which are only performed during maintenance.
				metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, "gardener.cloud/operation", "maintain")
				return nil
			})).To(Succeed())

			By("Wait for operation annotation to be gone (meaning controller picked up reconciliation request)")
			Eventually(func(g Gomega) {
				shoot := &gardencorev1beta1.Shoot{
					ObjectMeta: metav1.ObjectMeta{
						Name:      f.Shoot.Name,
						Namespace: f.Shoot.Namespace,
					},
				}

				g.Expect(f.GetShoot(ctx, shoot)).To(Succeed())
				g.Expect(shoot.Annotations).ToNot(HaveKey("gardener.cloud/operation"))
			}).Should(Succeed())

			Expect(f.WaitForShootToBeReconciled(ctx, f.Shoot)).To(Succeed())
			Expect(f.Shoot.Annotations).ToNot(HaveKey("shoot.gardener.cloud/skip-readiness"))

			By("Delete Shoot")
			ctx, cancel = context.WithTimeout(parentCtx, 20*time.Minute)
			defer cancel()
			Expect(f.DeleteShootAndWaitForDeletion(ctx, f.Shoot)).To(Succeed())
		})
	}

	Context("Shoot with workers", Label("basic"), func() {
		test(e2e.DefaultShoot("e2e-default"))
	})

	Context("Workerless Shoot", Label("workerless"), func() {
		test(e2e.DefaultWorkerlessShoot("e2e-default"))
	})
})
