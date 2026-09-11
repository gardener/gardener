// SPDX-FileCopyrightText: Contributors to the Gardener project
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
					UID:       "deploy-uid",
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: new(int32(1)),
					Selector: &metav1.LabelSelector{MatchLabels: labels},
				},
			}
		})

		It("should consider the deployment as updated", func() {
			deployment.Generation = 24
			deployment.Spec.Replicas = new(int32(1))
			deployment.Status.Conditions = []appsv1.DeploymentCondition{
				{Type: appsv1.DeploymentProgressing, Status: "True", Reason: "NewReplicaSetAvailable"},
				{Type: appsv1.DeploymentAvailable, Status: "True"},
			}
			deployment.Status.ObservedGeneration = deployment.Generation
			deployment.Status.Replicas = *deployment.Spec.Replicas
			deployment.Status.UpdatedReplicas = *deployment.Spec.Replicas
			deployment.Status.AvailableReplicas = *deployment.Spec.Replicas

			Expect(fakeClient.Create(ctx, deployment)).To(Succeed())

			replicaSet := &appsv1.ReplicaSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "replicaset",
					Namespace:       deployment.Namespace,
					UID:             "replicaset-uid",
					Labels:          labels,
					OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(deployment, appsv1.SchemeGroupVersion.WithKind("Deployment"))},
				},
			}
			Expect(fakeClient.Create(ctx, replicaSet)).To(Succeed())

			Expect(fakeClient.Create(ctx, &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "pod",
					Namespace:       deployment.Namespace,
					Labels:          labels,
					OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(replicaSet, appsv1.SchemeGroupVersion.WithKind("ReplicaSet"))},
				},
			})).To(Succeed())

			ok, err := health.IsDeploymentUpdated(fakeClient, deployment)(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
		})

		It("should not consider the deployment as updated since there are still terminating pods", func() {
			deployment.Generation = 24
			deployment.Spec.Replicas = new(int32(1))
			deployment.Status.Conditions = []appsv1.DeploymentCondition{
				{Type: appsv1.DeploymentProgressing, Status: "True", Reason: "NewReplicaSetAvailable"},
				{Type: appsv1.DeploymentAvailable, Status: "True"},
			}
			deployment.Status.ObservedGeneration = deployment.Generation
			deployment.Status.Replicas = *deployment.Spec.Replicas
			deployment.Status.UpdatedReplicas = *deployment.Spec.Replicas
			deployment.Status.AvailableReplicas = *deployment.Spec.Replicas

			Expect(fakeClient.Create(ctx, deployment)).To(Succeed())

			replicaSet := &appsv1.ReplicaSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "replicaset",
					Namespace:       deployment.Namespace,
					UID:             "replicaset-uid",
					Labels:          labels,
					OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(deployment, appsv1.SchemeGroupVersion.WithKind("Deployment"))},
				},
			}
			Expect(fakeClient.Create(ctx, replicaSet)).To(Succeed())

			for i := range 2 {
				Expect(fakeClient.Create(ctx, &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:            fmt.Sprintf("pod%d", i),
						Namespace:       deployment.Namespace,
						Labels:          labels,
						OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(replicaSet, appsv1.SchemeGroupVersion.WithKind("ReplicaSet"))},
					},
				})).To(Succeed())
			}

			ok, err := health.IsDeploymentUpdated(fakeClient, deployment)(ctx)
			Expect(err).To(MatchError(ContainSubstring("there are still non-terminated old pods")))
			Expect(ok).To(BeFalse())
		})

		It("should not consider the deployment as updated since it is not healthy", func() {
			deployment.Generation = 24
			deployment.Spec.Replicas = new(int32(1))
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
			deployment.Spec.Replicas = new(int32(1))
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
			replicaSet *appsv1.ReplicaSet
			pod        *corev1.Pod
		)

		BeforeEach(func() {
			fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()

			deployment = &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "deploy",
					Namespace: "namespace",
					UID:       "deploy-uid",
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: new(int32(1)),
					Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"foo": "bar"}},
				},
			}
			Expect(fakeClient.Create(ctx, deployment)).To(Succeed())

			replicaSet = &appsv1.ReplicaSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "replicaset",
					Namespace:       deployment.Namespace,
					UID:             "replicaset-uid",
					Labels:          deployment.Spec.Selector.MatchLabels,
					OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(deployment, appsv1.SchemeGroupVersion.WithKind("Deployment"))},
				},
			}
			Expect(fakeClient.Create(ctx, replicaSet)).To(Succeed())

			pod = &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName:    "pod-",
					Namespace:       deployment.Namespace,
					Labels:          deployment.Spec.Selector.MatchLabels,
					OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(replicaSet, appsv1.SchemeGroupVersion.WithKind("ReplicaSet"))},
				},
			}
		})

		It("should consider the deployment as updated", func() {
			Expect(fakeClient.Create(ctx, pod)).To(Succeed())

			ok, err := health.DeploymentHasExactNumberOfPods(ctx, fakeClient, deployment)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
		})

		It("should not consider the deployment as updated since there are still terminating pods", func() {
			for range 2 {
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

		It("should consider the deployment as updated even though there are still pods in a terminal phase", func() {
			p1 := pod.DeepCopy()
			p1.Status.Phase = corev1.PodFailed
			Expect(fakeClient.Create(ctx, p1)).To(Succeed())

			p2 := pod.DeepCopy()
			p2.Status.Phase = corev1.PodSucceeded
			Expect(fakeClient.Create(ctx, p2)).To(Succeed())

			p3 := pod.DeepCopy()
			Expect(fakeClient.Create(ctx, p3)).To(Succeed())

			ok, err := health.DeploymentHasExactNumberOfPods(ctx, fakeClient, deployment)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
		})

		It("should consider the deployment as updated even though there are still disrupted pods", func() {
			p1 := pod.DeepCopy()
			p1.Status.Conditions = []corev1.PodCondition{{Type: "DisruptionTarget", Status: "True", Reason: "TerminationByKubelet"}}
			Expect(fakeClient.Create(ctx, p1)).To(Succeed())

			p2 := pod.DeepCopy()
			Expect(fakeClient.Create(ctx, p2)).To(Succeed())

			ok, err := health.DeploymentHasExactNumberOfPods(ctx, fakeClient, deployment)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
		})

		It("should not consider pods of an unrelated Deployment sharing the same selector labels", func() {
			Expect(fakeClient.Create(ctx, pod)).To(Succeed())

			otherDeployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "other-deploy",
					Namespace: deployment.Namespace,
					UID:       "other-deploy-uid",
				},
				Spec: appsv1.DeploymentSpec{
					Selector: &metav1.LabelSelector{MatchLabels: deployment.Spec.Selector.MatchLabels},
				},
			}
			Expect(fakeClient.Create(ctx, otherDeployment)).To(Succeed())

			otherReplicaSet := &appsv1.ReplicaSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "other-replicaset",
					Namespace:       deployment.Namespace,
					UID:             "other-replicaset-uid",
					Labels:          deployment.Spec.Selector.MatchLabels,
					OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(otherDeployment, appsv1.SchemeGroupVersion.WithKind("Deployment"))},
				},
			}
			Expect(fakeClient.Create(ctx, otherReplicaSet)).To(Succeed())

			otherPod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName:    "other-pod-",
					Namespace:       deployment.Namespace,
					Labels:          deployment.Spec.Selector.MatchLabels,
					OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(otherReplicaSet, appsv1.SchemeGroupVersion.WithKind("ReplicaSet"))},
				},
			}
			Expect(fakeClient.Create(ctx, otherPod)).To(Succeed())

			ok, err := health.DeploymentHasExactNumberOfPods(ctx, fakeClient, deployment)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
		})
	})
})
