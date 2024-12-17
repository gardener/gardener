// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist_test

import (
	"context"
	"errors"
	"net"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	kubernetesmock "github.com/gardener/gardener/pkg/client/kubernetes/mock"
	mockcoredns "github.com/gardener/gardener/pkg/component/networking/coredns/mock"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	. "github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
	"github.com/gardener/gardener/pkg/gardenlet/operation/garden"
	shootpkg "github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
	"github.com/gardener/gardener/pkg/utils/test"
)

var _ = Describe("CoreDNS", func() {
	var (
		ctrl     *gomock.Controller
		botanist *Botanist
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		botanist = &Botanist{Operation: &operation.Operation{}}
		botanist.Shoot = &shootpkg.Shoot{}
		botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
			Spec: gardencorev1beta1.ShootSpec{
				Kubernetes: gardencorev1beta1.Kubernetes{
					Version: "1.31.1",
				},
			},
		})
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#DefaultCoreDNS", func() {
		var kubernetesClient *kubernetesmock.MockInterface

		BeforeEach(func() {
			kubernetesClient = kubernetesmock.NewMockInterface(ctrl)

			botanist.SeedClientSet = kubernetesClient
			botanist.Garden = &garden.Garden{}
		})

		It("should successfully create a coredns interface", func() {
			kubernetesClient.EXPECT().Client()

			coreDNS, err := botanist.DefaultCoreDNS()
			Expect(coreDNS).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())
		})

		Describe("#WithClusterProportionalAutoscaler", func() {
			BeforeEach(func() {
				botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
					Spec: gardencorev1beta1.ShootSpec{
						SystemComponents: &gardencorev1beta1.SystemComponents{
							CoreDNS: &gardencorev1beta1.CoreDNS{
								Autoscaling: &gardencorev1beta1.CoreDNSAutoscaling{
									Mode: gardencorev1beta1.CoreDNSAutoscalingModeClusterProportional,
								},
							},
						},
						Kubernetes: gardencorev1beta1.Kubernetes{
							Version: "1.31.1",
						},
						Networking: &gardencorev1beta1.Networking{},
					},
				})
			})

			It("should successfully create a coredns interface with cluster-proportional autoscaling enabled", func() {
				kubernetesClient.EXPECT().Client()

				coreDNS, err := botanist.DefaultCoreDNS()
				Expect(coreDNS).NotTo(BeNil())
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})

	Describe("#DeployCoreDNS", func() {
		var (
			coreDNS          *mockcoredns.MockInterface
			kubernetesClient *kubernetesmock.MockInterface
			c                client.Client

			ctx     = context.TODO()
			fakeErr = errors.New("fake err")
		)

		BeforeEach(func() {
			coreDNS = mockcoredns.NewMockInterface(ctrl)
			kubernetesClient = kubernetesmock.NewMockInterface(ctrl)
			c = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()

			botanist.ShootClientSet = kubernetesClient
			botanist.Shoot.Components = &shootpkg.Components{
				SystemComponents: &shootpkg.SystemComponents{
					CoreDNS: coreDNS,
				},
			}
			botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					Networking: &gardencorev1beta1.Networking{
						IPFamilies: []gardencorev1beta1.IPFamily{gardencorev1beta1.IPFamilyIPv4},
					},
				},
			})
			botanist.Shoot.Networks = &shootpkg.Networks{
				CoreDNS: []net.IP{net.ParseIP("18.19.20.21"), net.ParseIP("2001:db8::1")},
				Pods:    []net.IPNet{{IP: net.ParseIP("22.23.24.25")}, {IP: net.ParseIP("2001:db8::2")}},
				Nodes:   []net.IPNet{{IP: net.ParseIP("26.27.28.29")}, {IP: net.ParseIP("2001:db8::3")}},
			}

			coreDNS.EXPECT().SetNodeNetworkCIDRs(botanist.Shoot.Networks.Nodes)
			coreDNS.EXPECT().SetPodNetworkCIDRs(botanist.Shoot.Networks.Pods)
			coreDNS.EXPECT().SetClusterIPs(botanist.Shoot.Networks.CoreDNS)
		})

		It("should fail when the deploy function fails", func() {
			kubernetesClient.EXPECT().Client().Return(c)

			coreDNS.EXPECT().SetPodAnnotations(nil)
			coreDNS.EXPECT().SetIPFamilies([]gardencorev1beta1.IPFamily{gardencorev1beta1.IPFamilyIPv4})
			coreDNS.EXPECT().Deploy(ctx).Return(fakeErr)

			Expect(botanist.DeployCoreDNS(ctx)).To(MatchError(fakeErr))
		})

		It("should successfully deploy (coredns deployment not yet found)", func() {
			kubernetesClient.EXPECT().Client().Return(c)

			coreDNS.EXPECT().SetPodAnnotations(nil)
			coreDNS.EXPECT().SetIPFamilies([]gardencorev1beta1.IPFamily{gardencorev1beta1.IPFamilyIPv4})
			coreDNS.EXPECT().Deploy(ctx)

			Expect(botanist.DeployCoreDNS(ctx)).To(Succeed())
		})

		It("should successfully deploy (restart task annotation found)", func() {
			nowFunc := func() time.Time {
				return time.Date(1, 1, 1, 1, 1, 1, 1, time.UTC)
			}
			defer test.WithVar(&NowFunc, nowFunc)()

			shoot := botanist.Shoot.GetInfo()
			shoot.Annotations = map[string]string{"shoot.gardener.cloud/tasks": "restartCoreAddons"}
			botanist.Shoot.SetInfo(shoot)

			coreDNS.EXPECT().SetPodAnnotations(map[string]string{"gardener.cloud/restarted-at": nowFunc().Format(time.RFC3339)})
			coreDNS.EXPECT().SetIPFamilies([]gardencorev1beta1.IPFamily{gardencorev1beta1.IPFamilyIPv4})
			coreDNS.EXPECT().Deploy(ctx)

			Expect(botanist.DeployCoreDNS(ctx)).To(Succeed())
		})

		It("should successfully deploy (existing annotation found)", func() {
			annotations := map[string]string{"gardener.cloud/restarted-at": "2014-02-13T10:36:36Z"}

			kubernetesClient.EXPECT().Client().Return(c)
			Expect(c.Create(ctx, &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "coredns",
					Namespace: "kube-system",
				},
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Annotations: annotations,
						},
					},
				},
			})).To(Succeed())

			coreDNS.EXPECT().SetPodAnnotations(annotations)
			coreDNS.EXPECT().SetIPFamilies([]gardencorev1beta1.IPFamily{gardencorev1beta1.IPFamilyIPv4})
			coreDNS.EXPECT().Deploy(ctx)

			Expect(botanist.DeployCoreDNS(ctx)).To(Succeed())
		})

		It("should successfully deploy with dual-stack enabled", func() {

			botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					Networking: &gardencorev1beta1.Networking{
						IPFamilies: []gardencorev1beta1.IPFamily{gardencorev1beta1.IPFamilyIPv4, gardencorev1beta1.IPFamilyIPv6},
					},
				},
			})
			kubernetesClient.EXPECT().Client().Return(c)

			coreDNS.EXPECT().SetPodAnnotations(nil)
			coreDNS.EXPECT().SetIPFamilies([]gardencorev1beta1.IPFamily{gardencorev1beta1.IPFamilyIPv4, gardencorev1beta1.IPFamilyIPv6})
			coreDNS.EXPECT().Deploy(ctx)

			Expect(botanist.DeployCoreDNS(ctx)).To(Succeed())
		})
	})
})
