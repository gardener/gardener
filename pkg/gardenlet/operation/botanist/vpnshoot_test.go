// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist_test

import (
	"context"
	"errors"
	"net"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/clock"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	kubernetesmock "github.com/gardener/gardener/pkg/client/kubernetes/mock"
	mockvpnshoot "github.com/gardener/gardener/pkg/component/networking/vpn/shoot/mock"
	"github.com/gardener/gardener/pkg/features"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	. "github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
	seedpkg "github.com/gardener/gardener/pkg/gardenlet/operation/seed"
	shootpkg "github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("VPNShoot", func() {
	var (
		ctrl     *gomock.Controller
		botanist *Botanist
	)
	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())

		botanist = &Botanist{Operation: &operation.Operation{
			Clock: clock.RealClock{},
		}}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#DefaultVPNShoot", func() {
		var kubernetesClient *kubernetesmock.MockInterface

		BeforeEach(func() {
			kubernetesClient = kubernetesmock.NewMockInterface(ctrl)
			botanist.SeedClientSet = kubernetesClient
			botanist.Shoot = &shootpkg.Shoot{
				Networks: &shootpkg.Networks{
					Pods:     []net.IPNet{{IP: []byte("192.168.0.0"), Mask: []byte("16")}},
					Services: []net.IPNet{{IP: []byte("10.0.0.0"), Mask: []byte("24")}},
					Nodes:    []net.IPNet{{IP: []byte("10.181.0.0"), Mask: []byte("16")}},
				},
				ExternalClusterDomain: ptr.To("foo.local.gardener.cloud"),
			}
			botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					Kubernetes: gardencorev1beta1.Kubernetes{
						Version: "1.30.1",
					},
					Networking: &gardencorev1beta1.Networking{
						IPFamilies: []gardencorev1beta1.IPFamily{gardencorev1beta1.IPFamilyIPv4},
						Pods:       ptr.To("192.168.0.0/16"),
						Services:   ptr.To("10.0.0.0/24"),
						Nodes:      ptr.To("10.181.0.0/16"),
					},
				},
			})
			botanist.Seed = &seedpkg.Seed{}
			botanist.Seed.SetInfo(&gardencorev1beta1.Seed{})
		})

		It("should successfully create a vpnShoot interface for ReversedVPN", func() {
			kubernetesClient.EXPECT().Client()

			vpnShoot, err := botanist.DefaultVPNShoot()
			Expect(vpnShoot).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("#DeployVPNShoot", func() {
		var (
			vpnShoot *mockvpnshoot.MockInterface
			ctx      = context.TODO()
			shoot    *gardencorev1beta1.Shoot
			fakeErr  = errors.New("fake err")
		)

		BeforeEach(func() {
			vpnShoot = mockvpnshoot.NewMockInterface(ctrl)

			shoot = &gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "bar",
					Namespace: "foo",
				},
			}
			botanist.Shoot = &shootpkg.Shoot{
				Components: &shootpkg.Components{
					SystemComponents: &shootpkg.SystemComponents{
						VPNShoot: vpnShoot,
					},
				},
				Networks: &shootpkg.Networks{
					Pods:     []net.IPNet{{IP: []byte("192.168.0.0"), Mask: []byte("16")}},
					Services: []net.IPNet{{IP: []byte("10.0.0.0"), Mask: []byte("24")}},
					Nodes:    []net.IPNet{{IP: []byte("10.181.0.0"), Mask: []byte("16")}},
				},
			}

			botanist.Shoot.SetInfo(shoot)
			gardenClient := fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).WithStatusSubresource(&gardencorev1beta1.Shoot{}).WithObjects(shoot).Build()
			botanist.GardenClient = gardenClient
		})

		BeforeEach(func() {
			vpnShoot.EXPECT().SetNodeNetworkCIDRs(botanist.Shoot.Networks.Nodes)
			vpnShoot.EXPECT().SetServiceNetworkCIDRs(botanist.Shoot.Networks.Services)
			vpnShoot.EXPECT().SetPodNetworkCIDRs(botanist.Shoot.Networks.Pods)
		})

		It("should set the network ranges and deploy", func() {
			vpnShoot.EXPECT().Deploy(ctx)
			Expect(botanist.DeployVPNShoot(ctx)).To(Succeed())

			Expect(botanist.GardenClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
			Expect(shoot.Status.Constraints).NotTo(ContainCondition(
				OfType(gardencorev1beta1.ShootUsesUnifiedHTTPProxyPort),
			))
		})

		It("should report a constraint if feature gate UseUnifiedHTTPProxyPort is enabled", func() {
			DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.UseUnifiedHTTPProxyPort, true))

			vpnShoot.EXPECT().Deploy(ctx)
			Expect(botanist.DeployVPNShoot(ctx)).To(Succeed())
			Expect(botanist.GardenClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())

			Expect(shoot.Status.Constraints).To(ContainCondition(
				OfType(gardencorev1beta1.ShootUsesUnifiedHTTPProxyPort),
				WithStatus(gardencorev1beta1.ConditionTrue),
				WithReason("ShootUsesUnifiedHTTPProxyPort"),
			))
		})

		It("should fail when the deploy function fails", func() {
			vpnShoot.EXPECT().Deploy(ctx).Return(fakeErr)
			Expect(botanist.DeployVPNShoot(ctx)).To(Equal(fakeErr))
		})
	})
})
