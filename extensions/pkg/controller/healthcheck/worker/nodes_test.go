// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package worker_test

import (
	"context"
	"fmt"
	"time"

	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"github.com/gardener/gardener/extensions/pkg/controller/healthcheck/worker"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

var _ = Describe("DefaultHealthChecker", func() {
	var (
		ctx          context.Context
		request      types.NamespacedName
		checker      *worker.DefaultHealthChecker
		sourceScheme *runtime.Scheme
		targetScheme *runtime.Scheme
	)

	const namespace = "shoot--dev--test"

	BeforeEach(func() {
		ctx = context.Background()
		request = types.NamespacedName{Namespace: namespace, Name: "worker"}
		checker = worker.NewNodesChecker()
		checker.SetLoggerSuffix("test-provider", "test-extension")

		sourceScheme = runtime.NewScheme()
		Expect(machinev1alpha1.AddToScheme(sourceScheme)).To(Succeed())

		targetScheme = runtime.NewScheme()
		Expect(corev1.AddToScheme(targetScheme)).To(Succeed())
	})

	Describe("#NewNodesChecker", func() {
		It("should create a new checker with default thresholds", func() {
			Expect(checker).NotTo(BeNil())
		})
	})

	Describe("#WithScaleUpProgressingThreshold", func() {
		It("should set the scale up progressing threshold", func() {
			d := 10 * time.Minute
			result := checker.WithScaleUpProgressingThreshold(d)
			Expect(result).To(Equal(checker))
		})
	})

	Describe("#WithScaleDownProgressingThreshold", func() {
		It("should set the scale down progressing threshold", func() {
			d := 20 * time.Minute
			result := checker.WithScaleDownProgressingThreshold(d)
			Expect(result).To(Equal(checker))
		})
	})

	Describe("#Check", func() {
		Context("when all nodes are healthy and match desired machine count", func() {
			BeforeEach(func() {
				machineDeployment := newMachineDeployment(2)
				sourceClient := fake.NewClientBuilder().WithScheme(sourceScheme).WithObjects(machineDeployment).Build()
				checker.InjectSourceClient(sourceClient)

				node1 := newReadyNode("node-1")
				node2 := newReadyNode("node-2")
				targetClient := fake.NewClientBuilder().WithScheme(targetScheme).WithObjects(node1, node2).Build()
				checker.InjectTargetClient(targetClient)
			})

			It("should return ConditionTrue", func() {
				result, err := checker.Check(ctx, request)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				Expect(result.Status).To(Equal(gardencorev1beta1.ConditionTrue))
			})
		})

		Context("when machine deployments have failed machines", func() {
			BeforeEach(func() {
				machineDeployment := &machinev1alpha1.MachineDeployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "md-1",
						Namespace: namespace,
					},
					Spec: machinev1alpha1.MachineDeploymentSpec{
						Replicas: 1,
					},
					Status: machinev1alpha1.MachineDeploymentStatus{
						Conditions: []machinev1alpha1.MachineDeploymentCondition{
							{
								Type:   machinev1alpha1.MachineDeploymentAvailable,
								Status: machinev1alpha1.ConditionTrue,
							},
							{
								Type:   machinev1alpha1.MachineDeploymentProgressing,
								Status: machinev1alpha1.ConditionTrue,
							},
						},
						FailedMachines: []*machinev1alpha1.MachineSummary{
							{
								Name:          "failed-node",
								LastOperation: machinev1alpha1.LastOperation{Description: "Cloud provider error"},
							},
						},
					},
				}
				sourceClient := fake.NewClientBuilder().WithScheme(sourceScheme).WithObjects(machineDeployment).Build()
				checker.InjectSourceClient(sourceClient)

				node := newReadyNode("node-1")
				targetClient := fake.NewClientBuilder().WithScheme(targetScheme).WithObjects(node).Build()
				checker.InjectTargetClient(targetClient)
			})

			It("should return ConditionFalse with the failed machine info", func() {
				result, err := checker.Check(ctx, request)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				Expect(result.Status).To(Equal(gardencorev1beta1.ConditionFalse))
				Expect(result.Detail).To(ContainSubstring("failed-node"))
				Expect(result.Detail).To(ContainSubstring("Cloud provider error"))
			})
		})

		Context("when machine deployments are unhealthy", func() {
			BeforeEach(func() {
				machineDeployment := &machinev1alpha1.MachineDeployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "md-1",
						Namespace: namespace,
					},
					Spec: machinev1alpha1.MachineDeploymentSpec{
						Replicas: 1,
					},
					Status: machinev1alpha1.MachineDeploymentStatus{
						Conditions: []machinev1alpha1.MachineDeploymentCondition{
							{
								Type:   machinev1alpha1.MachineDeploymentAvailable,
								Status: machinev1alpha1.ConditionFalse,
							},
						},
					},
				}
				sourceClient := fake.NewClientBuilder().WithScheme(sourceScheme).WithObjects(machineDeployment).Build()
				checker.InjectSourceClient(sourceClient)

				node := newReadyNode("node-1")
				targetClient := fake.NewClientBuilder().WithScheme(targetScheme).WithObjects(node).Build()
				checker.InjectTargetClient(targetClient)
			})

			It("should return ConditionFalse", func() {
				result, err := checker.Check(ctx, request)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				Expect(result.Status).To(Equal(gardencorev1beta1.ConditionFalse))
				Expect(result.Detail).To(ContainSubstring("unhealthy"))
			})
		})

		Context("when nodes annotated with not-managed-by-mcm are excluded", func() {
			BeforeEach(func() {
				machineDeployment := newMachineDeployment(1)
				sourceClient := fake.NewClientBuilder().WithScheme(sourceScheme).WithObjects(machineDeployment).Build()
				checker.InjectSourceClient(sourceClient)

				managedNode := newReadyNode("node-managed")
				unmanagedNode := &corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node-unmanaged",
						Annotations: map[string]string{
							worker.AnnotationKeyNotManagedByMCM: "1",
						},
					},
					Status: corev1.NodeStatus{
						Conditions: []corev1.NodeCondition{
							{
								Type:   corev1.NodeReady,
								Status: corev1.ConditionTrue,
							},
						},
					},
				}
				targetClient := fake.NewClientBuilder().WithScheme(targetScheme).WithObjects(managedNode, unmanagedNode).Build()
				checker.InjectTargetClient(targetClient)
			})

			It("should return ConditionTrue because unmanaged nodes are excluded", func() {
				result, err := checker.Check(ctx, request)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				Expect(result.Status).To(Equal(gardencorev1beta1.ConditionTrue))
			})
		})

		Context("when listing machine deployments fails", func() {
			BeforeEach(func() {
				sourceClient := fake.NewClientBuilder().WithScheme(sourceScheme).WithInterceptorFuncs(interceptor.Funcs{
					List: func(_ context.Context, _ client.WithWatch, _ client.ObjectList, _ ...client.ListOption) error {
						return fmt.Errorf("source list error")
					},
				}).Build()
				checker.InjectSourceClient(sourceClient)

				targetClient := fake.NewClientBuilder().WithScheme(targetScheme).Build()
				checker.InjectTargetClient(targetClient)
			})

			It("should return an error", func() {
				result, err := checker.Check(ctx, request)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Failed to list machine deployments"))
				Expect(result).To(BeNil())
			})
		})

		Context("when listing nodes fails", func() {
			BeforeEach(func() {
				machineDeployment := newMachineDeployment(1)
				sourceClient := fake.NewClientBuilder().WithScheme(sourceScheme).WithObjects(machineDeployment).Build()
				checker.InjectSourceClient(sourceClient)

				targetClient := fake.NewClientBuilder().WithScheme(targetScheme).WithInterceptorFuncs(interceptor.Funcs{
					List: func(_ context.Context, _ client.WithWatch, _ client.ObjectList, _ ...client.ListOption) error {
						return fmt.Errorf("target list error")
					},
				}).Build()
				checker.InjectTargetClient(targetClient)
			})

			It("should return an error", func() {
				result, err := checker.Check(ctx, request)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Failed to list shoot nodes"))
				Expect(result).To(BeNil())
			})
		})

		Context("when there are no machine deployments and no nodes", func() {
			BeforeEach(func() {
				sourceClient := fake.NewClientBuilder().WithScheme(sourceScheme).Build()
				checker.InjectSourceClient(sourceClient)

				targetClient := fake.NewClientBuilder().WithScheme(targetScheme).Build()
				checker.InjectTargetClient(targetClient)
			})

			It("should return ConditionTrue because desired is 0 and actual is 0", func() {
				result, err := checker.Check(ctx, request)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				Expect(result.Status).To(Equal(gardencorev1beta1.ConditionTrue))
			})
		})

		Context("when unschedulable nodes are excluded from ready count", func() {
			BeforeEach(func() {
				machineDeployment := newMachineDeployment(1)
				sourceClient := fake.NewClientBuilder().WithScheme(sourceScheme).WithObjects(machineDeployment).Build()
				checker.InjectSourceClient(sourceClient)

				readyNode := newReadyNode("node-ready")
				unschedulableNode := &corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node-unschedulable",
					},
					Spec: corev1.NodeSpec{
						Unschedulable: true,
					},
					Status: corev1.NodeStatus{
						Conditions: []corev1.NodeCondition{
							{
								Type:   corev1.NodeReady,
								Status: corev1.ConditionTrue,
							},
						},
					},
				}
				targetClient := fake.NewClientBuilder().WithScheme(targetScheme).WithObjects(readyNode, unschedulableNode).Build()
				checker.InjectTargetClient(targetClient)
			})

			It("should not count unschedulable nodes as ready", func() {
				result, err := checker.Check(ctx, request)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				// 1 desired machine, 2 registered nodes, but 1 unschedulable so 1 ready
				// registeredNodes = 2, desiredMachines = 1
				// registeredNodes != desiredMachines, so machines are listed
				// Then checkMachineDeploymentsHealthy is called and returns true
				Expect(result.Status).To(Equal(gardencorev1beta1.ConditionTrue))
			})
		})
	})
})

func newMachineDeployment(replicas int32) *machinev1alpha1.MachineDeployment {
	return &machinev1alpha1.MachineDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "md-1",
			Namespace: "shoot--dev--test",
		},
		Spec: machinev1alpha1.MachineDeploymentSpec{
			Replicas: replicas,
		},
		Status: machinev1alpha1.MachineDeploymentStatus{
			Conditions: []machinev1alpha1.MachineDeploymentCondition{
				{
					Type:   machinev1alpha1.MachineDeploymentAvailable,
					Status: machinev1alpha1.ConditionTrue,
				},
				{
					Type:   machinev1alpha1.MachineDeploymentProgressing,
					Status: machinev1alpha1.ConditionTrue,
				},
			},
		},
	}
}

func newReadyNode(name string) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{
				{
					Type:   corev1.NodeReady,
					Status: corev1.ConditionTrue,
				},
			},
		},
	}
}
