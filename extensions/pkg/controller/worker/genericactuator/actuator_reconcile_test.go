// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package genericactuator

import (
	"context"

	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
)

var _ = Describe("ActuatorReconcile", func() {
	Describe("#updateWorkerStatusInPlaceUpdateWorkerPoolHash", func() {
		var (
			ctx        context.Context
			seedClient client.Client

			actuator *genericActuator
			worker   *extensionsv1alpha1.Worker
			cluster  *extensionscontroller.Cluster

			machineDeployment1 *machinev1alpha1.MachineDeployment
			machineDeployment2 *machinev1alpha1.MachineDeployment
		)

		BeforeEach(func() {
			seedClient = fakeclient.NewClientBuilder().
				WithScheme(kubernetes.SeedScheme).
				WithStatusSubresource(&extensionsv1alpha1.Worker{}, &machinev1alpha1.MachineDeployment{}).
				Build()

			ctx = context.Background()

			actuator = &genericActuator{
				seedClient: seedClient,
			}

			worker = &extensionsv1alpha1.Worker{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "worker",
					Namespace: "namespace",
				},
				Spec: extensionsv1alpha1.WorkerSpec{
					Pools: []extensionsv1alpha1.WorkerPool{
						{
							Name:              "pool1",
							UpdateStrategy:    ptr.To(gardencorev1beta1.AutoInPlaceUpdate),
							KubernetesVersion: ptr.To("1.32.0"),
						},
						{
							Name:              "pool2",
							UpdateStrategy:    ptr.To(gardencorev1beta1.ManualInPlaceUpdate),
							KubernetesVersion: ptr.To("1.31.0"),
						},
						{
							Name:              "pool3",
							UpdateStrategy:    ptr.To(gardencorev1beta1.AutoRollingUpdate),
							KubernetesVersion: ptr.To("1.31.0"),
						},
					},
				},
				Status: extensionsv1alpha1.WorkerStatus{
					InPlaceUpdates: &extensionsv1alpha1.InPlaceUpdatesWorkerStatus{
						WorkerPoolToHashMap: map[string]string{
							"pool1": "89e7871a154fd3d0",
							"pool2": "04863233bf6b9bb0",
						},
					},
				},
			}
			Expect(seedClient.Create(ctx, worker)).To(Succeed())

			machineDeployment1 = &machinev1alpha1.MachineDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "machine-deployment1",
					Namespace: worker.Namespace,
					Labels: map[string]string{
						"worker.gardener.cloud/name": worker.Name,
						"worker.gardener.cloud/pool": "pool2",
					},
				},
				Status: machinev1alpha1.MachineDeploymentStatus{
					Replicas:        2,
					UpdatedReplicas: 2,
				},
			}
			Expect(seedClient.Create(ctx, machineDeployment1)).To(Succeed())

			machineDeployment2 = &machinev1alpha1.MachineDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "machine-deployment2",
					Namespace: worker.Namespace,
					Labels: map[string]string{
						"worker.gardener.cloud/name": worker.Name,
						"worker.gardener.cloud/pool": "pool2",
					},
				},
				Status: machinev1alpha1.MachineDeploymentStatus{
					Replicas:        3,
					UpdatedReplicas: 3,
				},
			}
			Expect(seedClient.Create(ctx, machineDeployment2)).To(Succeed())

			DeferCleanup(func() {
				Expect(seedClient.Delete(ctx, worker)).To(Succeed())
				Expect(seedClient.Delete(ctx, machineDeployment1)).To(Succeed())
				Expect(seedClient.Delete(ctx, machineDeployment2)).To(Succeed())
			})

			cluster = &extensionscontroller.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster",
				},
				Shoot: &gardencorev1beta1.Shoot{
					Status: gardencorev1beta1.ShootStatus{},
				},
			}
		})

		It("should do nothing because the maps in worker status and inPlaceWorkerPoolToHashMap are same", func() {
			err := actuator.updateWorkerStatusInPlaceUpdateWorkerPoolHash(ctx, worker, cluster)
			Expect(err).NotTo(HaveOccurred())

			Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(worker), worker)).To(Succeed())
			Expect(worker.Status.InPlaceUpdates.WorkerPoolToHashMap).To(Equal(map[string]string{
				"pool1": "89e7871a154fd3d0",
				"pool2": "04863233bf6b9bb0",
			}))
		})

		It("should not add non in-place update worker pools to the worker status", func() {
			worker.Spec.Pools[0].UpdateStrategy = ptr.To(gardencorev1beta1.AutoRollingUpdate)

			err := actuator.updateWorkerStatusInPlaceUpdateWorkerPoolHash(ctx, worker, cluster)
			Expect(err).NotTo(HaveOccurred())

			Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(worker), worker)).To(Succeed())
			Expect(worker.Status.InPlaceUpdates.WorkerPoolToHashMap).To(Equal(map[string]string{
				"pool2": "04863233bf6b9bb0",
			}))
		})

		It("should remove the worker pools from the worker status because they are not present in the worker anymore", func() {
			worker.Spec.Pools = []extensionsv1alpha1.WorkerPool{
				{
					Name:              "pool1",
					UpdateStrategy:    ptr.To(gardencorev1beta1.AutoInPlaceUpdate),
					KubernetesVersion: ptr.To("1.32.0"),
				},
				{
					Name:              "pool3",
					UpdateStrategy:    ptr.To(gardencorev1beta1.AutoInPlaceUpdate),
					KubernetesVersion: ptr.To("1.32.0"),
				},
			}
			Expect(seedClient.Update(ctx, worker)).To(Succeed())

			err := actuator.updateWorkerStatusInPlaceUpdateWorkerPoolHash(ctx, worker, cluster)
			Expect(err).NotTo(HaveOccurred())

			Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(worker), worker)).To(Succeed())
			Expect(worker.Status.InPlaceUpdates.WorkerPoolToHashMap).To(Equal(map[string]string{
				"pool1": "89e7871a154fd3d0",
				"pool3": "89e7871a154fd3d0",
			}))
		})

		It("should update the worker status because the maps in worker status and inPlaceWorkerPoolToHashMap are different", func() {
			Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(worker), worker)).To(Succeed())
			Expect(worker.Status.InPlaceUpdates.WorkerPoolToHashMap).To(Equal(map[string]string{
				"pool1": "89e7871a154fd3d0",
				"pool2": "04863233bf6b9bb0",
			}))

			worker.Spec.Pools[0].KubernetesVersion = ptr.To("1.33.0")
			worker.Spec.Pools[1].KubernetesVersion = ptr.To("1.31.0")
			worker.Spec.Pools[2].KubernetesVersion = ptr.To("1.32.0")

			err := actuator.updateWorkerStatusInPlaceUpdateWorkerPoolHash(ctx, worker, cluster)
			Expect(err).NotTo(HaveOccurred())

			Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(worker), worker)).To(Succeed())
			Expect(worker.Status.InPlaceUpdates.WorkerPoolToHashMap).To(Equal(map[string]string{
				"pool1": "e1d8805194ff0b5e",
				"pool2": "04863233bf6b9bb0",
			}))
		})

		It("should not update the worker pools in the  status because the pool has strategy manual in-place and some machinedeployments have not been updated", func() {
			Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(worker), worker)).To(Succeed())
			Expect(worker.Status.InPlaceUpdates.WorkerPoolToHashMap).To(Equal(map[string]string{
				"pool1": "89e7871a154fd3d0",
				"pool2": "04863233bf6b9bb0",
			}))

			machineDeployment2.Status.UpdatedReplicas = 2
			machineDeployment2.Status.Replicas = 3
			Expect(seedClient.Status().Update(ctx, machineDeployment2)).To(Succeed())

			worker.Spec.Pools[0].KubernetesVersion = ptr.To("1.33.0")
			worker.Spec.Pools[1].KubernetesVersion = ptr.To("1.32.0")
			worker.Spec.Pools[2].KubernetesVersion = ptr.To("1.31.0")

			err := actuator.updateWorkerStatusInPlaceUpdateWorkerPoolHash(ctx, worker, cluster)
			Expect(err).NotTo(HaveOccurred())

			Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(worker), worker)).To(Succeed())
			Expect(worker.Status.InPlaceUpdates.WorkerPoolToHashMap).To(Equal(map[string]string{
				"pool1": "e1d8805194ff0b5e",
				"pool2": "04863233bf6b9bb0",
			}))
		})
	})
})
