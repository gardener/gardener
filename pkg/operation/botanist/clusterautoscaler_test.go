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

package botanist_test

import (
	"context"
	"fmt"

	"github.com/Masterminds/semver/v3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	kubernetesmock "github.com/gardener/gardener/pkg/client/kubernetes/mock"
	mockclusterautoscaler "github.com/gardener/gardener/pkg/component/clusterautoscaler/mock"
	mockworker "github.com/gardener/gardener/pkg/component/extensions/worker/mock"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	"github.com/gardener/gardener/pkg/operation"
	. "github.com/gardener/gardener/pkg/operation/botanist"
	seedpkg "github.com/gardener/gardener/pkg/operation/seed"
	shootpkg "github.com/gardener/gardener/pkg/operation/shoot"
)

var _ = Describe("ClusterAutoscaler", func() {
	var (
		ctx     = context.TODO()
		fakeErr = fmt.Errorf("fake err")

		ctrl             *gomock.Controller
		botanist         *Botanist
		kubernetesClient *kubernetesmock.MockInterface
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		kubernetesClient = kubernetesmock.NewMockInterface(ctrl)
		botanist = &Botanist{Operation: &operation.Operation{}}
		botanist.Seed = &seedpkg.Seed{
			KubernetesVersion: semver.MustParse("1.25.0"),
		}
		botanist.Shoot = &shootpkg.Shoot{}
		botanist.SeedClientSet = kubernetesClient
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#DefaultClusterAutoscaler", func() {
		BeforeEach(func() {
			kubernetesClient.EXPECT().Client()
			kubernetesClient.EXPECT().Version().Return("1.25.0")
			botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{Spec: gardencorev1beta1.ShootSpec{Kubernetes: gardencorev1beta1.Kubernetes{Version: "1.25.0"}}})
		})

		It("should successfully create a cluster-autoscaler interface", func() {
			clusterAutoscaler, err := botanist.DefaultClusterAutoscaler()
			Expect(err).NotTo(HaveOccurred())
			Expect(clusterAutoscaler).NotTo(BeNil())
		})
	})

	Describe("#DeployClusterAutoscaler", func() {
		var (
			clusterAutoscaler *mockclusterautoscaler.MockInterface
			worker            *mockworker.MockInterface

			namespaceUID       = types.UID("5678")
			machineDeployments = []extensionsv1alpha1.MachineDeployment{{}}
		)

		BeforeEach(func() {
			clusterAutoscaler = mockclusterautoscaler.NewMockInterface(ctrl)
			worker = mockworker.NewMockInterface(ctrl)

			botanist.SeedNamespaceObject = &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					UID: namespaceUID,
				},
			}
			botanist.Shoot = &shootpkg.Shoot{
				Components: &shootpkg.Components{
					ControlPlane: &shootpkg.ControlPlane{
						ClusterAutoscaler: clusterAutoscaler,
					},
					Extensions: &shootpkg.Extensions{
						Worker: worker,
					},
				},
			}
		})

		Context("CA wanted", func() {
			BeforeEach(func() {
				botanist.Shoot.WantsClusterAutoscaler = true

				clusterAutoscaler.EXPECT().SetNamespaceUID(namespaceUID)
				worker.EXPECT().MachineDeployments().Return(machineDeployments)
				clusterAutoscaler.EXPECT().SetMachineDeployments(machineDeployments)
			})

			It("should set the secrets, namespace uid, machine deployments, and deploy", func() {
				clusterAutoscaler.EXPECT().Deploy(ctx)
				Expect(botanist.DeployClusterAutoscaler(ctx)).To(Succeed())
			})

			It("should fail when the deploy function fails", func() {
				clusterAutoscaler.EXPECT().Deploy(ctx).Return(fakeErr)
				Expect(botanist.DeployClusterAutoscaler(ctx)).To(Equal(fakeErr))
			})
		})

		Context("CA unwanted", func() {
			BeforeEach(func() {
				botanist.Shoot.WantsClusterAutoscaler = false
			})

			It("should destroy", func() {
				clusterAutoscaler.EXPECT().Destroy(ctx)
				Expect(botanist.DeployClusterAutoscaler(ctx)).To(Succeed())
			})

			It("should fail when the destroy function fails", func() {
				clusterAutoscaler.EXPECT().Destroy(ctx).Return(fakeErr)
				Expect(botanist.DeployClusterAutoscaler(ctx)).To(Equal(fakeErr))
			})
		})
	})

	Describe("#ScaleClusterAutoscalerToZero", func() {
		var (
			c         *mockclient.MockClient
			sw        *mockclient.MockSubResourceClient
			patch     = client.RawPatch(types.MergePatchType, []byte(`{"spec":{"replicas":0}}`))
			namespace = "shoot--foo--bar"
		)

		BeforeEach(func() {
			botanist.SeedClientSet = kubernetesClient
			botanist.Shoot = &shootpkg.Shoot{
				SeedNamespace: namespace,
			}

			c = mockclient.NewMockClient(ctrl)
			kubernetesClient.EXPECT().Client().Return(c)

			sw = mockclient.NewMockSubResourceClient(ctrl)
			c.EXPECT().SubResource("scale").Return(sw)
		})

		It("should scale the CA deployment", func() {
			sw.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&appsv1.Deployment{}), patch)
			Expect(botanist.ScaleClusterAutoscalerToZero(ctx)).To(Succeed())
		})

		It("should fail when the scale call fails", func() {
			sw.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&appsv1.Deployment{}), patch).Return(fakeErr)
			Expect(botanist.ScaleClusterAutoscalerToZero(ctx)).To(MatchError(fakeErr))
		})
	})
})
