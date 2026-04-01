// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package general_test

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"github.com/gardener/gardener/extensions/pkg/controller/healthcheck/general"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

var _ = Describe("Deployment", func() {
	const (
		deploymentName = "test-deployment"
		namespace      = "test-namespace"
	)

	var (
		ctx     context.Context
		request types.NamespacedName
	)

	BeforeEach(func() {
		ctx = context.Background()
		request = types.NamespacedName{Namespace: namespace, Name: "extension-resource"}
	})

	Describe("SeedDeploymentHealthChecker", func() {
		var checker *general.SeedDeploymentHealthChecker

		BeforeEach(func() {
			checker = general.NewSeedDeploymentHealthChecker(deploymentName)
			checker.SetLoggerSuffix("test-provider", "test-extension")
		})

		Context("when the deployment does not exist", func() {
			BeforeEach(func() {
				fakeClient := fake.NewClientBuilder().Build()
				checker.InjectSourceClient(fakeClient)
			})

			It("should return ConditionFalse with not found detail", func() {
				result, err := checker.Check(ctx, request)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				Expect(result.Status).To(Equal(gardencorev1beta1.ConditionFalse))
				Expect(result.Detail).To(ContainSubstring("not found"))
				Expect(result.Detail).To(ContainSubstring(deploymentName))
				Expect(result.Detail).To(ContainSubstring(namespace))
			})
		})

		Context("when the deployment is healthy", func() {
			BeforeEach(func() {
				deployment := newHealthyDeployment()
				fakeClient := fake.NewClientBuilder().WithObjects(deployment).Build()
				checker.InjectSourceClient(fakeClient)
			})

			It("should return ConditionTrue", func() {
				result, err := checker.Check(ctx, request)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				Expect(result.Status).To(Equal(gardencorev1beta1.ConditionTrue))
				Expect(result.Detail).To(BeEmpty())
			})
		})

		Context("when the deployment has outdated observed generation", func() {
			BeforeEach(func() {
				deployment := newHealthyDeployment()
				deployment.Generation = 2
				deployment.Status.ObservedGeneration = 1
				fakeClient := fake.NewClientBuilder().WithObjects(deployment).Build()
				checker.InjectSourceClient(fakeClient)
			})

			It("should return ConditionFalse", func() {
				result, err := checker.Check(ctx, request)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				Expect(result.Status).To(Equal(gardencorev1beta1.ConditionFalse))
				Expect(result.Detail).To(ContainSubstring("unhealthy"))
			})
		})

		Context("when the deployment is missing the Available condition", func() {
			BeforeEach(func() {
				deployment := &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:       deploymentName,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: appsv1.DeploymentStatus{
						ObservedGeneration: 1,
						Conditions:         []appsv1.DeploymentCondition{},
					},
				}
				fakeClient := fake.NewClientBuilder().WithObjects(deployment).Build()
				checker.InjectSourceClient(fakeClient)
			})

			It("should return ConditionFalse because Available condition is required", func() {
				result, err := checker.Check(ctx, request)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				Expect(result.Status).To(Equal(gardencorev1beta1.ConditionFalse))
				Expect(result.Detail).To(ContainSubstring("unhealthy"))
			})
		})

		Context("when the deployment has Available=False", func() {
			BeforeEach(func() {
				deployment := &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:       deploymentName,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: appsv1.DeploymentStatus{
						ObservedGeneration: 1,
						Conditions: []appsv1.DeploymentCondition{
							{
								Type:    appsv1.DeploymentAvailable,
								Status:  corev1.ConditionFalse,
								Reason:  "MinimumReplicasUnavailable",
								Message: "Deployment does not have minimum availability.",
							},
						},
					},
				}
				fakeClient := fake.NewClientBuilder().WithObjects(deployment).Build()
				checker.InjectSourceClient(fakeClient)
			})

			It("should return ConditionFalse", func() {
				result, err := checker.Check(ctx, request)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				Expect(result.Status).To(Equal(gardencorev1beta1.ConditionFalse))
				Expect(result.Detail).To(ContainSubstring("unhealthy"))
			})
		})

		Context("when the deployment has Progressing=False", func() {
			BeforeEach(func() {
				deployment := &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:       deploymentName,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: appsv1.DeploymentStatus{
						ObservedGeneration: 1,
						Conditions: []appsv1.DeploymentCondition{
							{
								Type:   appsv1.DeploymentAvailable,
								Status: corev1.ConditionTrue,
							},
							{
								Type:    appsv1.DeploymentProgressing,
								Status:  corev1.ConditionFalse,
								Reason:  "ProgressDeadlineExceeded",
								Message: "Deployment has exceeded its progress deadline.",
							},
						},
					},
				}
				fakeClient := fake.NewClientBuilder().WithObjects(deployment).Build()
				checker.InjectSourceClient(fakeClient)
			})

			It("should return ConditionFalse", func() {
				result, err := checker.Check(ctx, request)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				Expect(result.Status).To(Equal(gardencorev1beta1.ConditionFalse))
				Expect(result.Detail).To(ContainSubstring("unhealthy"))
			})
		})

		Context("when the deployment has ReplicaFailure=True", func() {
			BeforeEach(func() {
				deployment := &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:       deploymentName,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: appsv1.DeploymentStatus{
						ObservedGeneration: 1,
						Conditions: []appsv1.DeploymentCondition{
							{
								Type:   appsv1.DeploymentAvailable,
								Status: corev1.ConditionTrue,
							},
							{
								Type:   appsv1.DeploymentProgressing,
								Status: corev1.ConditionTrue,
							},
							{
								Type:    appsv1.DeploymentReplicaFailure,
								Status:  corev1.ConditionTrue,
								Reason:  "FailedCreate",
								Message: "quota exceeded",
							},
						},
					},
				}
				fakeClient := fake.NewClientBuilder().WithObjects(deployment).Build()
				checker.InjectSourceClient(fakeClient)
			})

			It("should return ConditionFalse", func() {
				result, err := checker.Check(ctx, request)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				Expect(result.Status).To(Equal(gardencorev1beta1.ConditionFalse))
				Expect(result.Detail).To(ContainSubstring("unhealthy"))
			})
		})

		Context("when the client returns a non-NotFound error", func() {
			BeforeEach(func() {
				fakeClient := fake.NewClientBuilder().WithInterceptorFuncs(interceptor.Funcs{
					Get: func(_ context.Context, _ client.WithWatch, _ client.ObjectKey, _ client.Object, _ ...client.GetOption) error {
						return fmt.Errorf("internal error")
					},
				}).Build()
				checker.InjectSourceClient(fakeClient)
			})

			It("should return an error", func() {
				result, err := checker.Check(ctx, request)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to retrieve deployment"))
				Expect(result).To(BeNil())
			})
		})
	})

	Describe("ShootDeploymentHealthChecker", func() {
		var checker *general.ShootDeploymentHealthChecker

		BeforeEach(func() {
			checker = general.NewShootDeploymentHealthChecker(deploymentName)
			checker.SetLoggerSuffix("test-provider", "test-extension")
		})

		Context("when the deployment does not exist", func() {
			BeforeEach(func() {
				fakeClient := fake.NewClientBuilder().Build()
				checker.InjectTargetClient(fakeClient)
			})

			It("should return ConditionFalse with not found detail", func() {
				result, err := checker.Check(ctx, request)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				Expect(result.Status).To(Equal(gardencorev1beta1.ConditionFalse))
				Expect(result.Detail).To(ContainSubstring("not found"))
			})
		})

		Context("when the deployment is healthy", func() {
			BeforeEach(func() {
				deployment := newHealthyDeployment()
				fakeClient := fake.NewClientBuilder().WithObjects(deployment).Build()
				checker.InjectTargetClient(fakeClient)
			})

			It("should return ConditionTrue", func() {
				result, err := checker.Check(ctx, request)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				Expect(result.Status).To(Equal(gardencorev1beta1.ConditionTrue))
			})
		})

		Context("when the deployment is unhealthy", func() {
			BeforeEach(func() {
				deployment := newHealthyDeployment()
				deployment.Generation = 2
				deployment.Status.ObservedGeneration = 1
				fakeClient := fake.NewClientBuilder().WithObjects(deployment).Build()
				checker.InjectTargetClient(fakeClient)
			})

			It("should return ConditionFalse", func() {
				result, err := checker.Check(ctx, request)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				Expect(result.Status).To(Equal(gardencorev1beta1.ConditionFalse))
				Expect(result.Detail).To(ContainSubstring("unhealthy"))
			})
		})

		Context("when the client returns a non-NotFound error", func() {
			BeforeEach(func() {
				fakeClient := fake.NewClientBuilder().WithInterceptorFuncs(interceptor.Funcs{
					Get: func(_ context.Context, _ client.WithWatch, _ client.ObjectKey, _ client.Object, _ ...client.GetOption) error {
						return fmt.Errorf("internal error")
					},
				}).Build()
				checker.InjectTargetClient(fakeClient)
			})

			It("should return an error", func() {
				result, err := checker.Check(ctx, request)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to retrieve deployment"))
				Expect(result).To(BeNil())
			})
		})
	})
})

func newHealthyDeployment() *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-deployment",
			Namespace:  "test-namespace",
			Generation: 1,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: ptr.To[int32](1),
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "test"}},
		},
		Status: appsv1.DeploymentStatus{
			ObservedGeneration: 1,
			Replicas:           1,
			UpdatedReplicas:    1,
			AvailableReplicas:  1,
			Conditions: []appsv1.DeploymentCondition{
				{
					Type:   appsv1.DeploymentAvailable,
					Status: corev1.ConditionTrue,
				},
				{
					Type:   appsv1.DeploymentProgressing,
					Status: corev1.ConditionTrue,
				},
			},
		},
	}
}
