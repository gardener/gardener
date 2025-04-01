// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package kubernetes_test

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/retry"
	mockclient "github.com/gardener/gardener/third_party/mock/controller-runtime/client"
)

var _ = Describe("Deployments", func() {
	var (
		ctrl      *gomock.Controller
		c         *mockclient.MockClient
		namespace = "test"
		name      = "dummy-app"
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		c = mockclient.NewMockClient(ctrl)
	})

	AfterEach(func() {
		ctrl.Finish()
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

			c.EXPECT().
				Get(
					gomock.Any(),
					gomock.AssignableToTypeOf(client.ObjectKey{}),
					gomock.AssignableToTypeOf(&appsv1.Deployment{}),
				).
				DoAndReturn(func(
					_ context.Context,
					_ client.ObjectKey,
					deployment *appsv1.Deployment,
					_ ...client.GetOption,
				) error {
					var (
						replicas   int32 = 5
						generation int64 = 10
					)

					deployment.Generation = generation
					deployment.Spec.Replicas = &replicas
					deployment.Status = appsv1.DeploymentStatus{
						ObservedGeneration: generation,
						Replicas:           replicas,
						UpdatedReplicas:    replicas,
						AvailableReplicas:  replicas,
					}

					return nil
				})

			_, actualError := kubernetes.HasDeploymentRolloutCompleted(context.TODO(), c, namespace, name)
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

			c.EXPECT().
				Get(
					gomock.Any(),
					gomock.AssignableToTypeOf(client.ObjectKey{}),
					gomock.AssignableToTypeOf(&appsv1.Deployment{}),
				).
				DoAndReturn(func(
					_ context.Context,
					_ client.ObjectKey,
					deployment *appsv1.Deployment,
					_ ...client.GetOption,
				) error {
					var ()

					deployment.Generation = generation
					deployment.Spec.Replicas = &replicas
					deployment.Status = appsv1.DeploymentStatus{
						ObservedGeneration: observedGeneration,
						Replicas:           replicas - 1,
						UpdatedReplicas:    replicas - 1,
						AvailableReplicas:  replicas - 1,
					}

					return nil
				})

			_, actualError := kubernetes.HasDeploymentRolloutCompleted(context.TODO(), c, namespace, name)
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

			c.EXPECT().
				Get(
					gomock.Any(),
					gomock.AssignableToTypeOf(client.ObjectKey{}),
					gomock.AssignableToTypeOf(&appsv1.Deployment{}),
				).
				DoAndReturn(func(
					_ context.Context,
					_ client.ObjectKey,
					deployment *appsv1.Deployment,
					_ ...client.GetOption,
				) error {
					var ()

					deployment.Generation = generation
					deployment.Spec.Replicas = &replicas
					deployment.Status = appsv1.DeploymentStatus{
						ObservedGeneration: generation,
						Replicas:           replicas - 1,
						UpdatedReplicas:    updatedReplicas,
						AvailableReplicas:  availableReplicas,
					}

					return nil
				})

			_, actualError := kubernetes.HasDeploymentRolloutCompleted(context.TODO(), c, namespace, name)
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

			c.EXPECT().
				Get(
					gomock.Any(),
					gomock.AssignableToTypeOf(client.ObjectKey{}),
					gomock.AssignableToTypeOf(&appsv1.Deployment{}),
				).
				DoAndReturn(func(
					_ context.Context,
					_ client.ObjectKey,
					deployment *appsv1.Deployment,
					_ ...client.GetOption,
				) error {
					var ()

					deployment.Generation = generation
					deployment.Spec.Replicas = &replicas
					deployment.Status = appsv1.DeploymentStatus{
						ObservedGeneration: generation,
						Replicas:           replicas - 1,
						UpdatedReplicas:    updatedReplicas,
						AvailableReplicas:  availableReplicas,
					}

					return nil
				})

			_, actualError := kubernetes.HasDeploymentRolloutCompleted(context.TODO(), c, namespace, name)
			Expect(actualError).To(Equal(expectedError))
		})
	})
})
