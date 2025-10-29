// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package genericactuator

import (
	"context"

	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/testing"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	extensionsworkercontroller "github.com/gardener/gardener/extensions/pkg/controller/worker"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
)

var _ = Describe("ActuatorReconcile", func() {
	Describe("#deployMachineDeployments", func() {
		var (
			ctx                        context.Context
			log                        logr.Logger
			seedClient                 client.Client
			cluster                    *extensionscontroller.Cluster
			worker                     *extensionsv1alpha1.Worker
			existingMachineDeployments machinev1alpha1.MachineDeploymentList
			testDeployment             *machinev1alpha1.MachineDeployment
			returnedDeployment         machinev1alpha1.MachineDeployment
			wantedMachineDeployments   extensionsworkercontroller.MachineDeployments
			caUsed                     bool
		)

		BeforeEach(func() {
			// Starting with controller-runtime v0.22.0, the default object tracker does not work with resources which include
			// structs directly as pointer, e.g. *MachineConfiguration in Machine resource. Hence, use the old one instead.
			seedClient = fakeclient.NewClientBuilder().
				WithScheme(kubernetes.SeedScheme).
				WithStatusSubresource(&extensionsv1alpha1.Worker{}, &machinev1alpha1.MachineDeployment{}).
				WithObjectTracker(testing.NewObjectTracker(kubernetes.SeedScheme, scheme.Codecs.UniversalDecoder())).
				Build()

			ctx = context.Background()

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
					},
				},
			}
			Expect(seedClient.Create(ctx, worker)).To(Succeed())

			testDeployment = &machinev1alpha1.MachineDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "machine-deployment1",
					Namespace: worker.Namespace,
					Labels: map[string]string{
						"worker.gardener.cloud/name": worker.Name,
						"worker.gardener.cloud/pool": "pool1",
					},
					Annotations: map[string]string{
						"autoscaler.gardener.cloud/scale-down-utilization-threshold":     "0.3",
						"autoscaler.gardener.cloud/scale-down-gpu-utilization-threshold": "",
						"autoscaler.gardener.cloud/scale-down-unneeded-time":             "10m",
						"autoscaler.gardener.cloud/scale-down-unready-time":              "",
						"autoscaler.gardener.cloud/max-node-provision-time":              "",
					},
				},
			}

			Expect(seedClient.Create(ctx, testDeployment)).To(Succeed())
			wantedMachineDeployment := extensionsworkercontroller.MachineDeployment{
				Name:                         testDeployment.Name,
				PoolName:                     testDeployment.Labels["worker.gardener.cloud/pool"],
				Labels:                       testDeployment.Labels,
				ClusterAutoscalerAnnotations: testDeployment.Annotations,
			}
			wantedMachineDeployments = []extensionsworkercontroller.MachineDeployment{wantedMachineDeployment}

			DeferCleanup(func() {
				Expect(seedClient.Delete(ctx, worker)).To(Succeed())
				Expect(seedClient.Delete(ctx, testDeployment)).To(Succeed())
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
		It("should remove cluster autoscaler annotations with no values", func() {
			err := deployMachineDeployments(ctx, log, seedClient, cluster, worker, &existingMachineDeployments, wantedMachineDeployments, caUsed)
			Expect(err).NotTo(HaveOccurred())

			Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(worker), worker)).To(Succeed())
			Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(testDeployment), &returnedDeployment)).To(Succeed())
			Expect(returnedDeployment.Annotations).To(Equal(map[string]string{
				"autoscaler.gardener.cloud/scale-down-utilization-threshold": "0.3",
				"autoscaler.gardener.cloud/scale-down-unneeded-time":         "10m",
			}))
		})

		It("should remove all cluster autoscaler annotations", func() {
			testDeployment.Annotations = map[string]string{
				"autoscaler.gardener.cloud/scale-down-utilization-threshold":     "",
				"autoscaler.gardener.cloud/scale-down-gpu-utilization-threshold": "",
				"autoscaler.gardener.cloud/scale-down-unneeded-time":             "",
				"autoscaler.gardener.cloud/scale-down-unready-time":              "",
				"autoscaler.gardener.cloud/max-node-provision-time":              "",
			}
			wantedMachineDeployments[0].ClusterAutoscalerAnnotations = testDeployment.Annotations
			err := deployMachineDeployments(ctx, log, seedClient, cluster, worker, &existingMachineDeployments, wantedMachineDeployments, caUsed)
			Expect(err).NotTo(HaveOccurred())

			Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(worker), worker)).To(Succeed())
			Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(testDeployment), &returnedDeployment)).To(Succeed())
			Expect(returnedDeployment.Annotations).To(BeNil())
		})

		It("should not remove non-CA annotation and update CA annotations", func() {
			// Set non CA annotation
			Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(testDeployment), &returnedDeployment)).To(Succeed())
			metav1.SetMetaDataAnnotation(&returnedDeployment.ObjectMeta, "non-ca-annotation", "")
			Expect(seedClient.Update(ctx, &returnedDeployment)).To(Succeed())
			// Update existing CA annotation value and remove another CA annotation
			wantedMachineDeployments[0].ClusterAutoscalerAnnotations = map[string]string{
				"autoscaler.gardener.cloud/scale-down-utilization-threshold":     "",
				"autoscaler.gardener.cloud/scale-down-gpu-utilization-threshold": "",
				"autoscaler.gardener.cloud/scale-down-unneeded-time":             "20m",
				"autoscaler.gardener.cloud/scale-down-unready-time":              "",
				"autoscaler.gardener.cloud/max-node-provision-time":              "",
			}
			err := deployMachineDeployments(ctx, log, seedClient, cluster, worker, &existingMachineDeployments, wantedMachineDeployments, caUsed)
			Expect(err).NotTo(HaveOccurred())

			Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(worker), worker)).To(Succeed())
			Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(testDeployment), &returnedDeployment)).To(Succeed())
			Expect(returnedDeployment.ObjectMeta.Annotations).To(Equal(map[string]string{
				"non-ca-annotation": "",
				"autoscaler.gardener.cloud/scale-down-unneeded-time": "20m",
			}))
		})
	})
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
			// Starting with controller-runtime v0.22.0, the default object tracker does not work with resources which include
			// structs directly as pointer, e.g. *MachineConfiguration in Machine resource. Hence, use the old one instead.
			seedClient = fakeclient.NewClientBuilder().
				WithScheme(kubernetes.SeedScheme).
				WithStatusSubresource(&extensionsv1alpha1.Worker{}, &machinev1alpha1.MachineDeployment{}).
				WithObjectTracker(testing.NewObjectTracker(kubernetes.SeedScheme, scheme.Codecs.UniversalDecoder())).
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
