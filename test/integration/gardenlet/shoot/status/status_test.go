// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package status_test

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/features"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Shoot Status controller tests", func() {
	var (
		shoot  *gardencorev1beta1.Shoot
		worker *extensionsv1alpha1.Worker

		shootTechnicalID string
		shootNamespace   *corev1.Namespace
		cluster          *extensionsv1alpha1.Cluster
	)

	BeforeEach(func() {
		DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.InPlaceNodeUpdates, true))

		By("Create Shoot")
		shootName := "shoot-" + utils.ComputeSHA256Hex([]byte(uuid.NewUUID()))[:8]
		shoot = &gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				Name:      shootName,
				Namespace: projectNamespace.Name,
				UID:       "foo",
				Labels: map[string]string{
					testID: testRunID,
				},
				Annotations: map[string]string{
					v1beta1constants.GardenerOperation: v1beta1constants.ShootOperationForceInPlaceUpdate,
				},
			},
			Spec: gardencorev1beta1.ShootSpec{
				SeedName:          &seedName,
				SecretBindingName: ptr.To("test-sb"),
				CloudProfileName:  ptr.To("test-cloudprofile"),
				Region:            "foo-region",
				Kubernetes: gardencorev1beta1.Kubernetes{
					Version: "1.32.4",
					Kubelet: &gardencorev1beta1.KubeletConfig{
						CPUManagerPolicy: ptr.To("static"),
						EvictionHard: &gardencorev1beta1.KubeletConfigEviction{
							MemoryAvailable: ptr.To("100Mi"),
						},
						KubeReserved: &gardencorev1beta1.KubeletConfigReserved{
							CPU:    ptr.To(resource.MustParse("100m")),
							Memory: ptr.To(resource.MustParse("100Mi")),
						},
					},
				},
				Provider: gardencorev1beta1.Provider{
					Type: "provider",
					Workers: []gardencorev1beta1.Worker{
						{
							Name:    "worker1",
							Minimum: 2,
							Maximum: 2,
							Machine: gardencorev1beta1.Machine{
								Type: "large",
								Image: &gardencorev1beta1.ShootMachineImage{
									Name:    "image-1",
									Version: ptr.To("1.1.2"),
								},
							},
							Kubernetes: &gardencorev1beta1.WorkerKubernetes{
								Version: ptr.To("1.32.1"),
								Kubelet: &gardencorev1beta1.KubeletConfig{
									CPUManagerPolicy: ptr.To("static"),
									EvictionHard: &gardencorev1beta1.KubeletConfigEviction{
										MemoryAvailable: ptr.To("200Mi"),
									},
								},
							},
							UpdateStrategy: ptr.To(gardencorev1beta1.ManualInPlaceUpdate),
						},
						{
							Name:    "worker2",
							Minimum: 2,
							Maximum: 3,
							Machine: gardencorev1beta1.Machine{
								Type: "large",
								Image: &gardencorev1beta1.ShootMachineImage{
									Name:    "image-2",
									Version: ptr.To("1.3.0"),
								},
							},
							Kubernetes: &gardencorev1beta1.WorkerKubernetes{
								Version: ptr.To("1.31.1"),
							},
							UpdateStrategy: ptr.To(gardencorev1beta1.AutoInPlaceUpdate),
						},
						{
							Name:    "worker3",
							Minimum: 2,
							Maximum: 2,
							Machine: gardencorev1beta1.Machine{
								Type: "large",
								Image: &gardencorev1beta1.ShootMachineImage{
									Name:    "image-2",
									Version: ptr.To("1.2.1"),
								},
							},
							Kubernetes: &gardencorev1beta1.WorkerKubernetes{
								Version: ptr.To("1.31.1"),
								Kubelet: &gardencorev1beta1.KubeletConfig{
									KubeReserved: &gardencorev1beta1.KubeletConfigReserved{
										CPU: ptr.To(resource.MustParse("200m")),
									},
								},
							},
							UpdateStrategy: ptr.To(gardencorev1beta1.ManualInPlaceUpdate),
						},
						{
							Name:    "worker4",
							Minimum: 2,
							Maximum: 2,
							Machine: gardencorev1beta1.Machine{
								Type: "small",
								Image: &gardencorev1beta1.ShootMachineImage{
									Name:    "image-3",
									Version: ptr.To("1.2.0"),
								},
							},
							Kubernetes: &gardencorev1beta1.WorkerKubernetes{
								Version: ptr.To("1.30.1"),
							},
							UpdateStrategy: ptr.To(gardencorev1beta1.AutoRollingUpdate),
						},
						{
							Name:    "worker5",
							Minimum: 2,
							Maximum: 2,
							Machine: gardencorev1beta1.Machine{
								Type: "large",
								Image: &gardencorev1beta1.ShootMachineImage{
									Name:    "image-3",
									Version: ptr.To("1.1.0"),
								},
							},
							Kubernetes: &gardencorev1beta1.WorkerKubernetes{
								Version: ptr.To("1.32.3"),
								Kubelet: &gardencorev1beta1.KubeletConfig{
									CPUManagerPolicy: ptr.To("none"),
									KubeReserved: &gardencorev1beta1.KubeletConfigReserved{
										Memory: ptr.To(resource.MustParse("200Mi")),
									},
								},
							},
							UpdateStrategy: ptr.To(gardencorev1beta1.ManualInPlaceUpdate),
						},
					},
				},
				Networking: &gardencorev1beta1.Networking{
					Type: ptr.To("foo-networking"),
				},
			},
		}

		Expect(testClient.Create(ctx, shoot)).To(Succeed())
		log.Info("Created Shoot for test", "namespaceName", shoot.Name)

		By("Wait until manager has observed shoot")
		Eventually(func() error {
			return mgrClient.Get(ctx, client.ObjectKeyFromObject(shoot), &gardencorev1beta1.Shoot{})
		}).Should(Succeed())

		DeferCleanup(func() {
			By("Delete shoot")
			Expect(testClient.Delete(ctx, shoot)).To(Succeed())
			Eventually(func() error {
				return testClient.Get(ctx, client.ObjectKeyFromObject(shoot), &gardencorev1beta1.Shoot{})
			}).Should(BeNotFoundError())
		})

		shootTechnicalID = fmt.Sprintf("shoot--%s--%s", projectName, shootName)
		shootNamespace = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: shootTechnicalID,
			},
		}
		Expect(testClient.Create(ctx, shootNamespace)).To(Succeed())
		log.Info("Created namespace for test", "namespaceName", shootNamespace.Name)
		DeferCleanup(func() {
			By("Delete shoot namespace")
			Expect(testClient.Delete(ctx, shootNamespace)).To(Succeed())

			Eventually(func() error {
				return testClient.Get(ctx, client.ObjectKeyFromObject(shootNamespace), &corev1.Namespace{})
			}).Should(BeNotFoundError())
		})

		By("Create Cluster resource")
		cluster = &extensionsv1alpha1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name: shootTechnicalID,
			},
			Spec: extensionsv1alpha1.ClusterSpec{
				Shoot: runtime.RawExtension{
					Object: shoot,
				},
				Seed: runtime.RawExtension{
					Object: &gardencorev1beta1.Seed{},
				},
				CloudProfile: runtime.RawExtension{
					Object: &gardencorev1beta1.CloudProfile{},
				},
			},
		}

		Expect(testClient.Create(ctx, cluster)).To(Succeed())
		log.Info("Created cluster for test", "cluster", client.ObjectKeyFromObject(cluster))

		By("Ensure manager cache observes cluster creation")
		Eventually(func() error {
			return mgrClient.Get(ctx, client.ObjectKeyFromObject(cluster), &extensionsv1alpha1.Cluster{})
		}).Should(Succeed())

		DeferCleanup(func() {
			By("Delete cluster")
			Expect(testClient.Delete(ctx, cluster)).To(Or(Succeed(), BeNotFoundError()))
		})

		By("Create worker")
		worker = &extensionsv1alpha1.Worker{
			ObjectMeta: metav1.ObjectMeta{
				Name:      shootName,
				Namespace: shootTechnicalID,
			},
			Spec: extensionsv1alpha1.WorkerSpec{
				Pools: []extensionsv1alpha1.WorkerPool{
					{
						Name:              "worker1",
						Minimum:           2,
						Maximum:           2,
						MachineType:       "large",
						UpdateStrategy:    ptr.To(gardencorev1beta1.ManualInPlaceUpdate),
						KubernetesVersion: ptr.To("1.32.1"),
					},
					{
						Name:              "worker2",
						Minimum:           2,
						Maximum:           2,
						MachineType:       "large",
						UpdateStrategy:    ptr.To(gardencorev1beta1.AutoInPlaceUpdate),
						KubernetesVersion: ptr.To("1.31.1"),
					},
					{
						Name:              "worker3",
						Minimum:           2,
						Maximum:           2,
						MachineType:       "large",
						UpdateStrategy:    ptr.To(gardencorev1beta1.ManualInPlaceUpdate),
						KubernetesVersion: ptr.To("1.31.1"),
					},
					{
						Name:              "worker4",
						Minimum:           2,
						Maximum:           2,
						MachineType:       "large",
						UpdateStrategy:    ptr.To(gardencorev1beta1.AutoRollingUpdate),
						KubernetesVersion: ptr.To("1.30.1"),
					},
				},
			},
			Status: extensionsv1alpha1.WorkerStatus{
				InPlaceUpdates: &extensionsv1alpha1.InPlaceUpdatesWorkerStatus{},
			},
		}

		Expect(testClient.Create(ctx, worker)).To(Succeed())
		log.Info("Created worker for test", "worker", worker.Name)

		By("Update shoot status with pending in-place workers")
		patch := client.MergeFrom(shoot.DeepCopy())
		shoot.Status.TechnicalID = shootTechnicalID
		shoot.Status.InPlaceUpdates = &gardencorev1beta1.InPlaceUpdatesStatus{
			PendingWorkerUpdates: &gardencorev1beta1.PendingWorkerUpdates{
				AutoInPlaceUpdate:   []string{"worker2"},
				ManualInPlaceUpdate: []string{"worker1", "worker3", "worker5"},
			},
		}
		Expect(testClient.Status().Patch(ctx, shoot, patch)).To(Succeed())
	})

	It("should not remove the manual in-place update workers from Shoot status if the pool is not present in the worker status or the hash doesn't match", func() {
		patch := client.MergeFrom(worker.DeepCopy())
		workerPoolHashMap := map[string]string{
			"worker1": "ef492a9674e2778a",
			"worker2": "ecb9f30b6995e60d",
			"worker3": "different-hash",
		}
		worker.Status = extensionsv1alpha1.WorkerStatus{
			InPlaceUpdates: &extensionsv1alpha1.InPlaceUpdatesWorkerStatus{
				WorkerPoolToHashMap: workerPoolHashMap,
			},
		}
		Expect(testClient.Status().Patch(ctx, worker, patch)).To(Succeed())

		By("Waiting until the worker status is observed by the manager")
		Eventually(func(g Gomega) map[string]string {
			g.Expect(mgrClient.Get(ctx, client.ObjectKeyFromObject(worker), worker)).To(Succeed())
			g.Expect(worker.Status.InPlaceUpdates).NotTo(BeNil())
			return worker.Status.InPlaceUpdates.WorkerPoolToHashMap
		}).Should(Equal(workerPoolHashMap))

		Eventually(func(g Gomega) {
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
			g.Expect(shoot.Status.InPlaceUpdates).NotTo(BeNil())
			g.Expect(shoot.Status.InPlaceUpdates.PendingWorkerUpdates).NotTo(BeNil())
			// worker3 hash does not match, worker5 is not present in the worker status
			g.Expect(shoot.Status.InPlaceUpdates.PendingWorkerUpdates.ManualInPlaceUpdate).To(ConsistOf("worker3", "worker5"))
			// No change for auto in-place update workers
			g.Expect(shoot.Status.InPlaceUpdates.PendingWorkerUpdates.AutoInPlaceUpdate).To(ConsistOf("worker2"))
			g.Expect(shoot.Annotations).To(HaveKeyWithValue(v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationForceInPlaceUpdate))
		}).Should(Succeed())
	})

	It("should remove the manual in-place update workers from Shoot status and remove the force-update annotation if all hashes match", func() {
		shoot.Status.InPlaceUpdates.PendingWorkerUpdates.AutoInPlaceUpdate = nil
		Expect(testClient.Status().Update(ctx, shoot)).To(Succeed())

		By("Waiting until the shoot status is observed by the manager")
		Eventually(func(g Gomega) {
			g.Expect(mgrClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
			g.Expect(shoot.Status.InPlaceUpdates.PendingWorkerUpdates.AutoInPlaceUpdate).To(BeNil())
		}).Should(Succeed())

		patch := client.MergeFrom(worker.DeepCopy())
		workerPoolHashMap := map[string]string{
			"worker1": "ef492a9674e2778a",
			"worker3": "981b8e740cbbf058",
			"worker5": "2c12ce1fbb06b184",
		}
		worker.Status = extensionsv1alpha1.WorkerStatus{
			InPlaceUpdates: &extensionsv1alpha1.InPlaceUpdatesWorkerStatus{
				WorkerPoolToHashMap: workerPoolHashMap,
			},
		}
		Expect(testClient.Status().Patch(ctx, worker, patch)).To(Succeed())

		By("Waiting until the worker status is observed by the manager")
		Eventually(func(g Gomega) map[string]string {
			g.Expect(mgrClient.Get(ctx, client.ObjectKeyFromObject(worker), worker)).To(Succeed())
			g.Expect(worker.Status.InPlaceUpdates).NotTo(BeNil())
			return worker.Status.InPlaceUpdates.WorkerPoolToHashMap
		}).Should(Equal(workerPoolHashMap))

		Eventually(func(g Gomega) {
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
			g.Expect(shoot.Status.InPlaceUpdates).To(BeNil())
			g.Expect(shoot.Annotations).NotTo(HaveKey(v1beta1constants.GardenerOperation))
		}).Should(Succeed())
	})

	It("should not remove the force-update annotation if auto-inplace update workers are present", func() {
		patch := client.MergeFrom(worker.DeepCopy())
		workerPoolHashMap := map[string]string{
			"worker1": "ef492a9674e2778a",
			"worker3": "981b8e740cbbf058",
			"worker5": "2c12ce1fbb06b184",
		}
		worker.Status = extensionsv1alpha1.WorkerStatus{
			InPlaceUpdates: &extensionsv1alpha1.InPlaceUpdatesWorkerStatus{
				WorkerPoolToHashMap: workerPoolHashMap,
			},
		}
		Expect(testClient.Status().Patch(ctx, worker, patch)).To(Succeed())

		By("Waiting until the worker status is observed by the manager")
		Eventually(func(g Gomega) map[string]string {
			g.Expect(mgrClient.Get(ctx, client.ObjectKeyFromObject(worker), worker)).To(Succeed())
			g.Expect(worker.Status.InPlaceUpdates).NotTo(BeNil())
			return worker.Status.InPlaceUpdates.WorkerPoolToHashMap
		}).Should(Equal(workerPoolHashMap))

		Eventually(func(g Gomega) {
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
			g.Expect(shoot.Status.InPlaceUpdates).NotTo(BeNil())
			g.Expect(shoot.Status.InPlaceUpdates.PendingWorkerUpdates).NotTo(BeNil())
			g.Expect(shoot.Status.InPlaceUpdates.PendingWorkerUpdates.ManualInPlaceUpdate).To(BeNil())
			g.Expect(shoot.Status.InPlaceUpdates.PendingWorkerUpdates.AutoInPlaceUpdate).To(ConsistOf("worker2"))
			g.Expect(shoot.Annotations).To(HaveKeyWithValue(v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationForceInPlaceUpdate))
		}).Should(Succeed())
	})
})
