// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package kubernetes_test

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/retry"
)

var _ = Describe("Deployments", func() {
	var (
		ctx        = context.TODO()
		fakeClient client.Client
		namespace  = "test"
		name       = "dummy-app"
	)

	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetesscheme.Scheme).Build()
	})

	DescribeTable("#ValidDeploymentContainerImageVersion",
		func(containerName, minVersion string, expected bool) {
			fakeImage := "test:0.3.0"
			deployment := appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "lb-deployment",
									Image: fakeImage,
								},
							},
						},
					},
				},
			}
			ok, _ := kubernetes.ValidDeploymentContainerImageVersion(&deployment, containerName, minVersion)
			Expect(ok).To(Equal(expected))
		},
		Entry("invalid version", "lb-deployment", `0.4.0`, false),
		Entry("invalid container name", "deployment", "0.3.0", false),
	)

	Describe("#HasDeploymentRolloutCompleted", func() {
		It("Rollout is complete", func() {
			var (
				replicas   int32 = 5
				generation int64 = 10
			)

			deployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:       name,
					Namespace:  namespace,
					Generation: generation,
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: &replicas,
				},
				Status: appsv1.DeploymentStatus{
					ObservedGeneration: generation,
					Replicas:           replicas,
					UpdatedReplicas:    replicas,
					AvailableReplicas:  replicas,
				},
			}
			Expect(fakeClient.Create(ctx, deployment)).To(Succeed())

			_, actualError := kubernetes.HasDeploymentRolloutCompleted(ctx, fakeClient, namespace, name)
			Expect(actualError).NotTo(HaveOccurred())
		})

		It("Updated deployment hasn't been picked up yet", func() {
			var (
				replicas           int32 = 5
				generation         int64 = 10
				observedGeneration int64 = 11
			)

			_, expectedError := retry.MinorError(fmt.Errorf("%q not observed at latest generation (%d/%d)",
				name, observedGeneration, generation))

			deployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:       name,
					Namespace:  namespace,
					Generation: generation,
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: &replicas,
				},
				Status: appsv1.DeploymentStatus{
					ObservedGeneration: observedGeneration,
					Replicas:           replicas - 1,
					UpdatedReplicas:    replicas - 1,
					AvailableReplicas:  replicas - 1,
				},
			}
			Expect(fakeClient.Create(ctx, deployment)).To(Succeed())

			_, actualError := kubernetes.HasDeploymentRolloutCompleted(ctx, fakeClient, namespace, name)
			Expect(actualError).To(Equal(expectedError))
		})

		It("UpdatedReplicas isn't matching with desired", func() {
			var (
				replicas          int32 = 5
				updatedReplicas         = replicas - 1
				availableReplicas       = updatedReplicas
				generation        int64 = 10
			)

			_, expectedError := retry.MinorError(fmt.Errorf("deployment %q currently has Updated/Available: %d/%d replicas. Desired: %d",
				name, updatedReplicas, availableReplicas, replicas))

			deployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:       name,
					Namespace:  namespace,
					Generation: generation,
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: &replicas,
				},
				Status: appsv1.DeploymentStatus{
					ObservedGeneration: generation,
					Replicas:           replicas - 1,
					UpdatedReplicas:    updatedReplicas,
					AvailableReplicas:  availableReplicas,
				},
			}
			Expect(fakeClient.Create(ctx, deployment)).To(Succeed())

			_, actualError := kubernetes.HasDeploymentRolloutCompleted(ctx, fakeClient, namespace, name)
			Expect(actualError).To(Equal(expectedError))
		})

		It("AvailableReplicas isn't matching with desired", func() {
			var (
				replicas          int32 = 5
				updatedReplicas         = replicas
				availableReplicas       = replicas - 1
				generation        int64 = 10
			)

			_, expectedError := retry.MinorError(fmt.Errorf("deployment %q currently has Updated/Available: %d/%d replicas. Desired: %d",
				name, updatedReplicas, availableReplicas, replicas))

			deployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:       name,
					Namespace:  namespace,
					Generation: generation,
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: &replicas,
				},
				Status: appsv1.DeploymentStatus{
					ObservedGeneration: generation,
					Replicas:           replicas - 1,
					UpdatedReplicas:    updatedReplicas,
					AvailableReplicas:  availableReplicas,
				},
			}
			Expect(fakeClient.Create(ctx, deployment)).To(Succeed())

			_, actualError := kubernetes.HasDeploymentRolloutCompleted(ctx, fakeClient, namespace, name)
			Expect(actualError).To(Equal(expectedError))
		})
	})
})
