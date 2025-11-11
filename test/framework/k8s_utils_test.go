// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package framework_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	. "github.com/gardener/gardener/test/framework"
	shootoperation "github.com/gardener/gardener/test/utils/shoots/operation"
)

var _ = Describe("Kubernetes Utils", func() {
	Describe("#GetAllNodesInWorkerPool", func() {
		var (
			ctx           context.Context
			fakeClient    client.Client
			fakeInterface kubernetes.Interface

			workerPool string
		)

		BeforeEach(func() {
			ctx = context.Background()
			fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetesscheme.Scheme).Build()
			fakeInterface = fakekubernetes.NewClientSetBuilder().WithAPIReader(fakeClient).WithClient(fakeClient).Build()
		})

		It("should return all nodes when no worker pool is specified", func() {
			node1 := &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "node1",
				},
			}
			node2 := &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "node2",
				},
			}

			Expect(fakeClient.Create(ctx, node1)).To(Succeed())
			Expect(fakeClient.Create(ctx, node2)).To(Succeed())

			nodes, err := GetAllNodesInWorkerPool(ctx, fakeInterface, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(nodes.Items).To(HaveLen(2))
			Expect(nodes.Items).To(ContainElement(*node1))
			Expect(nodes.Items).To(ContainElement(*node2))
		})

		It("should return nodes belonging to a specific worker pool", func() {
			workerPool = "worker-pool-1"
			node1 := &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "node1",
					Labels: map[string]string{"worker.gardener.cloud/pool": workerPool},
				},
			}
			node2 := &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "node2",
					Labels: map[string]string{"worker.gardener.cloud/pool": "worker-pool-2"},
				},
			}

			Expect(fakeClient.Create(ctx, node1)).To(Succeed())
			Expect(fakeClient.Create(ctx, node2)).To(Succeed())

			nodes, err := GetAllNodesInWorkerPool(ctx, fakeInterface, &workerPool)
			Expect(err).NotTo(HaveOccurred())
			Expect(nodes.Items).To(HaveLen(1))
			Expect(nodes.Items[0].Name).To(Equal("node1"))
		})

		It("should return an empty list if no nodes match the worker pool", func() {
			workerPool = "non-existent-pool"
			node := &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "node1",
					Labels: map[string]string{"worker.gardener.cloud/pool": "worker-pool-1"},
				},
			}

			Expect(fakeClient.Create(ctx, node)).To(Succeed())

			nodes, err := GetAllNodesInWorkerPool(ctx, fakeInterface, &workerPool)
			Expect(err).NotTo(HaveOccurred())
			Expect(nodes.Items).To(BeEmpty())
		})
	})

	Describe("#ShootReconciliationSuccessful", func() {
		var (
			shoot *gardencorev1beta1.Shoot

			testShootReconciliationSuccessful = func(matchMessage, matchResult types.GomegaMatcher) {
				successful, msg := shootoperation.ReconciliationSuccessful(shoot)
				Expect(msg).To(matchMessage)
				Expect(successful).To(matchResult)
			}
		)

		BeforeEach(func() {
			shoot = &gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Generation: 1,
				},
				Spec: gardencorev1beta1.ShootSpec{
					Provider: gardencorev1beta1.Provider{
						Workers: []gardencorev1beta1.Worker{
							{Name: "worker"},
						},
					},
				},
				Status: gardencorev1beta1.ShootStatus{
					ObservedGeneration: 1,
				},
			}
		})

		Context("when lastOperation is Succeeded and all conditions are True", func() {
			BeforeEach(func() {
				shoot.Status.LastOperation = &gardencorev1beta1.LastOperation{
					Type:  gardencorev1beta1.LastOperationTypeCreate,
					State: gardencorev1beta1.LastOperationStateSucceeded,
				}
			})

			It("should return true if shoot is reconciled successfully", func() {
				appendShootConditionsToShoot(shoot)

				testShootReconciliationSuccessful(BeEmpty(), BeTrue())
			})

			It("should return true if workerless shoot is reconciled successfully", func() {
				shoot.Spec.Provider.Workers = nil
				appendShootConditionsToShoot(shoot)

				testShootReconciliationSuccessful(BeEmpty(), BeTrue())
			})

			It("should return true if shoot which acts as seed is reconciled successfully", func() {
				appendShootConditionsToShoot(shoot)
				appendSeedConditionsToShoot(shoot)

				testShootReconciliationSuccessful(BeEmpty(), BeTrue())
			})
		})

		Context("when generation is outdated", func() {
			It("should return false and appropriate message", func() {
				shoot.Status.ObservedGeneration = 0

				testShootReconciliationSuccessful(ContainSubstring("generation did not equal observed generation"), BeFalse())
			})
		})

		Context("when lastOperation and conditions are not set", func() {
			It("should return false and appropriate message", func() {
				shoot.Status.ObservedGeneration = 1

				testShootReconciliationSuccessful(ContainSubstring("no conditions and last operation present yet"), BeFalse())
			})
		})

		Context("when not all conditions are True", func() {
			It("should return false and appropriate message if not all conditions are True", func() {
				appendShootConditionsToShoot(shoot)
				setConditionToFalse(shoot, gardencorev1beta1.ShootSystemComponentsHealthy)

				testShootReconciliationSuccessful(ContainSubstring("condition type SystemComponentsHealthy is not true yet"), BeFalse())
			})

			It("should return false and appropriate message if not all conditions are True for workerless shoot", func() {
				shoot.Spec.Provider.Workers = nil
				appendShootConditionsToShoot(shoot)
				setConditionToFalse(shoot, gardencorev1beta1.ShootControlPlaneHealthy)

				testShootReconciliationSuccessful(ContainSubstring("condition type ControlPlaneHealthy is not true yet"), BeFalse())
			})

			It("should return false and appropriate message if shoot acts as seed and a seed condition is not True", func() {
				appendShootConditionsToShoot(shoot)
				appendSeedConditionsToShoot(shoot)
				setConditionToFalse(shoot, gardencorev1beta1.SeedExtensionsReady)

				testShootReconciliationSuccessful(ContainSubstring("condition type ExtensionsReady is not true yet"), BeFalse())
			})

			It("should return false and appropriate message if shoot acts as seed, not all shoot conditions are true and shoot is being hibernated", func() {
				shoot.Spec.Hibernation = &gardencorev1beta1.Hibernation{
					Enabled: ptr.To(true),
				}

				appendShootConditionsToShoot(shoot)
				appendSeedConditionsToShoot(shoot)
				setConditionToFalse(shoot, gardencorev1beta1.ShootSystemComponentsHealthy)

				testShootReconciliationSuccessful(ContainSubstring("condition type SystemComponentsHealthy is not true yet"), BeFalse())
			})

			It("should return true and empty message if shoot acts as seed, not all seed conditions are true and shoot is being hibernated", func() {
				shoot.Spec.Hibernation = &gardencorev1beta1.Hibernation{
					Enabled: ptr.To(true),
				}

				appendShootConditionsToShoot(shoot)
				appendSeedConditionsToShoot(shoot)
				setConditionToFalse(shoot, gardencorev1beta1.SeedExtensionsReady)

				testShootReconciliationSuccessful(BeEmpty(), BeTrue())
			})
		})

		Context("when lastOperation is not Succeeded", func() {
			BeforeEach(func() {
				appendShootConditionsToShoot(shoot)
			})

			DescribeTable("when lastOperation is",
				func(lastOperation *gardencorev1beta1.LastOperation, matchMessage, matchResult types.GomegaMatcher) {
					shoot.Status.LastOperation = lastOperation
					testShootReconciliationSuccessful(matchMessage, matchResult)
				},
				Entry("Create",
					&gardencorev1beta1.LastOperation{
						Type:  gardencorev1beta1.LastOperationTypeCreate,
						State: gardencorev1beta1.LastOperationStateFailed,
					},
					ContainSubstring("last operation type was create, reconcile or restore but state was not succeeded"),
					BeFalse(),
				),
				Entry("Reconcile",
					&gardencorev1beta1.LastOperation{
						Type:  gardencorev1beta1.LastOperationTypeReconcile,
						State: gardencorev1beta1.LastOperationStateFailed,
					},
					ContainSubstring("last operation type was create, reconcile or restore but state was not succeeded"),
					BeFalse(),
				),
				Entry("Migrate Failed",
					&gardencorev1beta1.LastOperation{
						Type:  gardencorev1beta1.LastOperationTypeMigrate,
						State: gardencorev1beta1.LastOperationStateFailed,
					},
					ContainSubstring("last operation type was migrate, the migration process is not finished yet"),
					BeFalse(),
				),
				Entry("Mgrate Succeeded",
					&gardencorev1beta1.LastOperation{
						Type:  gardencorev1beta1.LastOperationTypeMigrate,
						State: gardencorev1beta1.LastOperationStateSucceeded,
					},
					ContainSubstring("last operation type was migrate, the migration process is not finished yet"),
					BeFalse(),
				),
				Entry("Restore",
					&gardencorev1beta1.LastOperation{
						Type:  gardencorev1beta1.LastOperationTypeMigrate,
						State: gardencorev1beta1.LastOperationStateFailed,
					},
					ContainSubstring("last operation type was migrate, the migration process is not finished yet"),
					BeFalse(),
				),
			)
		})
	})
})

func appendShootConditionsToShoot(shoot *gardencorev1beta1.Shoot) {
	shoot.Status.Conditions = append(shoot.Status.Conditions, []gardencorev1beta1.Condition{
		{
			Type:   gardencorev1beta1.ShootAPIServerAvailable,
			Status: gardencorev1beta1.ConditionTrue,
		},
		{
			Type:   gardencorev1beta1.ShootControlPlaneHealthy,
			Status: gardencorev1beta1.ConditionTrue,
		},
		{
			Type:   gardencorev1beta1.ShootObservabilityComponentsHealthy,
			Status: gardencorev1beta1.ConditionTrue,
		},
		{
			Type:   gardencorev1beta1.ShootSystemComponentsHealthy,
			Status: gardencorev1beta1.ConditionTrue,
		},
	}...,
	)

	if !v1beta1helper.IsWorkerless(shoot) {
		shoot.Status.Conditions = append(shoot.Status.Conditions, []gardencorev1beta1.Condition{
			{
				Type:   gardencorev1beta1.ShootEveryNodeReady,
				Status: gardencorev1beta1.ConditionTrue,
			},
		}...,
		)
	}
}

func appendSeedConditionsToShoot(shoot *gardencorev1beta1.Shoot) {
	shoot.Status.Conditions = append(shoot.Status.Conditions, []gardencorev1beta1.Condition{
		{
			Type:   gardencorev1beta1.GardenletReady,
			Status: gardencorev1beta1.ConditionTrue,
		},
		{
			Type:   gardencorev1beta1.SeedBackupBucketsReady,
			Status: gardencorev1beta1.ConditionTrue,
		},
		{
			Type:   gardencorev1beta1.SeedSystemComponentsHealthy,
			Status: gardencorev1beta1.ConditionTrue,
		},
		{
			Type:   gardencorev1beta1.SeedExtensionsReady,
			Status: gardencorev1beta1.ConditionTrue,
		},
	}...)
}

func setConditionToFalse(shoot *gardencorev1beta1.Shoot, conditionType gardencorev1beta1.ConditionType) {
	for i, condition := range shoot.Status.Conditions {
		if condition.Type == conditionType {
			shoot.Status.Conditions[i].Status = gardencorev1beta1.ConditionFalse
			return
		}
	}
}
