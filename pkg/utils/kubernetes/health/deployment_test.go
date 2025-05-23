// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package health_test

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
)

var _ = Describe("Deployment", func() {
	DescribeTable("#CheckDeployment",
		func(deployment *appsv1.Deployment, matcher types.GomegaMatcher) {
			err := health.CheckDeployment(deployment)
			Expect(err).To(matcher)
		},
		Entry("healthy", &appsv1.Deployment{
			Status: appsv1.DeploymentStatus{Conditions: []appsv1.DeploymentCondition{
				{
					Type:   appsv1.DeploymentAvailable,
					Status: corev1.ConditionTrue,
				},
			}},
		}, BeNil()),
		Entry("healthy with progressing", &appsv1.Deployment{
			Status: appsv1.DeploymentStatus{Conditions: []appsv1.DeploymentCondition{
				{
					Type:   appsv1.DeploymentAvailable,
					Status: corev1.ConditionTrue,
				},
				{
					Type:   appsv1.DeploymentProgressing,
					Status: corev1.ConditionTrue,
				},
			}},
		}, BeNil()),
		Entry("not observed at latest version", &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Generation: 1},
		}, HaveOccurred()),
		Entry("not available", &appsv1.Deployment{
			Status: appsv1.DeploymentStatus{Conditions: []appsv1.DeploymentCondition{
				{
					Type:   appsv1.DeploymentAvailable,
					Status: corev1.ConditionFalse,
				},
				{
					Type:   appsv1.DeploymentProgressing,
					Status: corev1.ConditionTrue,
				},
			}},
		}, HaveOccurred()),
		Entry("not progressing", &appsv1.Deployment{
			Status: appsv1.DeploymentStatus{Conditions: []appsv1.DeploymentCondition{
				{
					Type:   appsv1.DeploymentAvailable,
					Status: corev1.ConditionTrue,
				},
				{
					Type:   appsv1.DeploymentProgressing,
					Status: corev1.ConditionFalse,
				},
			}},
		}, HaveOccurred()),
		Entry("available | progressing missing", &appsv1.Deployment{}, HaveOccurred()),
	)

	Describe("#IsDeploymentProgressing", func() {
		var deployment *appsv1.Deployment

		BeforeEach(func() {
			deployment = &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Generation: 42,
				},
				Status: appsv1.DeploymentStatus{
					ObservedGeneration: 42,
					Conditions: []appsv1.DeploymentCondition{{
						Type:    appsv1.DeploymentProgressing,
						Status:  corev1.ConditionTrue,
						Reason:  "NewReplicaSetAvailable",
						Message: `ReplicaSet "nginx-66b6c48dd5" has successfully progressed.`,
					}},
				},
			}
		})

		It("should return false if it is fully rolled out", func() {
			progressing, reason := health.IsDeploymentProgressing(deployment)
			Expect(progressing).To(BeFalse())
			Expect(reason).To(Equal("Deployment is fully rolled out"))
		})

		It("should return true if observedGeneration is outdated", func() {
			deployment.Status.ObservedGeneration--

			progressing, reason := health.IsDeploymentProgressing(deployment)
			Expect(progressing).To(BeTrue())
			Expect(reason).To(Equal("observed generation outdated (41/42)"))
		})

		It("should return true if Progressing condition is missing", func() {
			deployment.Status.Conditions = []appsv1.DeploymentCondition{}

			progressing, reason := health.IsDeploymentProgressing(deployment)
			Expect(progressing).To(BeTrue())
			Expect(reason).To(Equal(`condition "Progressing" is missing`))
		})

		It("should return true if Progressing condition is not True", func() {
			deployment.Status.Conditions = []appsv1.DeploymentCondition{{
				Type:    appsv1.DeploymentProgressing,
				Status:  corev1.ConditionFalse,
				Reason:  "ProgressDeadlineExceeded",
				Message: `ReplicaSet "nginx-946d57896" has timed out progressing.`,
			}}

			progressing, reason := health.IsDeploymentProgressing(deployment)
			Expect(progressing).To(BeTrue())
			Expect(reason).To(Equal(deployment.Status.Conditions[0].Message))
		})

		It("should return true if Progressing condition does not have reason NewReplicaSetAvailable", func() {
			deployment.Status.Conditions = []appsv1.DeploymentCondition{{
				Type:    appsv1.DeploymentProgressing,
				Status:  corev1.ConditionFalse,
				Reason:  "ReplicaSetUpdated",
				Message: `ReplicaSet "nginx-85cfdf946f" is progressing.`,
			}}

			progressing, reason := health.IsDeploymentProgressing(deployment)
			Expect(progressing).To(BeTrue())
			Expect(reason).To(Equal(deployment.Status.Conditions[0].Message))
		})
	})

	Describe("#IsDeploymentUpdated", func() {
		var (
			ctx        = context.TODO()
			fakeClient client.Client
			deployment *appsv1.Deployment
			labels     = map[string]string{"foo": "bar"}
		)

		BeforeEach(func() {
			fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
			deployment = &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "deploy",
					Namespace: "namespace",
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: ptr.To[int32](1),
					Selector: &metav1.LabelSelector{MatchLabels: labels},
				},
			}
		})

		It("should consider the deployment as updated", func() {
			deployment.Generation = 24
			deployment.Spec.Replicas = ptr.To[int32](1)
			deployment.Status.Conditions = []appsv1.DeploymentCondition{
				{Type: appsv1.DeploymentProgressing, Status: "True", Reason: "NewReplicaSetAvailable"},
				{Type: appsv1.DeploymentAvailable, Status: "True"},
			}
			deployment.Status.ObservedGeneration = deployment.Generation
			deployment.Status.Replicas = *deployment.Spec.Replicas
			deployment.Status.UpdatedReplicas = *deployment.Spec.Replicas
			deployment.Status.AvailableReplicas = *deployment.Spec.Replicas

			Expect(fakeClient.Create(ctx, deployment)).To(Succeed())

			Expect(fakeClient.Create(ctx, &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod",
					Namespace: deployment.Namespace,
					Labels:    labels,
				},
			})).To(Succeed())

			ok, err := health.IsDeploymentUpdated(fakeClient, deployment)(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
		})

		It("should not consider the deployment as updated since there are still terminating pods", func() {
			deployment.Generation = 24
			deployment.Spec.Replicas = ptr.To[int32](1)
			deployment.Status.Conditions = []appsv1.DeploymentCondition{
				{Type: appsv1.DeploymentProgressing, Status: "True", Reason: "NewReplicaSetAvailable"},
				{Type: appsv1.DeploymentAvailable, Status: "True"},
			}
			deployment.Status.ObservedGeneration = deployment.Generation
			deployment.Status.Replicas = *deployment.Spec.Replicas
			deployment.Status.UpdatedReplicas = *deployment.Spec.Replicas
			deployment.Status.AvailableReplicas = *deployment.Spec.Replicas

			Expect(fakeClient.Create(ctx, deployment)).To(Succeed())

			for i := 0; i < 2; i++ {
				Expect(fakeClient.Create(ctx, &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      fmt.Sprintf("pod%d", i),
						Namespace: deployment.Namespace,
						Labels:    labels,
					},
				})).To(Succeed())
			}

			ok, err := health.IsDeploymentUpdated(fakeClient, deployment)(ctx)
			Expect(err).To(MatchError(ContainSubstring("there are still non-terminated old pods")))
			Expect(ok).To(BeFalse())
		})

		It("should not consider the deployment as updated since it is not healthy", func() {
			deployment.Generation = 24
			deployment.Spec.Replicas = ptr.To[int32](1)
			deployment.Status.Conditions = []appsv1.DeploymentCondition{
				{Type: appsv1.DeploymentProgressing, Status: "True", Reason: "NewReplicaSetAvailable"},
			}
			deployment.Status.ObservedGeneration = deployment.Generation
			deployment.Status.Replicas = *deployment.Spec.Replicas
			deployment.Status.UpdatedReplicas = *deployment.Spec.Replicas
			deployment.Status.AvailableReplicas = *deployment.Spec.Replicas

			Expect(fakeClient.Create(ctx, deployment)).To(Succeed())

			ok, err := health.IsDeploymentUpdated(fakeClient, deployment)(ctx)
			Expect(err).To(MatchError(ContainSubstring(`condition "Available" is missing`)))
			Expect(ok).To(BeFalse())
		})

		It("should not consider the deployment as updated since it is not progressing", func() {
			deployment.Generation = 24
			deployment.Spec.Replicas = ptr.To[int32](1)
			deployment.Status.Conditions = []appsv1.DeploymentCondition{
				{Type: appsv1.DeploymentProgressing, Status: "False", Message: "whatever message"},
			}
			deployment.Status.ObservedGeneration = deployment.Generation
			deployment.Status.Replicas = *deployment.Spec.Replicas
			deployment.Status.UpdatedReplicas = *deployment.Spec.Replicas
			deployment.Status.AvailableReplicas = *deployment.Spec.Replicas

			Expect(fakeClient.Create(ctx, deployment)).To(Succeed())

			ok, err := health.IsDeploymentUpdated(fakeClient, deployment)(ctx)
			Expect(err).To(MatchError(ContainSubstring("whatever message")))
			Expect(ok).To(BeFalse())
		})
	})

	Describe("#DeploymentHasExactNumberOfPods", func() {
		var (
			ctx        = context.TODO()
			fakeClient client.Client

			deployment *appsv1.Deployment
			pod        *corev1.Pod
		)

		BeforeEach(func() {
			fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()

			deployment = &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "deploy",
					Namespace: "namespace",
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: ptr.To[int32](1),
					Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"foo": "bar"}},
				},
			}
			pod = &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "pod-",
					Namespace:    deployment.Namespace,
					Labels:       deployment.Spec.Selector.MatchLabels,
				},
			}

			Expect(fakeClient.Create(ctx, deployment)).To(Succeed())
		})

		It("should consider the deployment as updated", func() {
			Expect(fakeClient.Create(ctx, pod)).To(Succeed())

			ok, err := health.DeploymentHasExactNumberOfPods(ctx, fakeClient, deployment)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
		})

		It("should not consider the deployment as updated since there are still terminating pods", func() {
			for i := 0; i < 2; i++ {
				p := pod.DeepCopy()
				Expect(fakeClient.Create(ctx, p)).To(Succeed())
			}

			ok, err := health.DeploymentHasExactNumberOfPods(ctx, fakeClient, deployment)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeFalse())
		})

		It("should consider the deployment as updated even though there are still stale pods", func() {
			p1 := pod.DeepCopy()
			p1.Status.Reason = "Evicted"
			Expect(fakeClient.Create(ctx, p1)).To(Succeed())

			p2 := pod.DeepCopy()
			Expect(fakeClient.Create(ctx, p2)).To(Succeed())

			ok, err := health.DeploymentHasExactNumberOfPods(ctx, fakeClient, deployment)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
		})

		It("should consider the deployment as updated even though there are still completed pods", func() {
			p1 := pod.DeepCopy()
			p1.Status.Conditions = []corev1.PodCondition{{Type: "Ready", Status: "False", Reason: "PodCompleted"}}
			Expect(fakeClient.Create(ctx, p1)).To(Succeed())

			p2 := pod.DeepCopy()
			Expect(fakeClient.Create(ctx, p2)).To(Succeed())

			ok, err := health.DeploymentHasExactNumberOfPods(ctx, fakeClient, deployment)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
		})
	})
})
