// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
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
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/apis/config/gardenlet/v1alpha1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	mockdnsrecord "github.com/gardener/gardener/pkg/component/extensions/dnsrecord/mock"
	mockvpnshoot "github.com/gardener/gardener/pkg/component/networking/vpn/shoot/mock"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	. "github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
	"github.com/gardener/gardener/pkg/gardenlet/operation/garden"
	seedpkg "github.com/gardener/gardener/pkg/gardenlet/operation/seed"
	shootpkg "github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

var _ = Describe("VPN LiveMigration", func() {
	Describe("#LiveMigrationTemporaryVPNDNSName", func() {
		It("should prepend vpn-tmp. to the internal cluster domain", func() {
			Expect(LiveMigrationTemporaryVPNDNSName("foo.bar.internal.example.com")).
				To(Equal("vpn-tmp.foo.bar.internal.example.com"))
		})
	})

	Describe("Botanist VPN live-migration functions", func() {
		const (
			controlPlaneNS  = "shoot--foo--bar"
			istioNS         = "istio-ingress"
			internalDomainS = "foo.bar.internal.example.com"
			shootNameS      = "bar"
			ttlS      int64 = 300
		)

		var (
			ctrl       *gomock.Controller
			b          *Botanist
			fakeClient client.Client
			ctx        = context.TODO()
		)

		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())

			fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
			b = &Botanist{
				Operation: &operation.Operation{
					Config: &gardenletconfigv1alpha1.GardenletConfiguration{
						Controllers: &gardenletconfigv1alpha1.GardenletControllerConfiguration{
							Shoot: &gardenletconfigv1alpha1.ShootControllerConfiguration{
								DNSEntryTTLSeconds: new(ttlS),
							},
						},
						SNI: &gardenletconfigv1alpha1.SNI{
							Ingress: &gardenletconfigv1alpha1.SNIIngress{
								Namespace: new(istioNS),
								Labels: map[string]string{
									"app":   "istio-ingressgateway",
									"istio": "ingressgateway",
								},
							},
						},
					},
					Shoot: &shootpkg.Shoot{
						ControlPlaneNamespace: controlPlaneNS,
						InternalClusterDomain: new(internalDomainS),
					},
					Garden: &garden.Garden{
						InternalDomain: &gardenerutils.Domain{
							Provider: "internal-provider",
							Zone:     "internal-zone",
							Credentials: &corev1.Secret{
								Data: map[string][]byte{"key": []byte("val")},
							},
						},
					},
					Seed: &seedpkg.Seed{},
				},
			}
			b.Shoot.SetInfo(&gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      shootNameS,
					Namespace: "garden",
				},
				Spec: gardencorev1beta1.ShootSpec{
					Networking: &gardencorev1beta1.Networking{
						IPFamilies: []gardencorev1beta1.IPFamily{gardencorev1beta1.IPFamilyIPv4},
					},
				},
			})
			b.Seed.SetInfo(&gardencorev1beta1.Seed{})
			b.SeedClientSet = fakekubernetes.NewClientSetBuilder().WithClient(fakeClient).Build()
		})

		AfterEach(func() {
			ctrl.Finish()
		})

		Describe("#DefaultLiveMigrationVPNDNSRecord", func() {
			It("should build a component with the correct values when internal DNS is available", func() {
				r := b.DefaultLiveMigrationVPNDNSRecord()

				vals := r.GetValues()
				Expect(vals.Name).To(Equal(shootNameS + "-live-migration-vpn"))
				Expect(vals.SecretName).To(Equal(DNSRecordSecretPrefix + "-" + shootNameS + "-live-migration-vpn"))
				Expect(vals.Namespace).To(Equal(controlPlaneNS))
				Expect(vals.TTL).To(Equal(new(ttlS)))
				Expect(vals.Type).To(Equal("internal-provider"))
				Expect(vals.Zone).To(Equal(new("internal-zone")))
				Expect(vals.DNSName).To(Equal("vpn-tmp." + internalDomainS))
				Expect(vals.AnnotateOperation).To(BeTrue())
				Expect(vals.IPStack).To(Equal("ipv4"))
				Expect(vals.Labels).To(Equal(map[string]string{
					v1beta1constants.LabelRole:  "live-migration-vpn",
					v1beta1constants.GardenRole: v1beta1constants.GardenRoleControlPlane,
				}))
			})

			It("should build a component with no DNS provider when garden has no internal domain", func() {
				b.Garden.InternalDomain = nil

				r := b.DefaultLiveMigrationVPNDNSRecord()

				vals := r.GetValues()
				Expect(vals.Type).To(BeEmpty())
				Expect(vals.DNSName).To(BeEmpty())
			})
		})

		Describe("#DeployLiveMigrationVPNDNSRecord / #DestroyLiveMigrationVPNDNSRecord", func() {
			var mockDNS *mockdnsrecord.MockInterface

			BeforeEach(func() {
				mockDNS = mockdnsrecord.NewMockInterface(ctrl)
				b.Shoot.Components = &shootpkg.Components{
					Extensions: &shootpkg.Extensions{
						LiveMigrationVPNDNSRecord: mockDNS,
					},
				}
				b.APIServerAddress = "1.2.3.4"
			})

			It("should set record type and values then deploy", func() {
				mockDNS.EXPECT().SetRecordType(gomock.Any())
				mockDNS.EXPECT().SetValues([]string{"1.2.3.4"})
				mockDNS.EXPECT().Deploy(ctx)
				Expect(b.DeployLiveMigrationVPNDNSRecord(ctx)).To(Succeed())
			})

			It("should return deploy error", func() {
				fakeErr := errors.New("deploy-failed")
				mockDNS.EXPECT().SetRecordType(gomock.Any())
				mockDNS.EXPECT().SetValues([]string{"1.2.3.4"})
				mockDNS.EXPECT().Deploy(ctx).Return(fakeErr)
				Expect(b.DeployLiveMigrationVPNDNSRecord(ctx)).To(MatchError(fakeErr))
			})

			It("should destroy and wait for cleanup", func() {
				mockDNS.EXPECT().Destroy(ctx)
				mockDNS.EXPECT().WaitCleanup(ctx)
				Expect(b.DestroyLiveMigrationVPNDNSRecord(ctx)).To(Succeed())
			})

			It("should return destroy error without calling WaitCleanup", func() {
				fakeErr := errors.New("destroy-failed")
				mockDNS.EXPECT().Destroy(ctx).Return(fakeErr)
				Expect(b.DestroyLiveMigrationVPNDNSRecord(ctx)).To(MatchError(fakeErr))
			})
		})

		Describe("#DefaultTemporaryVPNShoot", func() {
			BeforeEach(func() {
				// ShootVersion() reads from KubernetesVersion
				b.Shoot.KubernetesVersion = semver.MustParse("1.31.1")
				b.Shoot.SetInfo(&gardencorev1beta1.Shoot{
					ObjectMeta: metav1.ObjectMeta{Name: shootNameS, Namespace: "garden"},
					Spec: gardencorev1beta1.ShootSpec{
						Kubernetes: gardencorev1beta1.Kubernetes{Version: "1.31.1"},
						Networking: &gardencorev1beta1.Networking{
							IPFamilies: []gardencorev1beta1.IPFamily{gardencorev1beta1.IPFamilyIPv4},
						},
					},
				})
				b.Seed.SetInfo(&gardencorev1beta1.Seed{})
			})

			It("should return a non-nil vpnshoot.Interface without error", func() {
				component, err := b.DefaultTemporaryVPNShoot()
				Expect(err).NotTo(HaveOccurred())
				Expect(component).NotTo(BeNil())
			})
		})

		Describe("#DeployTemporaryVPNShoot / #DestroyTemporaryVPNShoot", func() {
			var mockVPN *mockvpnshoot.MockInterface

			BeforeEach(func() {
				mockVPN = mockvpnshoot.NewMockInterface(ctrl)
				networks := &shootpkg.Networks{
					Pods:     []net.IPNet{{IP: net.IP{10, 0, 1, 0}, Mask: net.CIDRMask(24, 32)}},
					Services: []net.IPNet{{IP: net.IP{10, 0, 2, 0}, Mask: net.CIDRMask(24, 32)}},
					Nodes:    []net.IPNet{{IP: net.IP{10, 0, 3, 0}, Mask: net.CIDRMask(24, 32)}},
				}
				b.Shoot.Networks = networks
				b.Shoot.Components = &shootpkg.Components{
					SystemComponents: &shootpkg.SystemComponents{
						TemporaryVPNShoot: mockVPN,
					},
				}
			})

			It("should set network CIDRs and deploy", func() {
				nets := b.Shoot.Networks
				mockVPN.EXPECT().SetPodNetworkCIDRs(nets.Pods)
				mockVPN.EXPECT().SetServiceNetworkCIDRs(nets.Services)
				mockVPN.EXPECT().SetNodeNetworkCIDRs(nets.Nodes)
				mockVPN.EXPECT().Deploy(ctx)
				Expect(b.DeployTemporaryVPNShoot(ctx)).To(Succeed())
			})

			It("should return deploy error", func() {
				fakeErr := errors.New("vpn-deploy-failed")
				nets := b.Shoot.Networks
				mockVPN.EXPECT().SetPodNetworkCIDRs(nets.Pods)
				mockVPN.EXPECT().SetServiceNetworkCIDRs(nets.Services)
				mockVPN.EXPECT().SetNodeNetworkCIDRs(nets.Nodes)
				mockVPN.EXPECT().Deploy(ctx).Return(fakeErr)
				Expect(b.DeployTemporaryVPNShoot(ctx)).To(MatchError(fakeErr))
			})

			It("should call Destroy", func() {
				mockVPN.EXPECT().Destroy(ctx)
				Expect(b.DestroyTemporaryVPNShoot(ctx)).To(Succeed())
			})

			It("should return destroy error", func() {
				fakeErr := errors.New("vpn-destroy-failed")
				mockVPN.EXPECT().Destroy(ctx).Return(fakeErr)
				Expect(b.DestroyTemporaryVPNShoot(ctx)).To(MatchError(fakeErr))
			})
		})
	})
})
