// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package botanist_test

import (
	"context"

	"github.com/Masterminds/semver/v3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	. "github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
	shootpkg "github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
)

var _ = Describe("Botanist", func() {
	var (
		gardenClient              client.Client
		seedClient                client.Client
		botanist                  *Botanist
		gardenNamespace           *corev1.Namespace
		seedNamespace             *corev1.Namespace
		resourceManagerDeployment *appsv1.Deployment
	)

	BeforeEach(func(ctx context.Context) {
		seedClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		botanist = &Botanist{Operation: &operation.Operation{}}
		k8sSeedClient := fakekubernetes.NewClientSetBuilder().WithClient(seedClient).Build()

		gardenClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).WithStatusSubresource(&gardencorev1beta1.Shoot{}).Build()
		botanist.GardenClient = gardenClient

		gardenNamespace = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "garden-local"}}
		Expect(gardenClient.Create(ctx, gardenNamespace)).To(Succeed())
		DeferCleanup(func() {
			Expect(gardenClient.Delete(ctx, gardenNamespace)).To(Succeed())
		})

		seedNamespace = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: "botanist-"}}
		Expect(seedClient.Create(ctx, seedNamespace)).To(Succeed())
		DeferCleanup(func() {
			Expect(seedClient.Delete(ctx, seedNamespace)).To(Succeed())
		})

		botanist.SeedClientSet = k8sSeedClient
		botanist.Shoot = &shootpkg.Shoot{
			ControlPlaneNamespace: seedNamespace.Name,
			KubernetesVersion:     semver.MustParse("v1.31.0"),
		}

		resourceManagerDeployment = &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "gardener-resource-manager", Namespace: seedNamespace.Name}}
		Expect(seedClient.Create(ctx, resourceManagerDeployment)).To(Succeed())
		DeferCleanup(func() {
			Expect(seedClient.Delete(ctx, resourceManagerDeployment)).To(Succeed())
		})
	})

	Describe("#IsGardenerResourceManagerReady", func() {
		It("should return false if the gardener-resource-manager is not ready", func(ctx context.Context) {
			Expect(botanist.IsGardenerResourceManagerReady(ctx)).To(BeFalse())
		})

		It("should return true if the gardener-resource-manager is ready", func(ctx context.Context) {
			resourceManagerDeployment.Status.ReadyReplicas = 1
			Expect(seedClient.Status().Update(ctx, resourceManagerDeployment)).To(Succeed())

			Expect(botanist.IsGardenerResourceManagerReady(ctx)).To(BeTrue())
		})
	})

	Describe("#SetInPlaceUpdateStatus", func() {
		var (
			worker *extensionsv1alpha1.Worker
			shoot  *gardencorev1beta1.Shoot
		)

		BeforeEach(func() {
			shoot = &gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "shoot",
					Namespace: gardenNamespace.Name,
				},
				Spec: gardencorev1beta1.ShootSpec{
					Kubernetes: gardencorev1beta1.Kubernetes{
						Version: "1.32.0",
						Kubelet: &gardencorev1beta1.KubeletConfig{
							CPUManagerPolicy: new("static"),
						},
					},
					Provider: gardencorev1beta1.Provider{
						Workers: []gardencorev1beta1.Worker{
							{
								Name:           "pool-1",
								UpdateStrategy: new(gardencorev1beta1.AutoInPlaceUpdate),
								Machine: gardencorev1beta1.Machine{
									Image: &gardencorev1beta1.ShootMachineImage{
										Name:    "image-1",
										Version: new("1592.0"),
									},
								},
							},
							{
								Name:           "pool-2",
								UpdateStrategy: new(gardencorev1beta1.ManualInPlaceUpdate),
								Kubernetes: &gardencorev1beta1.WorkerKubernetes{
									Version: new("1.31.0"),
									Kubelet: &gardencorev1beta1.KubeletConfig{
										CPUManagerPolicy: new("none"),
									},
								},
								Machine: gardencorev1beta1.Machine{
									Image: &gardencorev1beta1.ShootMachineImage{
										Name:    "image-2",
										Version: new("1593.0"),
									},
								},
							},
						},
					},
				},
			}
		})

		Context("managed infra", func() {
			BeforeEach(func(ctx context.Context) {
				worker = &extensionsv1alpha1.Worker{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "worker",
						Namespace: seedNamespace.Name,
					},
					Spec: extensionsv1alpha1.WorkerSpec{
						Pools: []extensionsv1alpha1.WorkerPool{
							{
								Name:           "pool-0",
								UpdateStrategy: new(gardencorev1beta1.AutoRollingUpdate),
							},
							{
								Name:           "pool-1",
								UpdateStrategy: new(gardencorev1beta1.AutoInPlaceUpdate),
							},
							{
								Name:           "pool-2",
								UpdateStrategy: new(gardencorev1beta1.ManualInPlaceUpdate),
							},
							{
								Name:           "pool-3",
								UpdateStrategy: new(gardencorev1beta1.ManualInPlaceUpdate),
							},
						},
					},
					Status: extensionsv1alpha1.WorkerStatus{
						InPlaceUpdates: &extensionsv1alpha1.InPlaceUpdatesWorkerStatus{
							WorkerPoolToHashMap: map[string]string{
								"pool-1": "hash-1",
								"pool-2": "hash-2",
							},
						},
					},
				}

				shoot.Spec.CredentialsBindingName = new("my-secret")

				Expect(gardenClient.Create(ctx, shoot)).To(Succeed())
				botanist.Shoot.SetInfo(shoot)
			})

			It("should set the in-place update pending workers when worker is nil", func(ctx context.Context) {
				Expect(botanist.SetInPlaceUpdateStatus(ctx, nil)).To(Succeed())

				Expect(botanist.Shoot.GetInfo().Status.InPlaceUpdates).NotTo(BeNil())
				Expect(botanist.Shoot.GetInfo().Status.InPlaceUpdates.PendingWorkerUpdates).NotTo(BeNil())
				Expect(botanist.Shoot.GetInfo().Status.InPlaceUpdates.PendingWorkerUpdates.AutoInPlaceUpdate).To(ConsistOf("pool-1"))
				Expect(botanist.Shoot.GetInfo().Status.InPlaceUpdates.PendingWorkerUpdates.ManualInPlaceUpdate).To(ConsistOf("pool-2"))
			})

			It("should set the in-place update pending workers when worker does not contain this worker pool", func(ctx context.Context) {
				shoot.Spec.Provider.Workers = append(shoot.Spec.Provider.Workers, gardencorev1beta1.Worker{
					Name:           "pool-4",
					UpdateStrategy: new(gardencorev1beta1.ManualInPlaceUpdate),
				})
				botanist.Shoot.SetInfo(shoot)

				Expect(botanist.SetInPlaceUpdateStatus(ctx, worker)).To(Succeed())

				Expect(botanist.Shoot.GetInfo().Status.InPlaceUpdates).NotTo(BeNil())
				Expect(botanist.Shoot.GetInfo().Status.InPlaceUpdates.PendingWorkerUpdates.AutoInPlaceUpdate).To(ConsistOf("pool-1"))
				Expect(botanist.Shoot.GetInfo().Status.InPlaceUpdates.PendingWorkerUpdates.ManualInPlaceUpdate).To(ConsistOf("pool-2", "pool-4"))
			})

			It("should set the in-place update pending workers when worker is not nil but worker status map does not contain this worker pool", func(ctx context.Context) {
				worker.Status.InPlaceUpdates.WorkerPoolToHashMap = map[string]string{
					"pool-1": "hash-1",
				}

				Expect(botanist.SetInPlaceUpdateStatus(ctx, worker)).To(Succeed())

				Expect(botanist.Shoot.GetInfo().Status.InPlaceUpdates).NotTo(BeNil())
				Expect(botanist.Shoot.GetInfo().Status.InPlaceUpdates.PendingWorkerUpdates.AutoInPlaceUpdate).To(ConsistOf("pool-1"))
				Expect(botanist.Shoot.GetInfo().Status.InPlaceUpdates.PendingWorkerUpdates.ManualInPlaceUpdate).To(ConsistOf("pool-2"))
			})

			It("should set the in-place update pending workers when worker is not nil and hash is different", func(ctx context.Context) {
				Expect(botanist.SetInPlaceUpdateStatus(ctx, worker)).To(Succeed())

				Expect(botanist.Shoot.GetInfo().Status.InPlaceUpdates).NotTo(BeNil())
				Expect(botanist.Shoot.GetInfo().Status.InPlaceUpdates.PendingWorkerUpdates).NotTo(BeNil())
				Expect(botanist.Shoot.GetInfo().Status.InPlaceUpdates.PendingWorkerUpdates.AutoInPlaceUpdate).To(ConsistOf("pool-1"))
				Expect(botanist.Shoot.GetInfo().Status.InPlaceUpdates.PendingWorkerUpdates.ManualInPlaceUpdate).To(ConsistOf("pool-2"))
			})

			It("should not set the in-place update pending workers when worker is not nil and hash is same", func(ctx context.Context) {
				worker.Status.InPlaceUpdates.WorkerPoolToHashMap["pool-1"] = "c6cc5f56a36222bf"
				worker.Status.InPlaceUpdates.WorkerPoolToHashMap["pool-2"] = "8899f1cd0de77a6c"

				Expect(botanist.SetInPlaceUpdateStatus(ctx, worker)).To(Succeed())

				Expect(botanist.Shoot.GetInfo().Status.InPlaceUpdates).To(BeNil())
			})

			It("should not change the order of the workers when SetInPlaceUpdateStatus is called multiple times", func(ctx context.Context) {
				shoot.Spec.Provider.Workers = append(shoot.Spec.Provider.Workers,
					gardencorev1beta1.Worker{
						Name:           "pool-3",
						UpdateStrategy: new(gardencorev1beta1.AutoInPlaceUpdate),
					},
					gardencorev1beta1.Worker{
						Name:           "pool-4",
						UpdateStrategy: new(gardencorev1beta1.AutoInPlaceUpdate),
					},
					gardencorev1beta1.Worker{
						Name:           "pool-5",
						UpdateStrategy: new(gardencorev1beta1.ManualInPlaceUpdate),
					},
					gardencorev1beta1.Worker{
						Name:           "pool-6",
						UpdateStrategy: new(gardencorev1beta1.ManualInPlaceUpdate),
					},
				)
				botanist.Shoot.SetInfo(shoot)

				Expect(botanist.SetInPlaceUpdateStatus(ctx, nil)).To(Succeed())

				Expect(botanist.Shoot.GetInfo().Status.InPlaceUpdates).NotTo(BeNil())
				Expect(botanist.Shoot.GetInfo().Status.InPlaceUpdates.PendingWorkerUpdates.AutoInPlaceUpdate).To(Equal([]string{"pool-1", "pool-3", "pool-4"}))
				Expect(botanist.Shoot.GetInfo().Status.InPlaceUpdates.PendingWorkerUpdates.ManualInPlaceUpdate).To(Equal([]string{"pool-2", "pool-5", "pool-6"}))

				Expect(botanist.Shoot.UpdateInfo(ctx, gardenClient, false, func(shoot *gardencorev1beta1.Shoot) error {
					shoot.Spec.Provider.Workers = []gardencorev1beta1.Worker{
						{
							Name:           "pool-1",
							UpdateStrategy: new(gardencorev1beta1.AutoInPlaceUpdate),
						},
						{
							Name:           "pool-2",
							UpdateStrategy: new(gardencorev1beta1.ManualInPlaceUpdate),
						},
						{
							Name:           "pool-4",
							UpdateStrategy: new(gardencorev1beta1.AutoInPlaceUpdate),
						},
						{
							Name:           "pool-3",
							UpdateStrategy: new(gardencorev1beta1.AutoInPlaceUpdate),
						},
						{
							Name:           "pool-6",
							UpdateStrategy: new(gardencorev1beta1.ManualInPlaceUpdate),
						},
						{
							Name:           "pool-5",
							UpdateStrategy: new(gardencorev1beta1.ManualInPlaceUpdate),
						},
					}
					return nil
				})).To(Succeed())

				Expect(botanist.SetInPlaceUpdateStatus(ctx, nil)).To(Succeed())

				Expect(botanist.Shoot.GetInfo().Status.InPlaceUpdates).NotTo(BeNil())
				Expect(botanist.Shoot.GetInfo().Status.InPlaceUpdates.PendingWorkerUpdates.AutoInPlaceUpdate).To(Equal([]string{"pool-1", "pool-3", "pool-4"}))
				Expect(botanist.Shoot.GetInfo().Status.InPlaceUpdates.PendingWorkerUpdates.ManualInPlaceUpdate).To(Equal([]string{"pool-2", "pool-5", "pool-6"}))

				Expect(botanist.Shoot.UpdateInfo(ctx, gardenClient, false, func(shoot *gardencorev1beta1.Shoot) error {
					shoot.Spec.Provider.Workers = []gardencorev1beta1.Worker{
						{
							Name:           "pool-1",
							UpdateStrategy: new(gardencorev1beta1.AutoInPlaceUpdate),
						},
						{
							Name:           "pool-2",
							UpdateStrategy: new(gardencorev1beta1.ManualInPlaceUpdate),
						},
						{
							Name:           "pool-4",
							UpdateStrategy: new(gardencorev1beta1.AutoInPlaceUpdate),
						},
						{
							Name:           "pool-3",
							UpdateStrategy: new(gardencorev1beta1.AutoInPlaceUpdate),
						},
						{
							Name:           "pool-6",
							UpdateStrategy: new(gardencorev1beta1.ManualInPlaceUpdate),
						},
						{
							Name:           "pool-5",
							UpdateStrategy: new(gardencorev1beta1.ManualInPlaceUpdate),
						},
						{
							Name:           "pool-7",
							UpdateStrategy: new(gardencorev1beta1.AutoInPlaceUpdate),
						},
						{
							Name:           "pool-8",
							UpdateStrategy: new(gardencorev1beta1.ManualInPlaceUpdate),
						},
					}
					return nil
				})).To(Succeed())

				Expect(botanist.SetInPlaceUpdateStatus(ctx, nil)).To(Succeed())

				Expect(botanist.Shoot.GetInfo().Status.InPlaceUpdates).NotTo(BeNil())
				Expect(botanist.Shoot.GetInfo().Status.InPlaceUpdates.PendingWorkerUpdates.AutoInPlaceUpdate).To(Equal([]string{"pool-1", "pool-3", "pool-4", "pool-7"}))
				Expect(botanist.Shoot.GetInfo().Status.InPlaceUpdates.PendingWorkerUpdates.ManualInPlaceUpdate).To(Equal([]string{"pool-2", "pool-5", "pool-6", "pool-8"}))
			})

		})

		Context("unmanaged infra", func() {
			BeforeEach(func(ctx context.Context) {
				Expect(gardenClient.Create(ctx, shoot)).To(Succeed())
				botanist.Shoot.SetInfo(shoot)
			})

			It("should store initial hashes and not set pending workers on first reconciliation", func(ctx context.Context) {
				Expect(botanist.SetInPlaceUpdateStatus(ctx, nil)).To(Succeed())

				status := botanist.Shoot.GetInfo().Status.InPlaceUpdates
				Expect(status).NotTo(BeNil())
				Expect(status.WorkerPoolToHashMap).To(HaveKey("pool-1"))
				Expect(status.WorkerPoolToHashMap["pool-1"]).NotTo(BeEmpty())
				Expect(status.WorkerPoolToHashMap).To(HaveKey("pool-2"))
				Expect(status.WorkerPoolToHashMap["pool-2"]).NotTo(BeEmpty())
				Expect(status.PendingWorkerUpdates.AutoInPlaceUpdate).To(BeEmpty())
				Expect(status.PendingWorkerUpdates.ManualInPlaceUpdate).To(BeEmpty())
			})

			It("should set pending workers when the stored hash differs from the computed hash", func(ctx context.Context) {
				shoot.Status.InPlaceUpdates = &gardencorev1beta1.InPlaceUpdatesStatus{
					WorkerPoolToHashMap: map[string]string{
						"pool-1": "stale-hash-1",
						"pool-2": "stale-hash-2",
					},
				}
				Expect(gardenClient.Status().Update(ctx, shoot)).To(Succeed())
				botanist.Shoot.SetInfo(shoot)

				Expect(botanist.SetInPlaceUpdateStatus(ctx, nil)).To(Succeed())

				status := botanist.Shoot.GetInfo().Status.InPlaceUpdates
				Expect(status).NotTo(BeNil())
				Expect(status.PendingWorkerUpdates.AutoInPlaceUpdate).To(ConsistOf("pool-1"))
				Expect(status.PendingWorkerUpdates.ManualInPlaceUpdate).To(ConsistOf("pool-2"))
			})

			It("should only set pending workers for the pool with a stale hash", func(ctx context.Context) {
				// stores initial hashes for all pools.
				Expect(botanist.SetInPlaceUpdateStatus(ctx, nil)).To(Succeed())

				correctHashForPool1 := botanist.Shoot.GetInfo().Status.InPlaceUpdates.WorkerPoolToHashMap["pool-1"]
				Expect(correctHashForPool1).NotTo(BeEmpty())

				// pool-1 keeps its correct hash; pool-2 gets a stale one.
				updatedShoot := botanist.Shoot.GetInfo().DeepCopy()
				updatedShoot.Status.InPlaceUpdates = &gardencorev1beta1.InPlaceUpdatesStatus{
					WorkerPoolToHashMap: map[string]string{
						"pool-1": correctHashForPool1,
						"pool-2": "stale-hash-2",
					},
				}
				Expect(gardenClient.Status().Update(ctx, updatedShoot)).To(Succeed())
				botanist.Shoot.SetInfo(updatedShoot)

				Expect(botanist.SetInPlaceUpdateStatus(ctx, nil)).To(Succeed())

				status := botanist.Shoot.GetInfo().Status.InPlaceUpdates
				Expect(status).NotTo(BeNil())
				Expect(status.PendingWorkerUpdates.AutoInPlaceUpdate).To(BeEmpty())
				Expect(status.PendingWorkerUpdates.ManualInPlaceUpdate).To(ConsistOf("pool-2"))
			})

			It("should not set pending workers when the stored hash matches the computed hash", func(ctx context.Context) {
				// stores initial hashes.
				Expect(botanist.SetInPlaceUpdateStatus(ctx, nil)).To(Succeed())

				// hashes match, no pending workers should be added.
				Expect(botanist.SetInPlaceUpdateStatus(ctx, nil)).To(Succeed())

				status := botanist.Shoot.GetInfo().Status.InPlaceUpdates
				Expect(status).NotTo(BeNil())
				Expect(status.PendingWorkerUpdates.AutoInPlaceUpdate).To(BeEmpty())
				Expect(status.PendingWorkerUpdates.ManualInPlaceUpdate).To(BeEmpty())
			})
		})
	})
})
