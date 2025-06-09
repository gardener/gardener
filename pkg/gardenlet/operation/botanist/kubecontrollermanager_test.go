// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist_test

import (
	"context"
	"errors"
	"fmt"
	"net"

	"github.com/Masterminds/semver/v3"
	dwdapi "github.com/gardener/dependency-watchdog/api/prober"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	kubernetesmock "github.com/gardener/gardener/pkg/client/kubernetes/mock"
	kubeapiserver "github.com/gardener/gardener/pkg/component/kubernetes/apiserver"
	mockkubeapiserver "github.com/gardener/gardener/pkg/component/kubernetes/apiserver/mock"
	mockkubecontrollermanager "github.com/gardener/gardener/pkg/component/kubernetes/controllermanager/mock"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	. "github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
	seedpkg "github.com/gardener/gardener/pkg/gardenlet/operation/seed"
	shootpkg "github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
	mockclient "github.com/gardener/gardener/third_party/mock/controller-runtime/client"
)

var _ = Describe("KubeControllerManager", func() {
	var (
		ctrl             *gomock.Controller
		botanist         *Botanist
		kubernetesClient *kubernetesmock.MockInterface
		c                *mockclient.MockClient

		ctx       = context.TODO()
		fakeErr   = errors.New("fake err")
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
				KubernetesVersion: semver.MustParse("1.31.0"),
			}
			botanist.Shoot = &shootpkg.Shoot{
				KubernetesVersion: semver.MustParse("1.31.0"),
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
				ControlPlaneNamespace: namespace,
				Networks: &shootpkg.Networks{
					Pods:     []net.IPNet{{IP: net.ParseIP("10.0.0.0"), Mask: net.CIDRMask(24, 32)}},
					Services: []net.IPNet{{IP: net.ParseIP("10.0.1.0"), Mask: net.CIDRMask(24, 32)}},
				},
			}
			botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{})
		})

		Context("successfully deployment", func() {
			BeforeEach(func() {
				kubeControllerManager.EXPECT().Deploy(ctx)
				kubeAPIServer.EXPECT().GetValues().Return(kubeapiserver.Values{RuntimeConfig: map[string]bool{"foo": true}})
				kubeControllerManager.EXPECT().SetRuntimeConfig(map[string]bool{"foo": true})
				kubeControllerManager.EXPECT().SetServiceNetworks(botanist.Shoot.Networks.Services)
				kubeControllerManager.EXPECT().SetPodNetworks(botanist.Shoot.Networks.Pods)
			})

			Context("kube-apiserver is already scaled down", func() {
				BeforeEach(func() {
					kubeAPIServer.EXPECT().GetAutoscalingReplicas().Return(ptr.To[int32](0))
					botanist.Shoot.GetInfo().Status.LastOperation = nil
				})

				It("hibernation status unequal (true/false) and Kube-Apiserver is already scaled down", func() {
					botanist.Shoot.HibernationEnabled = true
					botanist.Shoot.GetInfo().Status.IsHibernated = false

					kubeControllerManager.EXPECT().SetReplicaCount(int32(0))
					kubernetesClient.EXPECT().Client().Return(c)
					c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: "kube-controller-manager"}, gomock.AssignableToTypeOf(&appsv1.Deployment{}))

					Expect(botanist.DeployKubeControllerManager(ctx)).To(Succeed())
				})
			})

			Context("last operation is nil or neither of type 'create' nor 'restore'", func() {
				BeforeEach(func() {
					kubeAPIServer.EXPECT().GetAutoscalingReplicas().Return(ptr.To[int32](1)).AnyTimes()
					botanist.Shoot.GetInfo().Status.LastOperation = nil
				})

				It("hibernation status unequal (true/false)", func() {
					botanist.Shoot.HibernationEnabled = true
					botanist.Shoot.GetInfo().Status.IsHibernated = false

					kubeControllerManager.EXPECT().SetReplicaCount(int32(1))
					kubernetesClient.EXPECT().Client().Return(c)
					c.EXPECT().Get(gomock.Any(), client.ObjectKey{Namespace: namespace, Name: "kube-controller-manager"}, gomock.AssignableToTypeOf(&appsv1.Deployment{})).AnyTimes()

					Expect(botanist.DeployKubeControllerManager(ctx)).To(Succeed())
				})

				It("hibernation status unequal (false/true)", func() {
					botanist.Shoot.HibernationEnabled = false
					botanist.Shoot.GetInfo().Status.IsHibernated = true

					kubeControllerManager.EXPECT().SetReplicaCount(int32(1))
					kubernetesClient.EXPECT().Client().Return(c)
					c.EXPECT().Get(gomock.Any(), client.ObjectKey{Namespace: namespace, Name: "kube-controller-manager"}, gomock.AssignableToTypeOf(&appsv1.Deployment{})).AnyTimes()

					Expect(botanist.DeployKubeControllerManager(ctx)).To(Succeed())
				})

				It("hibernation status equal (true/true)", func() {
					botanist.Shoot.HibernationEnabled = true
					botanist.Shoot.GetInfo().Status.IsHibernated = true

					var replicas int32 = 4
					kubernetesClient.EXPECT().Client().Return(c)
					c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: "kube-controller-manager"}, gomock.AssignableToTypeOf(&appsv1.Deployment{})).DoAndReturn(func(_ context.Context, _ types.NamespacedName, obj *appsv1.Deployment, _ ...client.GetOption) error {
						obj.Spec.Replicas = ptr.To(replicas)
						return nil
					}).AnyTimes()

					kubeControllerManager.EXPECT().SetReplicaCount(replicas)

					Expect(botanist.DeployKubeControllerManager(ctx)).To(Succeed())
				})

				It("hibernation status equal (false/false)", func() {
					botanist.Shoot.HibernationEnabled = false
					botanist.Shoot.GetInfo().Status.IsHibernated = false

					var replicas int32 = 4
					var dwdMeltdownProtectionActive = map[string]string{
						dwdapi.MeltdownProtectionActive: "",
					}
					kubernetesClient.EXPECT().Client().Return(c).AnyTimes()
					c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: "kube-controller-manager"}, gomock.AssignableToTypeOf(&appsv1.Deployment{})).DoAndReturn(func(_ context.Context, _ types.NamespacedName, obj *appsv1.Deployment, _ ...client.GetOption) error {
						obj.Spec.Replicas = ptr.To(replicas)
						obj.Annotations = dwdMeltdownProtectionActive
						return nil
					}).AnyTimes()
					c.EXPECT().Get(gomock.Any(), client.ObjectKey{Namespace: namespace, Name: "kube-controller-manager"}, gomock.AssignableToTypeOf(&appsv1.Deployment{})).AnyTimes()

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
					c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: "kube-controller-manager"}, gomock.AssignableToTypeOf(&appsv1.Deployment{})).DoAndReturn(func(_ context.Context, _ types.NamespacedName, obj *appsv1.Deployment, _ ...client.GetOption) error {
						obj.Spec.Replicas = ptr.To[int32](0)
						return nil
					})
					kubeControllerManager.EXPECT().SetReplicaCount(int32(0))
					c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: "kube-controller-manager"}, gomock.AssignableToTypeOf(&appsv1.Deployment{})).AnyTimes()
					Expect(botanist.DeployKubeControllerManager(ctx)).To(Succeed())
				})

				It("hibernation enabled and kube-controller-manager deployment does not exist", func() {
					botanist.Shoot.HibernationEnabled = true
					kubernetesClient.EXPECT().Client().Return(c)
					c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: "kube-controller-manager"}, gomock.AssignableToTypeOf(&appsv1.Deployment{})).Return(apierrors.NewNotFound(appsv1.Resource("Deployment"), "kube-controller-manager"))
					kubeControllerManager.EXPECT().SetReplicaCount(int32(0))

					Expect(botanist.DeployKubeControllerManager(ctx)).To(Succeed())
				})

				It("hibernation enabled and kube-controller-manager is already scaled up", func() {
					botanist.Shoot.HibernationEnabled = true
					kubernetesClient.EXPECT().Client().Return(c)
					c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: "kube-controller-manager"}, gomock.AssignableToTypeOf(&appsv1.Deployment{})).DoAndReturn(func(_ context.Context, _ types.NamespacedName, obj *appsv1.Deployment, _ ...client.GetOption) error {
						obj.Spec.Replicas = ptr.To[int32](1)
						return nil
					})
					kubeControllerManager.EXPECT().SetReplicaCount(int32(1))

					Expect(botanist.DeployKubeControllerManager(ctx)).To(Succeed())
				})

				It("hibernation disabled", func() {
					kubeAPIServer.EXPECT().GetAutoscalingReplicas().Return(ptr.To[int32](1))
					botanist.Shoot.HibernationEnabled = false

					kubeControllerManager.EXPECT().SetReplicaCount(int32(1))
					kubernetesClient.EXPECT().Client().Return(c)
					c.EXPECT().Get(gomock.Any(), client.ObjectKey{Namespace: namespace, Name: "kube-controller-manager"}, gomock.AssignableToTypeOf(&appsv1.Deployment{})).AnyTimes()
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
					c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: "kube-controller-manager"}, gomock.AssignableToTypeOf(&appsv1.Deployment{})).DoAndReturn(func(_ context.Context, _ types.NamespacedName, obj *appsv1.Deployment, _ ...client.GetOption) error {
						obj.Spec.Replicas = ptr.To[int32](0)
						return nil
					})
					kubeControllerManager.EXPECT().SetReplicaCount(int32(0))

					Expect(botanist.DeployKubeControllerManager(ctx)).To(Succeed())
				})

				It("hibernation enabled and kube-controller-manager deployment does not exist", func() {
					botanist.Shoot.HibernationEnabled = true
					kubernetesClient.EXPECT().Client().Return(c)
					c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: "kube-controller-manager"}, gomock.AssignableToTypeOf(&appsv1.Deployment{})).Return(apierrors.NewNotFound(appsv1.Resource("Deployment"), "kube-controller-manager"))
					kubeControllerManager.EXPECT().SetReplicaCount(int32(0))

					Expect(botanist.DeployKubeControllerManager(ctx)).To(Succeed())
				})

				It("hibernation enabled and kube-controller-manager is already scaled up", func() {
					botanist.Shoot.HibernationEnabled = true
					kubernetesClient.EXPECT().Client().Return(c)
					c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: "kube-controller-manager"}, gomock.AssignableToTypeOf(&appsv1.Deployment{})).DoAndReturn(func(_ context.Context, _ types.NamespacedName, obj *appsv1.Deployment, _ ...client.GetOption) error {
						obj.Spec.Replicas = ptr.To[int32](1)
						return nil
					})
					kubeControllerManager.EXPECT().SetReplicaCount(int32(1))

					Expect(botanist.DeployKubeControllerManager(ctx)).To(Succeed())
				})

				It("hibernation disabled", func() {
					kubeAPIServer.EXPECT().GetAutoscalingReplicas().Return(ptr.To[int32](1))
					botanist.Shoot.HibernationEnabled = false

					kubeControllerManager.EXPECT().SetReplicaCount(int32(1))

					kubernetesClient.EXPECT().Client().Return(c)
					c.EXPECT().Get(gomock.Any(), client.ObjectKey{Namespace: namespace, Name: "kube-controller-manager"}, gomock.AssignableToTypeOf(&appsv1.Deployment{}))
					Expect(botanist.DeployKubeControllerManager(ctx)).To(Succeed())
				})
			})
		})

		It("should fail when the replicas cannot be determined", func() {
			formattedFakeErr := fmt.Errorf("failed to check if deployment \"kube-controller-manager\" is controlled by dependency-watchdog: failed to get deployment \"kube-controller-manager\": fake err")
			kubernetesClient.EXPECT().Client().Return(c)
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: "kube-controller-manager"}, gomock.AssignableToTypeOf(&appsv1.Deployment{})).Return(fakeErr)

			Expect(botanist.DeployKubeControllerManager(ctx).Error()).To(Equal(formattedFakeErr.Error()))
		})

		It("should fail when the deploy function fails", func() {

			kubeAPIServer.EXPECT().GetAutoscalingReplicas().Return(ptr.To[int32](0)).AnyTimes()
			kubeAPIServer.EXPECT().GetValues().Return(kubeapiserver.Values{RuntimeConfig: map[string]bool{"foo": true}}).AnyTimes()
			kubeControllerManager.EXPECT().SetReplicaCount(int32(0))
			kubeControllerManager.EXPECT().SetRuntimeConfig(map[string]bool{"foo": true})
			kubeControllerManager.EXPECT().SetServiceNetworks(botanist.Shoot.Networks.Services)
			kubeControllerManager.EXPECT().SetPodNetworks(botanist.Shoot.Networks.Pods)
			kubeControllerManager.EXPECT().Deploy(ctx).Return(fakeErr)

			kubernetesClient.EXPECT().Client().Return(c)
			c.EXPECT().Get(gomock.Any(), client.ObjectKey{Namespace: namespace, Name: "kube-controller-manager"}, gomock.AssignableToTypeOf(&appsv1.Deployment{})).AnyTimes()

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
				ControlPlaneNamespace: namespace,
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
