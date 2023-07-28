// Copyright 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package kubernetes_test

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	appsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	"github.com/gardener/gardener/pkg/utils/retry"
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
					key client.ObjectKey,
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
					key client.ObjectKey,
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
				updatedReplicas   int32 = replicas - 1
				availableReplicas int32 = updatedReplicas
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
					key client.ObjectKey,
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
				updatedReplicas   int32 = replicas
				availableReplicas int32 = replicas - 1
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
					key client.ObjectKey,
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
