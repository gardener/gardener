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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/testing"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	extensionsworkercontroller "github.com/gardener/gardener/extensions/pkg/controller/worker"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/gardener/shootstate"
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
			testDeploymentObjectKey    client.ObjectKey
		)

		buildWantedMachineDeployment := func(suffix string, strategyType machinev1alpha1.MachineDeploymentStrategyType, taints []corev1.Taint) extensionsworkercontroller.MachineDeployment {
			md := extensionsworkercontroller.MachineDeployment{
				Name:      "machine-deployment" + suffix,
				PoolName:  "pool" + suffix,
				ClassName: "test-machine-class-" + suffix,
				Labels: map[string]string{
					"worker.gardener.cloud/name": worker.Name,
					"worker.gardener.cloud/pool": "pool" + suffix,
				},
				Annotations: map[string]string{
					"node-annotation-1": "ann-value1",
				},
				ClusterAutoscalerAnnotations: map[string]string{
					"autoscaler.gardener.cloud/scale-down-utilization-threshold":     "0.3",
					"autoscaler.gardener.cloud/scale-down-gpu-utilization-threshold": "",
					"autoscaler.gardener.cloud/scale-down-unneeded-time":             "10m",
					"autoscaler.gardener.cloud/scale-down-unready-time":              "",
					"autoscaler.gardener.cloud/max-node-provision-time":              "",
				},
				Minimum: 2,
				Maximum: 10,
				Taints:  taints,
			}

			if strategyType == machinev1alpha1.RollingUpdateMachineDeploymentStrategyType {
				md.Strategy = machinev1alpha1.MachineDeploymentStrategy{
					Type: machinev1alpha1.RollingUpdateMachineDeploymentStrategyType,
					RollingUpdate: &machinev1alpha1.RollingUpdateMachineDeployment{
						UpdateConfiguration: machinev1alpha1.UpdateConfiguration{
							MaxSurge:       ptr.To(intstr.FromInt32(1)),
							MaxUnavailable: ptr.To(intstr.FromInt32(0)),
						},
					},
				}
			}

			if strategyType == machinev1alpha1.InPlaceUpdateMachineDeploymentStrategyType {
				md.Strategy = machinev1alpha1.MachineDeploymentStrategy{
					Type: machinev1alpha1.InPlaceUpdateMachineDeploymentStrategyType,
					InPlaceUpdate: &machinev1alpha1.InPlaceUpdateMachineDeployment{
						UpdateConfiguration: machinev1alpha1.UpdateConfiguration{
							MaxSurge:       ptr.To(intstr.FromInt32(1)),
							MaxUnavailable: ptr.To(intstr.FromInt32(0)),
						},
					},
				}
			}

			return md
		}

		buildExpectedMachineDeployment := func(namespace string, replicas int32, resourceVersion string, machineDeployment extensionsworkercontroller.MachineDeployment) *machinev1alpha1.MachineDeployment {
			templateLabels := map[string]string{
				"name": machineDeployment.Name,
			}

			if machineDeployment.Strategy.Type == machinev1alpha1.InPlaceUpdateMachineDeploymentStrategyType {
				templateLabels = utils.MergeStringMaps(templateLabels, map[string]string{
					v1beta1constants.LabelWorkerName: "worker",
				})
			}

			md := &machinev1alpha1.MachineDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: namespace,
					Name:      machineDeployment.Name,
					Labels: map[string]string{
						"worker.gardener.cloud/pool": machineDeployment.PoolName,
					},
					ResourceVersion: resourceVersion,
				},
				Spec: machinev1alpha1.MachineDeploymentSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"name": machineDeployment.Name,
						},
					},
					Strategy:             machineDeployment.Strategy,
					Replicas:             replicas,
					MinReadySeconds:      500,
					RevisionHistoryLimit: ptr.To[int32](0),
					Template: machinev1alpha1.MachineTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: templateLabels,
						},
						Spec: machinev1alpha1.MachineSpec{
							Class: machinev1alpha1.ClassSpec{
								Kind: "MachineClass",
								Name: machineDeployment.ClassName,
							},
							NodeTemplateSpec: machinev1alpha1.NodeTemplateSpec{
								ObjectMeta: metav1.ObjectMeta{
									Labels:      machineDeployment.Labels,
									Annotations: machineDeployment.Annotations,
								},
								Spec: corev1.NodeSpec{
									Taints: machineDeployment.Taints,
								},
							},
						},
					},
				},
			}

			for k, v := range machineDeployment.ClusterAutoscalerAnnotations {
				if v != "" {
					metav1.SetMetaDataAnnotation(&md.ObjectMeta, k, v)
				}
			}

			return md
		}

		BeforeEach(func() {
			// Starting with controller-runtime v0.22.0, the default object tracker does not work with resources which include
			// structs directly as pointer, e.g. *MachineConfiguration in Machine resource. Hence, use the old one instead.
			seedClient = fakeclient.NewClientBuilder().
				WithScheme(kubernetes.SeedScheme).
				WithStatusSubresource(&machinev1alpha1.MachineDeployment{}).
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

			wantedMachineDeployment := buildWantedMachineDeployment("1", machinev1alpha1.RollingUpdateMachineDeploymentStrategyType, nil)
			testDeploymentObjectKey = client.ObjectKey{Namespace: "namespace", Name: wantedMachineDeployment.Name}
			wantedMachineDeployments = []extensionsworkercontroller.MachineDeployment{wantedMachineDeployment}

			cluster = &extensionscontroller.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster",
				},
				Shoot: &gardencorev1beta1.Shoot{
					Status: gardencorev1beta1.ShootStatus{},
				},
			}
		})

		testReplicaCount := func(setup func(), caUsed bool, expectedReplicas int) {
			if setup != nil {
				setup()
			}

			err := deployMachineDeployments(ctx, log, seedClient, cluster, worker, &existingMachineDeployments, wantedMachineDeployments, caUsed)
			Expect(err).NotTo(HaveOccurred())

			Expect(seedClient.Get(ctx, testDeploymentObjectKey, &returnedDeployment)).To(Succeed())
			Expect(returnedDeployment.Spec.Replicas).To(Equal(int32(expectedReplicas)))
		}

		testMachineDeployments := func(wantedMachineDeployments extensionsworkercontroller.MachineDeployments, expectedMachineDeployments []*machinev1alpha1.MachineDeployment) {
			err := deployMachineDeployments(ctx, log, seedClient, cluster, worker, &existingMachineDeployments, wantedMachineDeployments, true)
			Expect(err).NotTo(HaveOccurred())

			for _, expectedMachineDeployment := range expectedMachineDeployments {
				returnedDeployment := &machinev1alpha1.MachineDeployment{}
				Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(expectedMachineDeployment), returnedDeployment)).To(Succeed(), expectedMachineDeployment.Name+" should be retrieved successfully")
				Expect(returnedDeployment).To(Equal(expectedMachineDeployment), "should be equal to expected machine deployment "+expectedMachineDeployment.Name)
			}
		}

		When("there are no existing MachineDeployments", func() {
			It("should correctly deploy multiple machine deployments", func() {
				wantedMachineDeployments = append(wantedMachineDeployments,
					buildWantedMachineDeployment("2", machinev1alpha1.InPlaceUpdateMachineDeploymentStrategyType,
						[]corev1.Taint{
							{
								Key:    "taint-key-2",
								Value:  "taint-value-2",
								Effect: corev1.TaintEffectNoExecute,
							},
						},
					),
				)

				expectedMachineDeployments := []*machinev1alpha1.MachineDeployment{
					buildExpectedMachineDeployment("namespace", 2, "1", wantedMachineDeployments[0]),
					buildExpectedMachineDeployment("namespace", 2, "1", wantedMachineDeployments[1]),
				}

				testMachineDeployments(wantedMachineDeployments, expectedMachineDeployments)
			})

			DescribeTable("verify replica count", testReplicaCount,
				Entry("should use replicas from state when restoring machine deployment", func() {
					wantedMachineDeployments[0].State = &shootstate.MachineDeploymentState{
						Replicas: 5,
					}
				}, true, 5),
				Entry("should use min replicas when restoring machine deployment without cluster autoscaler", func() {
					wantedMachineDeployments[0].State = &shootstate.MachineDeploymentState{
						Replicas: 5,
					}
					wantedMachineDeployments[0].Maximum = wantedMachineDeployments[0].Minimum
				}, false, 2),
				Entry("should use min replicas when creating a machine deployment", nil, true, 2),
				Entry("should use min replicas when creating a machine deployment without cluster autoscaler", func() {
					wantedMachineDeployments[0].Maximum = wantedMachineDeployments[0].Minimum
				}, false, 2),
			)
		})

		When("there are existing MachineDeployments", func() {
			BeforeEach(func() {
				testDeployment = &machinev1alpha1.MachineDeployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      wantedMachineDeployments[0].Name,
						Namespace: worker.Namespace,
						Labels: map[string]string{
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
					Spec: machinev1alpha1.MachineDeploymentSpec{
						Replicas: 4,
					},
				}
				Expect(seedClient.Create(ctx, testDeployment)).To(Succeed())

				Expect(seedClient.List(ctx, &existingMachineDeployments)).To(Succeed())
			})

			It("should remove all cluster autoscaler annotations", func() {
				wantedMachineDeployments[0].ClusterAutoscalerAnnotations = map[string]string{
					"autoscaler.gardener.cloud/scale-down-utilization-threshold":     "",
					"autoscaler.gardener.cloud/scale-down-gpu-utilization-threshold": "",
					"autoscaler.gardener.cloud/scale-down-unneeded-time":             "",
					"autoscaler.gardener.cloud/scale-down-unready-time":              "",
					"autoscaler.gardener.cloud/max-node-provision-time":              "",
				}

				err := deployMachineDeployments(ctx, log, seedClient, cluster, worker, &existingMachineDeployments, wantedMachineDeployments, false)
				Expect(err).NotTo(HaveOccurred())

				Expect(seedClient.Get(ctx, testDeploymentObjectKey, &returnedDeployment)).To(Succeed())
				Expect(returnedDeployment.Annotations).To(BeNil())
			})

			It("should not remove non-CA annotation and update CA annotations", func() {
				metav1.SetMetaDataAnnotation(&testDeployment.ObjectMeta, "non-ca-annotation", "")
				Expect(seedClient.Update(ctx, testDeployment)).To(Succeed())
				Expect(seedClient.List(ctx, &existingMachineDeployments)).To(Succeed())

				wantedMachineDeployments[0].ClusterAutoscalerAnnotations = map[string]string{
					"autoscaler.gardener.cloud/scale-down-utilization-threshold":     "",
					"autoscaler.gardener.cloud/scale-down-gpu-utilization-threshold": "",
					"autoscaler.gardener.cloud/scale-down-unneeded-time":             "20m",
					"autoscaler.gardener.cloud/scale-down-unready-time":              "",
					"autoscaler.gardener.cloud/max-node-provision-time":              "",
				}
				err := deployMachineDeployments(ctx, log, seedClient, cluster, worker, &existingMachineDeployments, wantedMachineDeployments, false)
				Expect(err).NotTo(HaveOccurred())

				Expect(seedClient.Get(ctx, testDeploymentObjectKey, &returnedDeployment)).To(Succeed())
				Expect(returnedDeployment.ObjectMeta.Annotations).To(And(HaveKeyWithValue("non-ca-annotation", ""), HaveKeyWithValue("autoscaler.gardener.cloud/scale-down-unneeded-time", "20m")))
			})

			It("should correctly deploy multiple machine deployments", func() {
				wantedMachineDeployments = append(wantedMachineDeployments,
					buildWantedMachineDeployment("2", machinev1alpha1.InPlaceUpdateMachineDeploymentStrategyType,
						[]corev1.Taint{
							{
								Key:    "taint-key-2",
								Value:  "taint-value-2",
								Effect: corev1.TaintEffectNoExecute,
							},
						},
					),
				)

				expectedMachineDeployments := []*machinev1alpha1.MachineDeployment{
					buildExpectedMachineDeployment("namespace", 4, "2", wantedMachineDeployments[0]),
					buildExpectedMachineDeployment("namespace", 2, "1", wantedMachineDeployments[1]),
				}

				testMachineDeployments(wantedMachineDeployments, expectedMachineDeployments)
			})

			DescribeTable("verify replica count", testReplicaCount,
				Entry("should keep existing replicas when restoring machine deployment", func() {
					wantedMachineDeployments[0].State = &shootstate.MachineDeploymentState{
						Replicas: 4,
					}
				}, true, 4),
				Entry("should set replicas to min when restoring machine deployment without cluster autoscaler", func() {
					wantedMachineDeployments[0].State = &shootstate.MachineDeploymentState{
						Replicas: 5,
					}
					wantedMachineDeployments[0].Maximum = wantedMachineDeployments[0].Minimum
				}, false, 2),
				Entry("should keep existing replicas when deploying a machine deployment", nil, true, 4),
				Entry("should set replicas to min when deploying a machine deployment without cluster autoscaler", func() {
					wantedMachineDeployments[0].Maximum = wantedMachineDeployments[0].Minimum
				}, false, 2),
				Entry("should set replicas to 0 when shoot is hibernated", func() {
					cluster.Shoot.Spec.Hibernation = &gardencorev1beta1.Hibernation{
						Enabled: ptr.To(true),
					}
				}, true, 0),
				Entry("should set replicas to min when cluster autoscaler is not used", func() {
					wantedMachineDeployments[0].Maximum = wantedMachineDeployments[0].Minimum
				}, false, 2),
				Entry("should set replicas to min when replicas are below minimum", func() {
					testDeployment.Spec.Replicas = 1
					Expect(seedClient.Update(ctx, testDeployment)).To(Succeed())
					Expect(seedClient.List(ctx, &existingMachineDeployments)).To(Succeed())
				}, true, 2),
				Entry("should set replicas to max when replicas are above minimum", func() {
					testDeployment.Spec.Replicas = 15
					Expect(seedClient.Update(ctx, testDeployment)).To(Succeed())
					Expect(seedClient.List(ctx, &existingMachineDeployments)).To(Succeed())
				}, true, 10),
			)

			It("should mark all machines for forceful deletion when shoot is hibernated", func() {
				machine1 := &machinev1alpha1.Machine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "machine-1",
						Namespace: worker.Namespace,
					},
				}
				machine2 := &machinev1alpha1.Machine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "machine-2",
						Namespace: worker.Namespace,
						Labels: map[string]string{
							"worker.gardener.cloud/name": worker.Name,
						},
					},
				}
				machine3 := &machinev1alpha1.Machine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "machine-3",
						Namespace: worker.Namespace,
						Labels: map[string]string{
							"worker.gardener.cloud/name": worker.Name,
							"force-deletion":             "True",
						},
					},
				}

				for _, machine := range []*machinev1alpha1.Machine{machine1, machine2, machine3} {
					Expect(seedClient.Create(ctx, machine)).To(Succeed(), machine.Name+" should be created successfully")
				}

				cluster.Shoot.Spec.Hibernation = &gardencorev1beta1.Hibernation{
					Enabled: ptr.To(true),
				}

				err := deployMachineDeployments(ctx, log, seedClient, cluster, worker, &existingMachineDeployments, wantedMachineDeployments, false)
				Expect(err).NotTo(HaveOccurred())

				for _, machine := range []*machinev1alpha1.Machine{machine1, machine2, machine3} {
					var returnedMachine machinev1alpha1.Machine
					Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(machine), &returnedMachine)).To(Succeed(), machine.Name+" should be retrieved successfully")
					Expect(returnedMachine.Labels).To(HaveKeyWithValue("force-deletion", "True"), machine.Name+" should have `force-deletion: True` annotation")
				}
			})

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
