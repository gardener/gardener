// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist_test

import (
	"context"
	"errors"
	"net"

	"github.com/Masterminds/semver/v3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	kubernetesmock "github.com/gardener/gardener/pkg/client/kubernetes/mock"
	mockclusterautoscaler "github.com/gardener/gardener/pkg/component/autoscaling/clusterautoscaler/mock"
	mockworker "github.com/gardener/gardener/pkg/component/extensions/worker/mock"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	. "github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
	seedpkg "github.com/gardener/gardener/pkg/gardenlet/operation/seed"
	shootpkg "github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
	mockclient "github.com/gardener/gardener/third_party/mock/controller-runtime/client"
)

var _ = Describe("ClusterAutoscaler", func() {
	var (
		ctx     = context.TODO()
		fakeErr = errors.New("fake err")

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
		botanist.Shoot = &shootpkg.Shoot{Networks: &shootpkg.Networks{}}
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
				botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{})

				clusterAutoscaler.EXPECT().SetNamespaceUID(namespaceUID)
				worker.EXPECT().MachineDeployments().Return(machineDeployments)
				clusterAutoscaler.EXPECT().SetMachineDeployments(machineDeployments)
				clusterAutoscaler.EXPECT().SetMaxNodesTotal(int64(0))
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

	DescribeTable("#CalculateMaxNodesForShoot",
		func(shoot *gardencorev1beta1.Shoot, expectedResult *int64) {
			if shoot.Spec.Networking != nil {
				if shoot.Spec.Networking.Pods != nil {
					_, pods, err := net.ParseCIDR(*shoot.Spec.Networking.Pods)
					Expect(err).NotTo(HaveOccurred())
					botanist.Shoot.Networks.Pods = append(botanist.Shoot.Networks.Pods, *pods)
				}
				if shoot.Spec.Networking.Nodes != nil {
					_, nodes, err := net.ParseCIDR(*shoot.Spec.Networking.Nodes)
					Expect(err).NotTo(HaveOccurred())
					botanist.Shoot.Networks.Nodes = append(botanist.Shoot.Networks.Nodes, *nodes)
				}
			}
			if shoot.Status.Networking != nil {
				for _, p := range shoot.Status.Networking.Pods {
					_, pods, err := net.ParseCIDR(p)
					Expect(err).NotTo(HaveOccurred())
					botanist.Shoot.Networks.Pods = append(botanist.Shoot.Networks.Pods, *pods)
				}
				for _, n := range shoot.Status.Networking.Nodes {
					_, nodes, err := net.ParseCIDR(n)
					Expect(err).NotTo(HaveOccurred())
					botanist.Shoot.Networks.Nodes = append(botanist.Shoot.Networks.Nodes, *nodes)
				}
			}
			maxNode, err := botanist.CalculateMaxNodesForShoot(shoot)
			Expect(err).NotTo(HaveOccurred())
			Expect(maxNode).To(Equal(expectedResult))
		},

		Entry(
			"no network",
			&gardencorev1beta1.Shoot{Spec: gardencorev1beta1.ShootSpec{
				Kubernetes: gardencorev1beta1.Kubernetes{
					KubeControllerManager: &gardencorev1beta1.KubeControllerManagerConfig{
						NodeCIDRMaskSize: ptr.To[int32](24),
					},
				},
			}},
			nil,
		),
		Entry(
			"Pods network only",
			&gardencorev1beta1.Shoot{Spec: gardencorev1beta1.ShootSpec{
				Kubernetes: gardencorev1beta1.Kubernetes{
					KubeControllerManager: &gardencorev1beta1.KubeControllerManagerConfig{
						NodeCIDRMaskSize: ptr.To[int32](24),
					},
				},
				Networking: &gardencorev1beta1.Networking{
					Pods: ptr.To("100.64.0.0/12"),
				},
			}},
			ptr.To[int64](4096),
		),
		Entry(
			"Default Pods network",
			&gardencorev1beta1.Shoot{Spec: gardencorev1beta1.ShootSpec{
				Kubernetes: gardencorev1beta1.Kubernetes{
					KubeControllerManager: &gardencorev1beta1.KubeControllerManagerConfig{
						NodeCIDRMaskSize: ptr.To[int32](24),
					},
				},
				Networking: &gardencorev1beta1.Networking{
					Pods:  ptr.To("100.64.0.0/11"),
					Nodes: ptr.To("10.250.0.0/16"),
				},
			}},
			ptr.To[int64](8192),
		),
		Entry(
			"Pods network is restriction",
			&gardencorev1beta1.Shoot{Spec: gardencorev1beta1.ShootSpec{
				Kubernetes: gardencorev1beta1.Kubernetes{
					KubeControllerManager: &gardencorev1beta1.KubeControllerManagerConfig{
						NodeCIDRMaskSize: ptr.To[int32](24),
					},
				},
				Networking: &gardencorev1beta1.Networking{
					Pods:  ptr.To("100.64.0.0/12"),
					Nodes: ptr.To("10.250.0.0/16"),
				},
			}},
			ptr.To[int64](4096),
		),
		Entry(
			"Nodes network is restriction",
			&gardencorev1beta1.Shoot{Spec: gardencorev1beta1.ShootSpec{
				Kubernetes: gardencorev1beta1.Kubernetes{
					KubeControllerManager: &gardencorev1beta1.KubeControllerManagerConfig{
						NodeCIDRMaskSize: ptr.To[int32](24),
					},
				},
				Networking: &gardencorev1beta1.Networking{
					Pods:  ptr.To("100.64.0.0/11"),
					Nodes: ptr.To("10.250.0.0/20"),
				},
			}},
			ptr.To[int64](4094),
		),
		Entry(
			"IPv6",
			&gardencorev1beta1.Shoot{Spec: gardencorev1beta1.ShootSpec{
				Kubernetes: gardencorev1beta1.Kubernetes{
					KubeControllerManager: &gardencorev1beta1.KubeControllerManagerConfig{
						NodeCIDRMaskSize: ptr.To[int32](64),
					},
				},
				Networking: &gardencorev1beta1.Networking{
					Pods:       ptr.To("2001:db8:1::/48"),
					Nodes:      ptr.To("2001:db8:2::/48"),
					IPFamilies: []gardencorev1beta1.IPFamily{gardencorev1beta1.IPFamilyIPv6},
				},
			}},
			ptr.To[int64](65536),
		),
		Entry(
			"Multiple pods network only",
			&gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					Kubernetes: gardencorev1beta1.Kubernetes{
						KubeControllerManager: &gardencorev1beta1.KubeControllerManagerConfig{
							NodeCIDRMaskSize: ptr.To[int32](24),
						},
					},
					Networking: &gardencorev1beta1.Networking{
						Pods: ptr.To("100.64.0.0/12"),
					},
				},
				Status: gardencorev1beta1.ShootStatus{
					Networking: &gardencorev1beta1.NetworkingStatus{
						Pods: []string{"10.0.0.0/24", "10.0.1.0/24"},
					},
				},
			},
			ptr.To[int64](4098),
		),
		Entry(
			"Pods network is restriction (with multiple networks)",
			&gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					Kubernetes: gardencorev1beta1.Kubernetes{
						KubeControllerManager: &gardencorev1beta1.KubeControllerManagerConfig{
							NodeCIDRMaskSize: ptr.To[int32](24),
						},
					},
					Networking: &gardencorev1beta1.Networking{
						Pods:  ptr.To("100.64.0.0/12"),
						Nodes: ptr.To("10.250.0.0/16"),
					},
				},
				Status: gardencorev1beta1.ShootStatus{
					Networking: &gardencorev1beta1.NetworkingStatus{
						Pods:  []string{"100.80.0.0/12", "100.96.0.0/12"},
						Nodes: []string{"10.251.0.0/16"},
					},
				},
			},
			ptr.To[int64](12288),
		),
		Entry(
			"Nodes network is restriction (with multiple networks)",
			&gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					Kubernetes: gardencorev1beta1.Kubernetes{
						KubeControllerManager: &gardencorev1beta1.KubeControllerManagerConfig{
							NodeCIDRMaskSize: ptr.To[int32](24),
						},
					},
					Networking: &gardencorev1beta1.Networking{
						Pods:  ptr.To("100.64.0.0/11"),
						Nodes: ptr.To("10.250.0.0/20"),
					},
				},
				Status: gardencorev1beta1.ShootStatus{
					Networking: &gardencorev1beta1.NetworkingStatus{
						Pods:  []string{"100.96.0.0/11"},
						Nodes: []string{"10.251.0.0/20"},
					},
				},
			},
			ptr.To[int64](8188),
		),
	)
})
