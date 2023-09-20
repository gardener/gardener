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
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	kubernetesmock "github.com/gardener/gardener/pkg/client/kubernetes/mock"
	mockkubeapiserver "github.com/gardener/gardener/pkg/component/kubeapiserver/mock"
	mockkubecontrollermanager "github.com/gardener/gardener/pkg/component/kubecontrollermanager/mock"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	"github.com/gardener/gardener/pkg/operation"
	. "github.com/gardener/gardener/pkg/operation/botanist"
	seedpkg "github.com/gardener/gardener/pkg/operation/seed"
	shootpkg "github.com/gardener/gardener/pkg/operation/shoot"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

var _ = Describe("KubeControllerManager", func() {
	var (
		ctrl             *gomock.Controller
		botanist         *Botanist
		kubernetesClient *kubernetesmock.MockInterface
		c                *mockclient.MockClient

		ctx       = context.TODO()
		fakeErr   = fmt.Errorf("fake err")
		namespace = "foo"
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		botanist = &Botanist{Operation: &operation.Operation{}}
		kubernetesClient = kubernetesmock.NewMockInterface(ctrl)
		c = mockclient.NewMockClient(ctrl)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#DefaultKubeControllerManager", func() {
		BeforeEach(func() {
			botanist.Logger = logr.Discard()
			botanist.SeedClientSet = kubernetesClient
			botanist.Seed = &seedpkg.Seed{
				KubernetesVersion: semver.MustParse("1.25.0"),
			}
			botanist.Shoot = &shootpkg.Shoot{
				KubernetesVersion: semver.MustParse("1.25.0"),
				Networks:          &shootpkg.Networks{},
			}
			botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{})
		})

		It("should successfully create a kube-controller-manager interface", func() {
			kubeControllerManager, err := botanist.DefaultKubeControllerManager()
			Expect(kubeControllerManager).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("#DeployKubeControllerManager", func() {
		var (
			kubeAPIServer         *mockkubeapiserver.MockInterface
			kubeControllerManager *mockkubecontrollermanager.MockInterface
		)

		BeforeEach(func() {
			kubeAPIServer = mockkubeapiserver.NewMockInterface(ctrl)
			kubeControllerManager = mockkubecontrollermanager.NewMockInterface(ctrl)

			botanist.SeedClientSet = kubernetesClient
			botanist.Shoot = &shootpkg.Shoot{
				Components: &shootpkg.Components{
					ControlPlane: &shootpkg.ControlPlane{
						KubeAPIServer:         kubeAPIServer,
						KubeControllerManager: kubeControllerManager,
					},
				},
				SeedNamespace: namespace,
			}
			botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{})
		})

		Context("successfully deployment", func() {
			BeforeEach(func() {
				kubeControllerManager.EXPECT().Deploy(ctx)
			})

			Context("kube-apiserver is already scaled down", func() {
				BeforeEach(func() {
					kubeAPIServer.EXPECT().GetAutoscalingReplicas().Return(pointer.Int32(0))
					botanist.Shoot.GetInfo().Status.LastOperation = nil
				})

				It("hibernation status unequal (true/false) and Kube-Apiserver is already scaled down", func() {
					botanist.Shoot.HibernationEnabled = true
					botanist.Shoot.GetInfo().Status.IsHibernated = false

					kubeControllerManager.EXPECT().SetReplicaCount(int32(0))

					Expect(botanist.DeployKubeControllerManager(ctx)).To(Succeed())
				})
			})

			Context("last operation is nil or neither of type 'create' nor 'restore'", func() {
				BeforeEach(func() {
					kubeAPIServer.EXPECT().GetAutoscalingReplicas().Return(pointer.Int32(1)).AnyTimes()
					botanist.Shoot.GetInfo().Status.LastOperation = nil
				})

				It("hibernation status unequal (true/false)", func() {
					botanist.Shoot.HibernationEnabled = true
					botanist.Shoot.GetInfo().Status.IsHibernated = false

					kubeControllerManager.EXPECT().SetReplicaCount(int32(1))

					Expect(botanist.DeployKubeControllerManager(ctx)).To(Succeed())
				})

				It("hibernation status unequal (false/true)", func() {
					botanist.Shoot.HibernationEnabled = false
					botanist.Shoot.GetInfo().Status.IsHibernated = true

					kubeControllerManager.EXPECT().SetReplicaCount(int32(1))

					Expect(botanist.DeployKubeControllerManager(ctx)).To(Succeed())
				})

				It("hibernation status equal (true/true)", func() {
					botanist.Shoot.HibernationEnabled = true
					botanist.Shoot.GetInfo().Status.IsHibernated = true

					var replicas int32 = 4
					kubernetesClient.EXPECT().Client().Return(c)
					c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, "kube-controller-manager"), gomock.AssignableToTypeOf(&appsv1.Deployment{})).DoAndReturn(func(_ context.Context, _ types.NamespacedName, obj *appsv1.Deployment, _ ...client.GetOption) error {
						obj.Spec.Replicas = pointer.Int32(replicas)
						return nil
					})
					kubeControllerManager.EXPECT().SetReplicaCount(replicas)

					Expect(botanist.DeployKubeControllerManager(ctx)).To(Succeed())
				})

				It("hibernation status equal (false/false)", func() {
					botanist.Shoot.HibernationEnabled = false
					botanist.Shoot.GetInfo().Status.IsHibernated = false

					var replicas int32 = 4
					kubernetesClient.EXPECT().Client().Return(c)
					c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, "kube-controller-manager"), gomock.AssignableToTypeOf(&appsv1.Deployment{})).DoAndReturn(func(_ context.Context, _ types.NamespacedName, obj *appsv1.Deployment, _ ...client.GetOption) error {
						obj.Spec.Replicas = pointer.Int32(replicas)
						return nil
					})
					kubeControllerManager.EXPECT().SetReplicaCount(replicas)

					Expect(botanist.DeployKubeControllerManager(ctx)).To(Succeed())
				})
			})

			Context("last operation is not nil and of type 'create'", func() {
				BeforeEach(func() {
					botanist.Shoot.GetInfo().Status.LastOperation = &gardencorev1beta1.LastOperation{Type: gardencorev1beta1.LastOperationTypeCreate}
				})

				It("hibernation enabled", func() {
					botanist.Shoot.HibernationEnabled = true
					kubernetesClient.EXPECT().Client().Return(c)
					c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, "kube-controller-manager"), gomock.AssignableToTypeOf(&appsv1.Deployment{})).DoAndReturn(func(_ context.Context, _ types.NamespacedName, obj *appsv1.Deployment, _ ...client.GetOption) error {
						obj.Spec.Replicas = pointer.Int32(0)
						return nil
					})
					kubeControllerManager.EXPECT().SetReplicaCount(int32(0))

					Expect(botanist.DeployKubeControllerManager(ctx)).To(Succeed())
				})

				It("hibernation enabled and kube-controller-manager deployment does not exist", func() {
					botanist.Shoot.HibernationEnabled = true
					kubernetesClient.EXPECT().Client().Return(c)
					c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, "kube-controller-manager"), gomock.AssignableToTypeOf(&appsv1.Deployment{})).Return(apierrors.NewNotFound(appsv1.Resource("Deployment"), "kube-controller-manager"))
					kubeControllerManager.EXPECT().SetReplicaCount(int32(0))

					Expect(botanist.DeployKubeControllerManager(ctx)).To(Succeed())
				})

				It("hibernation enabled and kube-controller-manager is already scaled up", func() {
					botanist.Shoot.HibernationEnabled = true
					kubernetesClient.EXPECT().Client().Return(c)
					c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, "kube-controller-manager"), gomock.AssignableToTypeOf(&appsv1.Deployment{})).DoAndReturn(func(_ context.Context, _ types.NamespacedName, obj *appsv1.Deployment, _ ...client.GetOption) error {
						obj.Spec.Replicas = pointer.Int32(1)
						return nil
					})
					kubeControllerManager.EXPECT().SetReplicaCount(int32(1))

					Expect(botanist.DeployKubeControllerManager(ctx)).To(Succeed())
				})

				It("hibernation disabled", func() {
					kubeAPIServer.EXPECT().GetAutoscalingReplicas().Return(pointer.Int32(1))
					botanist.Shoot.HibernationEnabled = false

					kubeControllerManager.EXPECT().SetReplicaCount(int32(1))

					Expect(botanist.DeployKubeControllerManager(ctx)).To(Succeed())
				})
			})

			Context("last operation is not nil and of type 'restore'", func() {
				BeforeEach(func() {
					botanist.Shoot.GetInfo().Status.LastOperation = &gardencorev1beta1.LastOperation{Type: gardencorev1beta1.LastOperationTypeRestore}
				})

				It("hibernation enabled", func() {
					botanist.Shoot.HibernationEnabled = true
					kubernetesClient.EXPECT().Client().Return(c)
					c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, "kube-controller-manager"), gomock.AssignableToTypeOf(&appsv1.Deployment{})).DoAndReturn(func(_ context.Context, _ types.NamespacedName, obj *appsv1.Deployment, _ ...client.GetOption) error {
						obj.Spec.Replicas = pointer.Int32(0)
						return nil
					})
					kubeControllerManager.EXPECT().SetReplicaCount(int32(0))

					Expect(botanist.DeployKubeControllerManager(ctx)).To(Succeed())
				})

				It("hibernation enabled and kube-controller-manager deployment does not exist", func() {
					botanist.Shoot.HibernationEnabled = true
					kubernetesClient.EXPECT().Client().Return(c)
					c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, "kube-controller-manager"), gomock.AssignableToTypeOf(&appsv1.Deployment{})).Return(apierrors.NewNotFound(appsv1.Resource("Deployment"), "kube-controller-manager"))
					kubeControllerManager.EXPECT().SetReplicaCount(int32(0))

					Expect(botanist.DeployKubeControllerManager(ctx)).To(Succeed())
				})

				It("hibernation enabled and kube-controller-manager is already scaled up", func() {
					botanist.Shoot.HibernationEnabled = true
					kubernetesClient.EXPECT().Client().Return(c)
					c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, "kube-controller-manager"), gomock.AssignableToTypeOf(&appsv1.Deployment{})).DoAndReturn(func(_ context.Context, _ types.NamespacedName, obj *appsv1.Deployment, _ ...client.GetOption) error {
						obj.Spec.Replicas = pointer.Int32(1)
						return nil
					})
					kubeControllerManager.EXPECT().SetReplicaCount(int32(1))

					Expect(botanist.DeployKubeControllerManager(ctx)).To(Succeed())
				})

				It("hibernation disabled", func() {
					kubeAPIServer.EXPECT().GetAutoscalingReplicas().Return(pointer.Int32(1))
					botanist.Shoot.HibernationEnabled = false

					kubeControllerManager.EXPECT().SetReplicaCount(int32(1))

					Expect(botanist.DeployKubeControllerManager(ctx)).To(Succeed())
				})
			})
		})

		It("should fail when the replicas cannot be determined", func() {
			kubernetesClient.EXPECT().Client().Return(c)
			c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, "kube-controller-manager"), gomock.AssignableToTypeOf(&appsv1.Deployment{})).Return(fakeErr)

			Expect(botanist.DeployKubeControllerManager(ctx)).To(Equal(fakeErr))
		})

		It("should fail when the deploy function fails", func() {
			kubernetesClient.EXPECT().Client().Return(c)
			c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, "kube-controller-manager"), gomock.AssignableToTypeOf(&appsv1.Deployment{}))
			kubeControllerManager.EXPECT().SetReplicaCount(int32(0))
			kubeControllerManager.EXPECT().Deploy(ctx).Return(fakeErr)

			Expect(botanist.DeployKubeControllerManager(ctx)).To(Equal(fakeErr))
		})
	})

	Describe("#ScaleKubeControllerManagerToOne", func() {
		var (
			sw    *mockclient.MockSubResourceClient
			patch = client.RawPatch(types.MergePatchType, []byte(`{"spec":{"replicas":1}}`))
		)

		BeforeEach(func() {
			botanist.SeedClientSet = kubernetesClient
			botanist.Shoot = &shootpkg.Shoot{
				SeedNamespace: namespace,
			}

			kubernetesClient.EXPECT().Client().Return(c)

			sw = mockclient.NewMockSubResourceClient(ctrl)
			c.EXPECT().SubResource("scale").Return(sw)
		})

		It("should scale the KCM deployment", func() {
			sw.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&appsv1.Deployment{}), patch)
			Expect(botanist.ScaleKubeControllerManagerToOne(ctx)).To(Succeed())
		})

		It("should fail when the scale call fails", func() {
			sw.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&appsv1.Deployment{}), patch).Return(fakeErr)
			Expect(botanist.ScaleKubeControllerManagerToOne(ctx)).To(MatchError(fakeErr))
		})
	})
})
