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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	kubernetesmock "github.com/gardener/gardener/pkg/client/kubernetes/mock"
	mockvpnseedserver "github.com/gardener/gardener/pkg/component/networking/vpn/seedserver/mock"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	. "github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
	"github.com/gardener/gardener/pkg/gardenlet/operation/garden"
	"github.com/gardener/gardener/pkg/gardenlet/operation/seed"
	shootpkg "github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
)

var _ = Describe("VPNSeedServer", func() {
	var (
		ctrl     *gomock.Controller
		botanist *Botanist
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		botanist = &Botanist{Operation: &operation.Operation{
			Garden: &garden.Garden{},
		}}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#DefaultVPNSeedServer", func() {
		var kubernetesClient *kubernetesmock.MockInterface

		BeforeEach(func() {
			kubernetesClient = kubernetesmock.NewMockInterface(ctrl)
			kubernetesClient.EXPECT().Version()

			botanist.SeedClientSet = kubernetesClient
			botanist.Shoot = &shootpkg.Shoot{}
			botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					Networking: &gardencorev1beta1.Networking{
						Nodes: ptr.To("10.0.0.0/24"),
					},
				},
			})
			botanist.Seed = &seed.Seed{
				KubernetesVersion: semver.MustParse("1.31.1"),
			}
			botanist.Seed.SetInfo(&gardencorev1beta1.Seed{})
			botanist.Config = &gardenletconfigv1alpha1.GardenletConfiguration{
				SNI: &gardenletconfigv1alpha1.SNI{
					Ingress: &gardenletconfigv1alpha1.SNIIngress{
						Namespace: ptr.To("test-ns"),
						Labels: map[string]string{
							"istio": "foo-bar",
						},
					},
				},
			}
		})

		It("should successfully create a vpn seed server interface", func() {
			kubernetesClient.EXPECT().Client()
			kubernetesClient.EXPECT().Version()

			vpnSeedServer, err := botanist.DefaultVPNSeedServer()
			Expect(vpnSeedServer).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())
		})

		DescribeTable("should correctly set the deployment replicas",
			func(hibernated, highAvailable bool, expectedReplicas int) {
				kubernetesClient.EXPECT().Client()
				kubernetesClient.EXPECT().Version()
				botanist.Shoot.HibernationEnabled = hibernated
				if highAvailable {
					botanist.Shoot.VPNHighAvailabilityEnabled = highAvailable
					botanist.Shoot.VPNHighAvailabilityNumberOfSeedServers = 2
				}

				vpnSeedServer, err := botanist.DefaultVPNSeedServer()
				Expect(vpnSeedServer).NotTo(BeNil())
				Expect(vpnSeedServer.GetValues().Replicas).To(Equal(int32(expectedReplicas)))
				Expect(err).NotTo(HaveOccurred())
			},

			Entry("non-HA & awake", false, false, 1),
			Entry("non-HA & hibernated", true, false, 0),
			Entry("HA & awake", false, true, 2),
			Entry("HA & hibernated", true, true, 0),
		)
	})

	Describe("#DeployVPNSeedServer", func() {
		var (
			vpnSeedServer *mockvpnseedserver.MockInterface

			ctx     = context.TODO()
			fakeErr = errors.New("fake err")

			namespaceUID = types.UID("1234")
		)

		BeforeEach(func() {
			vpnSeedServer = mockvpnseedserver.NewMockInterface(ctrl)

			botanist.Shoot = &shootpkg.Shoot{
				Components: &shootpkg.Components{
					ControlPlane: &shootpkg.ControlPlane{
						VPNSeedServer: vpnSeedServer,
					},
				},
				Networks: &shootpkg.Networks{
					Services: []net.IPNet{{IP: net.IP{10, 0, 1, 0}, Mask: net.CIDRMask(24, 32)}},
					Pods:     []net.IPNet{{IP: net.IP{10, 0, 2, 0}, Mask: net.CIDRMask(24, 32)}},
					Nodes:    []net.IPNet{{IP: net.IP{10, 0, 3, 0}, Mask: net.CIDRMask(24, 32)}},
				},
			}
			botanist.Config = &gardenletconfigv1alpha1.GardenletConfiguration{
				SNI: &gardenletconfigv1alpha1.SNI{
					Ingress: &gardenletconfigv1alpha1.SNIIngress{
						Namespace: ptr.To("test-ns"),
						Labels: map[string]string{
							"istio": "foo-bar",
						},
					},
				},
			}
			botanist.SeedNamespaceObject = &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					UID: "1234",
				},
			}
		})

		BeforeEach(func() {
			vpnSeedServer.EXPECT().SetNodeNetworkCIDRs(botanist.Shoot.Networks.Nodes)
			vpnSeedServer.EXPECT().SetServiceNetworkCIDRs(botanist.Shoot.Networks.Services)
			vpnSeedServer.EXPECT().SetPodNetworkCIDRs(botanist.Shoot.Networks.Pods)
			vpnSeedServer.EXPECT().SetSeedNamespaceObjectUID(namespaceUID)
		})

		It("should set the secrets and SNI config and deploy", func() {
			vpnSeedServer.EXPECT().Deploy(ctx)
			Expect(botanist.DeployVPNServer(ctx)).To(Succeed())
		})

		It("should fail when the deploy function fails", func() {
			vpnSeedServer.EXPECT().Deploy(ctx).Return(fakeErr)
			Expect(botanist.DeployVPNServer(ctx)).To(Equal(fakeErr))
		})
	})
})
