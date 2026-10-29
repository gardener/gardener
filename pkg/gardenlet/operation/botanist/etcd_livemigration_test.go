// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package botanist_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	druidcorev1alpha1 "github.com/gardener/etcd-druid/api/core/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
	testclock "k8s.io/utils/clock/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/apis/config/gardenlet/v1alpha1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/etcd/etcd"
	etcdconstants "github.com/gardener/gardener/pkg/component/etcd/etcd/constants"
	mocketcd "github.com/gardener/gardener/pkg/component/etcd/etcd/mock"
	"github.com/gardener/gardener/pkg/component/etcd/memberexposure"
	mockcomponent "github.com/gardener/gardener/pkg/component/mock"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	. "github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
	seedpkg "github.com/gardener/gardener/pkg/gardenlet/operation/seed"
	shootpkg "github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
	"github.com/gardener/gardener/pkg/utils/test"
)

var _ = Describe("Etcd LiveMigration", func() {
	var (
		ctrl *gomock.Controller

		ctx         = context.Background()
		testErr     = errors.New("test")
		namespace   = "shoot--p1--foo"
		seedName    = "src-seed"
		ingress     = "ingress.seed.example.com"
		dstSeedName = "dst-seed"
		dstIngress  = "ingress.dst.example.com"

		values       *etcd.Values
		gardenReader client.Client

		memberExposureComponent *mockcomponent.MockDeployWaiter

		actualNamespace string
		actualValues    memberexposure.Values

		b *Botanist
	)

	Describe("#DeployEtcdMemberExposure", func() {
		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())
			memberExposureComponent = mockcomponent.NewMockDeployWaiter(ctrl)

			actualNamespace = ""
			actualValues = memberexposure.Values{}
		})

		JustBeforeEach(func() {
			fakeClient := fakeclient.NewClientBuilder().WithScheme(kubernetesscheme.Scheme).Build()
			kubernetesClient := fakekubernetes.NewClientSetBuilder().WithClient(fakeClient).Build()

			b = &Botanist{Operation: &operation.Operation{
				SeedClientSet: kubernetesClient,
				Config: &gardenletconfigv1alpha1.GardenletConfiguration{
					SNI: &gardenletconfigv1alpha1.SNI{
						Ingress: &gardenletconfigv1alpha1.SNIIngress{
							Namespace: new("istio-ingress"),
							Labels:    map[string]string{"istio": "ingressgateway"},
						},
					},
				},
			}}
			b.Seed = &seedpkg.Seed{}
			b.Seed.SetInfo(&gardencorev1beta1.Seed{
				ObjectMeta: metav1.ObjectMeta{Name: seedName},
				Spec: gardencorev1beta1.SeedSpec{
					Ingress: &gardencorev1beta1.Ingress{Domain: ingress},
				},
			})
			b.Shoot = &shootpkg.Shoot{ControlPlaneNamespace: namespace}
			b.Shoot.SetInfo(&gardencorev1beta1.Shoot{})
			b.Shoot.Components = &shootpkg.Components{ControlPlane: &shootpkg.ControlPlane{}}
		})

		AfterEach(func() {
			ctrl.Finish()
		})

		It("should deploy with correct namespace, members and Istio config (no ClientHost when not source seed)", func() {
			memberExposureComponent.EXPECT().Deploy(ctx)

			DeferCleanup(test.WithVar(&NewMemberExposure, func(_ client.Client, ns string, vals memberexposure.Values) component.DeployWaiter {
				actualNamespace = ns
				actualValues = vals
				return memberExposureComponent
			}))

			b.Shoot.Components.ControlPlane.EtcdMemberExposure = b.DefaultEtcdMemberExposure()

			Expect(b.DeployEtcdMemberExposure(ctx)).To(Succeed())

			Expect(actualNamespace).To(Equal(namespace))
			Expect(actualValues.Role).To(Equal(v1beta1constants.ETCDRoleMain))
			Expect(actualValues.Members).To(HaveLen(1))
			Expect(actualValues.Members[0].PeerSNIHost).To(Equal("etcd-main-peer-0-9bd85b.ingress.seed.example.com"))
			Expect(actualValues.Members[0].PodFQDN).To(Equal("etcd-main-0.etcd-main-peer.shoot--p1--foo.svc.cluster.local"))
			Expect(actualValues.Members[0].ExternalPort).To(Equal(uint32(etcdconstants.PortEtcdPeerExternal)))
			Expect(actualValues.ClientHost).To(BeEmpty())
			Expect(actualValues.Members[0].ClientSNIHost).ToNot(BeEmpty())
			Expect(actualValues.IstioIngressGatewayNamespace).To(Equal("istio-ingress"))
			Expect(actualValues.IstioIngressGatewayLabels).To(HaveKeyWithValue("istio", "ingressgateway"))
		})

		It("should set ClientHost and ClientMemberHosts when seed is the source of a live migration", func() {
			memberExposureComponent.EXPECT().Deploy(ctx)

			DeferCleanup(test.WithVar(&NewMemberExposure, func(_ client.Client, _ string, vals memberexposure.Values) component.DeployWaiter {
				actualValues = vals
				return memberExposureComponent
			}))

			b.Shoot.SetInfo(&gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{v1beta1constants.AnnotationMigrationLiveMigrate: "true"},
				},
				Spec:   gardencorev1beta1.ShootSpec{SeedName: new(dstSeedName)},
				Status: gardencorev1beta1.ShootStatus{SeedName: new(seedName)},
			})
			b.Shoot.Components.ControlPlane.EtcdMemberExposure = b.DefaultEtcdMemberExposure()

			Expect(b.DeployEtcdMemberExposure(ctx)).To(Succeed())

			Expect(actualValues.ClientHost).To(Equal("etcd-main-client-44d4fb.ingress.seed.example.com"))
			Expect(actualValues.Members[0].ClientSNIHost).To(Equal(
				LiveMigrationEtcdClientMemberHost(seedName, namespace, ingress, v1beta1constants.ETCDRoleMain, 0),
			))
		})

		It("should set ClientMemberHosts (but no ClientHost) when seed is the destination of a live migration", func() {
			memberExposureComponent.EXPECT().Deploy(ctx)

			DeferCleanup(test.WithVar(&NewMemberExposure, func(_ client.Client, _ string, vals memberexposure.Values) component.DeployWaiter {
				actualValues = vals
				return memberExposureComponent
			}))

			b.Seed.SetInfo(&gardencorev1beta1.Seed{
				ObjectMeta: metav1.ObjectMeta{Name: dstSeedName},
				Spec:       gardencorev1beta1.SeedSpec{Ingress: &gardencorev1beta1.Ingress{Domain: dstIngress}},
			})
			b.Shoot.SetInfo(&gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{v1beta1constants.AnnotationMigrationLiveMigrate: "true"},
				},
				Spec:   gardencorev1beta1.ShootSpec{SeedName: new(dstSeedName)},
				Status: gardencorev1beta1.ShootStatus{SeedName: new(seedName)},
			})
			b.Shoot.Components.ControlPlane.EtcdMemberExposure = b.DefaultEtcdMemberExposure()

			Expect(b.DeployEtcdMemberExposure(ctx)).To(Succeed())

			Expect(actualValues.ClientHost).To(BeEmpty())
			Expect(actualValues.Members[0].ClientSNIHost).To(Equal(
				LiveMigrationEtcdClientMemberHost(dstSeedName, namespace, dstIngress, v1beta1constants.ETCDRoleMain, 0),
			))
		})

		It("should return an error when Deploy fails", func() {
			memberExposureComponent.EXPECT().Deploy(ctx).Return(testErr)

			DeferCleanup(test.WithVar(&NewMemberExposure, func(_ client.Client, _ string, _ memberexposure.Values) component.DeployWaiter {
				return memberExposureComponent
			}))

			b.Shoot.Components.ControlPlane.EtcdMemberExposure = b.DefaultEtcdMemberExposure()

			Expect(b.DeployEtcdMemberExposure(ctx)).To(MatchError(ContainSubstring(testErr.Error())))
		})

		It("should compute 3 members for an HA shoot", func() {
			memberExposureComponent.EXPECT().Deploy(ctx)

			DeferCleanup(test.WithVar(&NewMemberExposure, func(_ client.Client, _ string, vals memberexposure.Values) component.DeployWaiter {
				actualValues = vals
				return memberExposureComponent
			}))

			b.Shoot.SetInfo(&gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					ControlPlane: &gardencorev1beta1.ControlPlane{
						HighAvailability: &gardencorev1beta1.HighAvailability{
							FailureTolerance: gardencorev1beta1.FailureTolerance{Type: gardencorev1beta1.FailureToleranceTypeZone},
						},
					},
				},
			})

			b.Shoot.Components.ControlPlane.EtcdMemberExposure = b.DefaultEtcdMemberExposure()

			Expect(b.DeployEtcdMemberExposure(ctx)).To(Succeed())

			Expect(actualValues.Members).To(HaveLen(int(etcdconstants.HAReplicaCount)))
			for i := 0; i < int(etcdconstants.HAReplicaCount); i++ {
				Expect(actualValues.Members[i].ExternalPort).To(Equal(uint32(etcdconstants.PortEtcdPeerExternal) + uint32(i)))
			}
		})
	})

	Describe("#DestroyEtcdPeerExposure", func() {
		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())
			memberExposureComponent = mockcomponent.NewMockDeployWaiter(ctrl)

			actualNamespace = ""
			actualValues = memberexposure.Values{}
		})

		JustBeforeEach(func() {
			fakeClient := fakeclient.NewClientBuilder().WithScheme(kubernetesscheme.Scheme).Build()
			kubernetesClient := fakekubernetes.NewClientSetBuilder().WithClient(fakeClient).Build()

			b = &Botanist{Operation: &operation.Operation{
				SeedClientSet: kubernetesClient,
				Config: &gardenletconfigv1alpha1.GardenletConfiguration{
					SNI: &gardenletconfigv1alpha1.SNI{
						Ingress: &gardenletconfigv1alpha1.SNIIngress{
							Namespace: new("istio-ingress"),
							Labels:    map[string]string{"istio": "ingressgateway"},
						},
					},
				},
			}}
			b.Seed = &seedpkg.Seed{}
			b.Seed.SetInfo(&gardencorev1beta1.Seed{
				ObjectMeta: metav1.ObjectMeta{Name: seedName},
				Spec: gardencorev1beta1.SeedSpec{
					Ingress: &gardencorev1beta1.Ingress{Domain: ingress},
				},
			})
			b.Shoot = &shootpkg.Shoot{ControlPlaneNamespace: namespace}
			b.Shoot.SetInfo(&gardencorev1beta1.Shoot{})
			b.Shoot.Components = &shootpkg.Components{ControlPlane: &shootpkg.ControlPlane{}}
		})

		AfterEach(func() {
			ctrl.Finish()
		})

		It("should destroy with correct namespace, members and Istio config (no ClientHost)", func() {
			memberExposureComponent.EXPECT().Destroy(ctx)

			DeferCleanup(test.WithVar(&NewMemberExposure, func(_ client.Client, ns string, vals memberexposure.Values) component.DeployWaiter {
				actualNamespace = ns
				actualValues = vals
				return memberExposureComponent
			}))

			b.Shoot.Components.ControlPlane.EtcdMemberExposure = b.DefaultEtcdMemberExposure()

			Expect(b.DestroyEtcdMemberExposure(ctx)).To(Succeed())

			Expect(actualNamespace).To(Equal(namespace))
			Expect(actualValues.Role).To(Equal(v1beta1constants.ETCDRoleMain))
			Expect(actualValues.Members).To(HaveLen(1))
			Expect(actualValues.ClientHost).To(BeEmpty())
			Expect(actualValues.IstioIngressGatewayNamespace).To(Equal("istio-ingress"))
		})

		It("should return an error when Destroy fails", func() {
			memberExposureComponent.EXPECT().Destroy(ctx).Return(testErr)

			DeferCleanup(test.WithVar(&NewMemberExposure, func(_ client.Client, _ string, _ memberexposure.Values) component.DeployWaiter {
				return memberExposureComponent
			}))

			b.Shoot.Components.ControlPlane.EtcdMemberExposure = b.DefaultEtcdMemberExposure()

			Expect(b.DestroyEtcdMemberExposure(ctx)).To(MatchError(ContainSubstring(testErr.Error())))
		})
	})

	Describe("#SetLiveMigrationEtcdValues", func() {
		BeforeEach(func() {
			values = &etcd.Values{}

			s := runtime.NewScheme()
			Expect(gardencorev1beta1.AddToScheme(s)).To(Succeed())
			gardenReader = fakeclient.NewClientBuilder().WithScheme(s).WithObjects(
				&gardencorev1beta1.Seed{
					ObjectMeta: metav1.ObjectMeta{Name: seedName},
					Spec:       gardencorev1beta1.SeedSpec{Ingress: &gardencorev1beta1.Ingress{Domain: ingress}},
				},
				&gardencorev1beta1.Seed{
					ObjectMeta: metav1.ObjectMeta{Name: dstSeedName},
					Spec:       gardencorev1beta1.SeedSpec{Ingress: &gardencorev1beta1.Ingress{Domain: dstIngress}},
				},
			).Build()
		})

		JustBeforeEach(func() {
			b = &Botanist{Operation: &operation.Operation{
				GardenAPIReader: gardenReader,
			}}
			b.Seed = &seedpkg.Seed{}
			b.Shoot = &shootpkg.Shoot{ControlPlaneNamespace: namespace}
		})

		liveMigratingShoot := func(sourceSeed, destSeed string) *gardencorev1beta1.Shoot {
			return &gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						v1beta1constants.AnnotationMigrationLiveMigrate: "true",
					},
				},
				Spec:   gardencorev1beta1.ShootSpec{SeedName: new(destSeed)},
				Status: gardencorev1beta1.ShootStatus{SeedName: new(sourceSeed)},
			}
		}

		Context("no-op cases", func() {
			It("should return nil and not modify values for a self-hosted shoot", func() {
				b.Seed.SetInfo(&gardencorev1beta1.Seed{ObjectMeta: metav1.ObjectMeta{Name: seedName}})
				b.Shoot.SetInfo(&gardencorev1beta1.Shoot{
					Spec: gardencorev1beta1.ShootSpec{
						Provider: gardencorev1beta1.Provider{
							Workers: []gardencorev1beta1.Worker{
								{Name: "cp-pool", ControlPlane: &gardencorev1beta1.WorkerControlPlane{}},
							},
						},
					},
				})

				Expect(SetLiveMigrationEtcdValues(b, ctx, values, v1beta1constants.ETCDRoleMain)).To(Succeed())
				Expect(values).To(Equal(&etcd.Values{}))
			})

			It("should return nil and not modify values for a non-main role", func() {
				b.Seed.SetInfo(&gardencorev1beta1.Seed{ObjectMeta: metav1.ObjectMeta{Name: seedName}})
				b.Shoot.SetInfo(liveMigratingShoot(seedName, dstSeedName))

				Expect(SetLiveMigrationEtcdValues(b, ctx, values, v1beta1constants.ETCDRoleEvents)).To(Succeed())
				Expect(values).To(Equal(&etcd.Values{}))
			})

			It("should return nil and not modify values when the shoot is not live-migrating", func() {
				b.Seed.SetInfo(&gardencorev1beta1.Seed{ObjectMeta: metav1.ObjectMeta{Name: seedName}})
				b.Shoot.SetInfo(&gardencorev1beta1.Shoot{
					Spec:   gardencorev1beta1.ShootSpec{SeedName: new(seedName)},
					Status: gardencorev1beta1.ShootStatus{SeedName: new(seedName)},
				})

				Expect(SetLiveMigrationEtcdValues(b, ctx, values, v1beta1constants.ETCDRoleMain)).To(Succeed())
				Expect(values).To(Equal(&etcd.Values{}))
			})
		})

		Context("source role", func() {
			JustBeforeEach(func() {
				b.Seed.SetInfo(&gardencorev1beta1.Seed{
					ObjectMeta: metav1.ObjectMeta{Name: seedName},
					Spec:       gardencorev1beta1.SeedSpec{Ingress: &gardencorev1beta1.Ingress{Domain: ingress}},
				})
				b.Shoot.SetInfo(liveMigratingShoot(seedName, dstSeedName))
			})

			It("should set AdditionalAdvertisedURLs, ExtraClientServiceDNSNames, ExtraPeerServiceDNSNames and SkipClientSANVerification", func() {
				Expect(SetLiveMigrationEtcdValues(b, ctx, values, v1beta1constants.ETCDRoleMain)).To(Succeed())

				Expect(values.LiveMigration).NotTo(BeNil())
				Expect(values.LiveMigration.SkipClientSANVerification).To(BeTrue())
				Expect(values.LiveMigration.AdditionalAdvertisedURLs).To(Equal(&druidcorev1alpha1.AdditionalAdvertiseURLsSpec{
					PeerURLs: &druidcorev1alpha1.AdditionalURLsSpec{
						OverrideDefaultURL: new(true),
						Members:            ComputeMemberPeerURLs(seedName, namespace, ingress, v1beta1constants.ETCDRoleMain, 1),
					},
					ClientURLs: &druidcorev1alpha1.AdditionalURLsSpec{
						OverrideDefaultURL: new(true),
						Members:            ComputeMemberClientURLs(seedName, namespace, ingress, v1beta1constants.ETCDRoleMain, 1),
					},
				}))
				Expect(values.LiveMigration.ExtraClientServiceDNSNames).To(ConsistOf(append(
					CrossSeedClientMemberHosts(seedName, ingress, dstSeedName, dstIngress, namespace, v1beta1constants.ETCDRoleMain, 1),
					LiveMigrationEtcdClientHost(seedName, namespace, ingress, v1beta1constants.ETCDRoleMain),
				)))
				Expect(values.LiveMigration.ExtraPeerServiceDNSNames).To(Equal(CrossSeedPeerHostnames(
					seedName, ingress, dstSeedName, dstIngress, namespace, v1beta1constants.ETCDRoleMain, 1,
				)))
				Expect(values.LiveMigration.BootstrapWithExistingCluster).To(BeNil())
			})

			It("should return an error when the destination seed cannot be fetched", func() {
				s := runtime.NewScheme()
				Expect(gardencorev1beta1.AddToScheme(s)).To(Succeed())
				b.GardenAPIReader = fakeclient.NewClientBuilder().WithScheme(s).Build()

				Expect(SetLiveMigrationEtcdValues(b, ctx, values, v1beta1constants.ETCDRoleMain)).
					To(MatchError(ContainSubstring("failed to get destination seed")))
			})
		})

		Context("destination role", func() {
			JustBeforeEach(func() {
				b.Seed.SetInfo(&gardencorev1beta1.Seed{
					ObjectMeta: metav1.ObjectMeta{Name: dstSeedName},
					Spec:       gardencorev1beta1.SeedSpec{Ingress: &gardencorev1beta1.Ingress{Domain: dstIngress}},
				})
				b.Shoot.SetInfo(liveMigratingShoot(seedName, dstSeedName))
			})

			It("should set BootstrapWithExistingCluster, ExtraPeerServiceDNSNames, AdditionalAdvertisedURLs, ExtraClientServiceDNSNames and SkipClientSANVerification", func() {
				Expect(SetLiveMigrationEtcdValues(b, ctx, values, v1beta1constants.ETCDRoleMain)).To(Succeed())

				Expect(values.LiveMigration).NotTo(BeNil())
				Expect(values.LiveMigration.SkipClientSANVerification).To(BeTrue())
				Expect(values.LiveMigration.AdditionalAdvertisedURLs).To(Equal(&druidcorev1alpha1.AdditionalAdvertiseURLsSpec{
					PeerURLs: &druidcorev1alpha1.AdditionalURLsSpec{
						OverrideDefaultURL: new(true),
						Members:            ComputeMemberPeerURLs(dstSeedName, namespace, dstIngress, v1beta1constants.ETCDRoleMain, 1),
					},
					ClientURLs: &druidcorev1alpha1.AdditionalURLsSpec{
						OverrideDefaultURL: new(true),
						Members:            ComputeMemberClientURLs(dstSeedName, namespace, dstIngress, v1beta1constants.ETCDRoleMain, 1),
					},
				}))
				Expect(values.LiveMigration.ExtraPeerServiceDNSNames).To(Equal(CrossSeedPeerHostnames(
					seedName, ingress, dstSeedName, dstIngress, namespace, v1beta1constants.ETCDRoleMain, 1,
				)))
				Expect(values.LiveMigration.ExtraClientServiceDNSNames).To(ConsistOf(
					CrossSeedClientMemberHosts(seedName, ingress, dstSeedName, dstIngress, namespace, v1beta1constants.ETCDRoleMain, 1),
				))
				Expect(values.LiveMigration.BootstrapWithExistingCluster).NotTo(BeNil())
				Expect(values.LiveMigration.BootstrapWithExistingCluster.Members).To(HaveLen(1))
				Expect(values.LiveMigration.BootstrapWithExistingCluster.Members[0].Name).To(Equal("src-seed-etcd-main-0"))
				Expect(values.LiveMigration.BootstrapWithExistingCluster.ClientEndpoints).To(ConsistOf(
					ContainSubstring(LiveMigrationEtcdClientHost(seedName, namespace, ingress, v1beta1constants.ETCDRoleMain)),
				))
			})

			It("should return an error when the source seed cannot be fetched", func() {
				s := runtime.NewScheme()
				Expect(gardencorev1beta1.AddToScheme(s)).To(Succeed())
				b.GardenAPIReader = fakeclient.NewClientBuilder().WithScheme(s).Build()

				Expect(SetLiveMigrationEtcdValues(b, ctx, values, v1beta1constants.ETCDRoleMain)).
					To(MatchError(ContainSubstring("failed to get source seed")))
			})
		})
	})

	Describe("#LiveMigrationEtcdPeerHost", func() {
		It("should compose a stable cross-seed SNI host with a hashed left-most label", func() {
			Expect(LiveMigrationEtcdPeerHost("src-seed", "shoot--p1--foo", "ingress.seed.example.com", "main", 0)).To(Equal("etcd-main-peer-0-9bd85b.ingress.seed.example.com"))
			Expect(LiveMigrationEtcdPeerHost("src-seed", "shoot--p1--foo", "ingress.seed.example.com", "events", 2)).To(Equal("etcd-events-peer-2-3b8ac1.ingress.seed.example.com"))
		})

		It("should be deterministic across calls (so source and destination seeds agree)", func() {
			Expect(LiveMigrationEtcdPeerHost("src-seed", "shoot--p1--foo", "ingress.seed.example.com", "main", 0)).
				To(Equal(LiveMigrationEtcdPeerHost("src-seed", "shoot--p1--foo", "ingress.seed.example.com", "main", 0)))
		})

		It("should produce distinct labels per ordinal, role, seed and namespace", func() {
			label := func(host string) string { l, _, _ := strings.Cut(host, "."); return l }
			hosts := []string{
				LiveMigrationEtcdPeerHost("src-seed", "shoot--p1--foo", "d", "main", 0),
				LiveMigrationEtcdPeerHost("src-seed", "shoot--p1--foo", "d", "main", 1),
				LiveMigrationEtcdPeerHost("src-seed", "shoot--p1--foo", "d", "events", 0),
				LiveMigrationEtcdPeerHost("dst-seed", "shoot--p1--foo", "d", "main", 0),
				LiveMigrationEtcdPeerHost("src-seed", "shoot--p1--bar", "d", "main", 0),
			}
			seen := map[string]bool{}
			for _, h := range hosts {
				Expect(seen[label(h)]).To(BeFalse(), "label %q must be unique", label(h))
				seen[label(h)] = true
			}
		})

		It("should keep the left-most DNS label within 63 characters even for maximal-length inputs", func() {
			// Seed names are only bounded by the DNS label limit (63); shoot namespaces can be up to 30 chars.
			longSeed := strings.Repeat("s", 63)
			longNamespace := "shoot--" + strings.Repeat("p", 10) + "--" + strings.Repeat("f", 11)
			host := LiveMigrationEtcdPeerHost(longSeed, longNamespace, "ingress.seed.example.com", "events", 9)
			label, _, _ := strings.Cut(host, ".")
			Expect(len(label)).To(BeNumerically("<=", 63))
		})
	})

	Describe("#LiveMigrationEtcdClientHost", func() {
		It("should compose a stable cross-seed client SNI host with a hashed left-most label", func() {
			Expect(LiveMigrationEtcdClientHost("src-seed", "shoot--p1--foo", "ingress.seed.example.com", "main")).To(Equal("etcd-main-client-44d4fb.ingress.seed.example.com"))
		})

		It("should keep the left-most DNS label within 63 characters even for maximal-length inputs", func() {
			longSeed := strings.Repeat("s", 63)
			longNamespace := "shoot--" + strings.Repeat("p", 10) + "--" + strings.Repeat("f", 11)
			host := LiveMigrationEtcdClientHost(longSeed, longNamespace, "ingress.seed.example.com", "events")
			label, _, _ := strings.Cut(host, ".")
			Expect(len(label)).To(BeNumerically("<=", 63))
		})
	})

	Describe("#LiveMigrationEtcdClientMemberHost", func() {
		It("should compose a stable per-ordinal cross-seed client SNI host with ordinal and hash in the left-most label", func() {
			Expect(LiveMigrationEtcdClientMemberHost("src-seed", "shoot--p1--foo", "ingress.seed.example.com", "main", 0)).To(Equal("etcd-main-client-0-9bd85b.ingress.seed.example.com"))
			Expect(LiveMigrationEtcdClientMemberHost("src-seed", "shoot--p1--foo", "ingress.seed.example.com", "main", 1)).To(Equal("etcd-main-client-1-18879a.ingress.seed.example.com"))
		})

		It("should produce distinct labels per ordinal, role, seed and namespace", func() {
			label := func(host string) string { l, _, _ := strings.Cut(host, "."); return l }
			hosts := []string{
				LiveMigrationEtcdClientMemberHost("src-seed", "shoot--p1--foo", "d", "main", 0),
				LiveMigrationEtcdClientMemberHost("src-seed", "shoot--p1--foo", "d", "main", 1),
				LiveMigrationEtcdClientMemberHost("src-seed", "shoot--p1--foo", "d", "events", 0),
				LiveMigrationEtcdClientMemberHost("dst-seed", "shoot--p1--foo", "d", "main", 0),
				LiveMigrationEtcdClientMemberHost("src-seed", "shoot--p1--bar", "d", "main", 0),
			}
			seen := map[string]bool{}
			for _, h := range hosts {
				Expect(seen[label(h)]).To(BeFalse(), "label %q must be unique", label(h))
				seen[label(h)] = true
			}
		})

		It("should keep the left-most DNS label within 63 characters even for maximal-length inputs", func() {
			longSeed := strings.Repeat("s", 63)
			longNamespace := "shoot--" + strings.Repeat("p", 10) + "--" + strings.Repeat("f", 11)
			host := LiveMigrationEtcdClientMemberHost(longSeed, longNamespace, "ingress.seed.example.com", "events", 9)
			label, _, _ := strings.Cut(host, ".")
			Expect(len(label)).To(BeNumerically("<=", 63))
		})
	})

	Describe("#ComputeMemberPeerURLs", func() {
		It("should compute one entry per member with distinct peer port URLs", func() {
			Expect(ComputeMemberPeerURLs("src-seed", "shoot--p1--foo", "ingress.seed.example.com", "main", 3)).To(Equal([]druidcorev1alpha1.MemberURLs{
				{Name: "src-seed-etcd-main-0", URLs: []string{fmt.Sprintf("https://etcd-main-peer-0-9bd85b.ingress.seed.example.com:%d", etcdconstants.PortEtcdPeerExternal)}},
				{Name: "src-seed-etcd-main-1", URLs: []string{fmt.Sprintf("https://etcd-main-peer-1-18879a.ingress.seed.example.com:%d", etcdconstants.PortEtcdPeerExternal+1)}},
				{Name: "src-seed-etcd-main-2", URLs: []string{fmt.Sprintf("https://etcd-main-peer-2-26b70e.ingress.seed.example.com:%d", etcdconstants.PortEtcdPeerExternal+2)}},
			}))
		})

		It("should return an empty slice for zero replicas", func() {
			Expect(ComputeMemberPeerURLs("src-seed", "shoot--p1--foo", "ingress.seed.example.com", "main", 0)).To(BeEmpty())
		})
	})

	Describe("#ComputeMemberClientURLs", func() {
		It("should compute one entry per member each with a distinct per-ordinal client SNI host", func() {
			Expect(ComputeMemberClientURLs("src-seed", "shoot--p1--foo", "ingress.seed.example.com", "main", 3)).To(Equal([]druidcorev1alpha1.MemberURLs{
				{Name: "src-seed-etcd-main-0", URLs: []string{"https://etcd-main-client-0-9bd85b.ingress.seed.example.com:443"}},
				{Name: "src-seed-etcd-main-1", URLs: []string{"https://etcd-main-client-1-18879a.ingress.seed.example.com:443"}},
				{Name: "src-seed-etcd-main-2", URLs: []string{"https://etcd-main-client-2-26b70e.ingress.seed.example.com:443"}},
			}))
		})

		It("should return an empty slice for zero replicas", func() {
			Expect(ComputeMemberClientURLs("src-seed", "shoot--p1--foo", "ingress.seed.example.com", "main", 0)).To(BeEmpty())
		})
	})

	Describe("#CrossSeedPeerHostnames", func() {
		It("should list source hostnames first then destination hostnames", func() {
			Expect(CrossSeedPeerHostnames(
				"src-seed", "ingress.src.example.com",
				"dst-seed", "ingress.dst.example.com",
				"shoot--p1--foo", "main", 2,
			)).To(Equal([]string{
				"etcd-main-peer-0-9bd85b.ingress.src.example.com",
				"etcd-main-peer-1-18879a.ingress.src.example.com",
				"etcd-main-peer-0-7ba53a.ingress.dst.example.com",
				"etcd-main-peer-1-20a72c.ingress.dst.example.com",
			}))
		})

		It("should return an empty slice for zero replicas", func() {
			Expect(CrossSeedPeerHostnames("src-seed", "ingress.src.example.com", "dst-seed", "ingress.dst.example.com", "shoot--p1--foo", "main", 0)).To(BeEmpty())
		})
	})

	Describe("#CrossSeedClientMemberHosts", func() {
		It("should list source per-ordinal hostnames first then destination per-ordinal hostnames", func() {
			Expect(CrossSeedClientMemberHosts(
				"src-seed", "ingress.src.example.com",
				"dst-seed", "ingress.dst.example.com",
				"shoot--p1--foo", "main", 2,
			)).To(Equal([]string{
				"etcd-main-client-0-9bd85b.ingress.src.example.com",
				"etcd-main-client-1-18879a.ingress.src.example.com",
				"etcd-main-client-0-7ba53a.ingress.dst.example.com",
				"etcd-main-client-1-20a72c.ingress.dst.example.com",
			}))
		})

		It("should return an empty slice for zero replicas", func() {
			Expect(CrossSeedClientMemberHosts("src-seed", "ingress.src.example.com", "dst-seed", "ingress.dst.example.com", "shoot--p1--foo", "main", 0)).To(BeEmpty())
		})
	})

	Describe("EtcdMigrationComplete phase helpers", func() {
		var (
			ctrl      *gomock.Controller
			fakeClock *testclock.FakeClock

			etcdMain   *mocketcd.MockInterface
			etcdEvents *mocketcd.MockInterface
			b          *Botanist
		)

		baseEtcdCR := func() *druidcorev1alpha1.Etcd {
			return &druidcorev1alpha1.Etcd{
				ObjectMeta: metav1.ObjectMeta{
					Name:      etcd.Name(v1beta1constants.ETCDRoleMain),
					Namespace: namespace,
				},
				Spec: druidcorev1alpha1.EtcdSpec{
					Replicas: 3,
					Etcd: druidcorev1alpha1.EtcdConfig{
						BootstrapWithExistingCluster: &druidcorev1alpha1.BootstrapWithExistingCluster{
							Members: []druidcorev1alpha1.BootstrapExistingMember{
								{Name: "src-seed-etcd-main-0"},
							},
						},
					},
				},
				Status: druidcorev1alpha1.EtcdStatus{
					Conditions: []druidcorev1alpha1.Condition{
						{
							Type:   druidcorev1alpha1.ConditionTypeScaleOperationComplete,
							Status: druidcorev1alpha1.ConditionTrue,
						},
						{
							Type:   druidcorev1alpha1.ConditionTypeBootstrappedWithExistingCluster,
							Status: druidcorev1alpha1.ConditionTrue,
						},
					},
				},
			}
		}

		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())
			fakeClock = testclock.NewFakeClock(time.Now())

			etcdMain = mocketcd.NewMockInterface(ctrl)
			etcdEvents = mocketcd.NewMockInterface(ctrl)

			s := runtime.NewScheme()
			Expect(druidcorev1alpha1.AddToScheme(s)).To(Succeed())
			fakeClient := fakeclient.NewClientBuilder().WithScheme(s).Build()
			kubernetesClient := fakekubernetes.NewClientSetBuilder().WithClient(fakeClient).Build()

			b = &Botanist{Operation: &operation.Operation{
				SeedClientSet: kubernetesClient,
				Clock:         fakeClock,
			}}
			b.Seed = &seedpkg.Seed{}
			b.Seed.SetInfo(&gardencorev1beta1.Seed{
				ObjectMeta: metav1.ObjectMeta{Name: dstSeedName},
				Spec: gardencorev1beta1.SeedSpec{
					Ingress: &gardencorev1beta1.Ingress{Domain: dstIngress},
				},
			})
			b.Shoot = &shootpkg.Shoot{ControlPlaneNamespace: namespace}
			b.Shoot.SetInfo(&gardencorev1beta1.Shoot{})
			b.Shoot.Components = &shootpkg.Components{
				ControlPlane: &shootpkg.ControlPlane{
					EtcdMain:   etcdMain,
					EtcdEvents: etcdEvents,
				},
			}
		})

		AfterEach(func() {
			ctrl.Finish()
		})

		Describe("#DeployEtcdForLiveMigrationPhase", func() {
			var (
				capturedLM  *etcd.LiveMigrationValues
				setLMCalled int
				originalLM  *etcd.LiveMigrationValues
			)

			BeforeEach(func() {
				capturedLM = nil
				setLMCalled = 0
				originalLM = &etcd.LiveMigrationValues{
					SkipClientSANVerification:    true,
					ExtraPeerServiceDNSNames:     []string{"peer-san"},
					ExtraClientServiceDNSNames:   []string{"client-san"},
					BootstrapWithExistingCluster: &druidcorev1alpha1.BootstrapWithExistingCluster{Members: []druidcorev1alpha1.BootstrapExistingMember{{Name: "src-seed-etcd-main-0"}}},
					AdditionalAdvertisedURLs:     &druidcorev1alpha1.AdditionalAdvertiseURLsSpec{},
				}
				etcdMain.EXPECT().GetValues().Return(etcd.Values{Replicas: new(int32(3)), LiveMigration: originalLM}).AnyTimes()
				etcdMain.EXPECT().GetReplicas().Return(new(int32(3))).AnyTimes()
				etcdMain.EXPECT().SetReplicas(gomock.Any()).AnyTimes()
				etcdMain.EXPECT().SetLiveMigration(gomock.Any()).DoAndReturn(func(v *etcd.LiveMigrationValues) {
					setLMCalled++
					if setLMCalled == 1 {
						capturedLM = v
					}
				}).AnyTimes()
				etcdMain.EXPECT().UnmarkAsPeerCARolloutCompleted(ctx).AnyTimes()
				etcdEvents.EXPECT().UnmarkAsPeerCARolloutCompleted(ctx).AnyTimes()
				etcdMain.EXPECT().Deploy(ctx).Return(nil).AnyTimes()
				etcdEvents.EXPECT().Deploy(ctx).Return(nil).AnyTimes()
			})

			It("should return an error when LiveMigration values are nil", func() {
				etcdMain.EXPECT().GetValues().Return(etcd.Values{LiveMigration: nil})

				Expect(b.DeployEtcdForLiveMigrationPhase(ctx, 2)).To(MatchError(ContainSubstring("LiveMigration")))
			})

			It("should restore original replicas and LiveMigration values after deploy (defer)", func() {
				var lastSetReplicas *int32
				etcdMain.EXPECT().SetReplicas(gomock.Any()).DoAndReturn(func(v *int32) {
					lastSetReplicas = v
				}).AnyTimes()

				Expect(b.DeployEtcdForLiveMigrationPhase(ctx, 2)).To(Succeed())
				// defer must have restored original values
				Expect(setLMCalled).To(Equal(2)) // call #1: mutated phase values; call #2: defer restore
				Expect(capturedLM).NotTo(Equal(originalLM)) // call #1 was the mutated phase value
				Expect(lastSetReplicas).NotTo(BeNil())
				Expect(*lastSetReplicas).To(Equal(int32(3))) // restored to original 3 replicas
			})

			DescribeTable("advertised member count always equals replica count",
				func(replicas int32) {
					Expect(b.DeployEtcdForLiveMigrationPhase(ctx, replicas)).To(Succeed())
					Expect(capturedLM).NotTo(BeNil())
					Expect(capturedLM.AdditionalAdvertisedURLs.PeerURLs.Members).To(HaveLen(int(replicas)))
					Expect(capturedLM.AdditionalAdvertisedURLs.ClientURLs.Members).To(HaveLen(int(replicas)))
					Expect(capturedLM.SkipClientSANVerification).To(Equal(originalLM.SkipClientSANVerification))
					Expect(capturedLM.BootstrapWithExistingCluster).To(Equal(originalLM.BootstrapWithExistingCluster))
					Expect(capturedLM.ExtraPeerServiceDNSNames).To(Equal(originalLM.ExtraPeerServiceDNSNames))
					Expect(capturedLM.ExtraClientServiceDNSNames).To(Equal(originalLM.ExtraClientServiceDNSNames))
				},
				Entry("replicas=1", int32(1)),
				Entry("replicas=2", int32(2)),
				Entry("replicas=3", int32(3)),
			)
		})

		Describe("#RemoveSourceEtcdMembers", func() {
			It("should return an error (no patch) when BootstrappedWithExistingCluster condition is not True", func() {
				cr := baseEtcdCR()
				for i := range cr.Status.Conditions {
					if cr.Status.Conditions[i].Type == druidcorev1alpha1.ConditionTypeBootstrappedWithExistingCluster {
						cr.Status.Conditions[i].Status = druidcorev1alpha1.ConditionFalse
					}
				}
				etcdMain.EXPECT().Get(ctx).Return(cr, nil)

				Expect(b.SeedClientSet.Client().Create(ctx, cr)).To(Succeed())

				Expect(b.RemoveSourceEtcdMembers(ctx)).To(MatchError(ContainSubstring("bootstrapped")))

				updated := &druidcorev1alpha1.Etcd{}
				Expect(b.SeedClientSet.Client().Get(ctx, client.ObjectKeyFromObject(cr), updated)).To(Succeed())
				Expect(updated.Spec.Etcd.BootstrapWithExistingCluster).NotTo(BeNil())
			})

			It("should return an error (no patch) when BootstrappedWithExistingCluster condition is absent", func() {
				cr := baseEtcdCR()
				cr.Status.Conditions = []druidcorev1alpha1.Condition{
					{
						Type:   druidcorev1alpha1.ConditionTypeScaleOperationComplete,
						Status: druidcorev1alpha1.ConditionTrue,
					},
				}
				etcdMain.EXPECT().Get(ctx).Return(cr, nil)

				Expect(b.SeedClientSet.Client().Create(ctx, cr)).To(Succeed())

				Expect(b.RemoveSourceEtcdMembers(ctx)).To(MatchError(ContainSubstring("bootstrapped")))

				updated := &druidcorev1alpha1.Etcd{}
				Expect(b.SeedClientSet.Client().Get(ctx, client.ObjectKeyFromObject(cr), updated)).To(Succeed())
				Expect(updated.Spec.Etcd.BootstrapWithExistingCluster).NotTo(BeNil())
			})

			It("should clear BootstrapWithExistingCluster and set operation+timestamp in one patch", func() {
				cr := baseEtcdCR()
				etcdMain.EXPECT().Get(ctx).Return(cr, nil)
				Expect(b.SeedClientSet.Client().Create(ctx, cr)).To(Succeed())

				Expect(b.RemoveSourceEtcdMembers(ctx)).To(Succeed())

				updated := &druidcorev1alpha1.Etcd{}
				Expect(b.SeedClientSet.Client().Get(ctx, client.ObjectKeyFromObject(cr), updated)).To(Succeed())
				Expect(updated.Spec.Etcd.BootstrapWithExistingCluster).To(BeNil())
				Expect(updated.Annotations).To(HaveKey(v1beta1constants.GardenerOperation))
				Expect(updated.Annotations[v1beta1constants.GardenerOperation]).To(Equal(v1beta1constants.GardenerOperationReconcile))
				Expect(updated.Annotations).To(HaveKey(v1beta1constants.GardenerTimestamp))
			})

			It("should return nil with no patch when BootstrapWithExistingCluster is already nil", func() {
				cr := baseEtcdCR()
				cr.Spec.Etcd.BootstrapWithExistingCluster = nil
				etcdMain.EXPECT().Get(ctx).Return(cr, nil)

				Expect(b.RemoveSourceEtcdMembers(ctx)).To(Succeed())
			})

			It("should return a retriable error when ScaleOperationComplete is False (GUARD A)", func() {
				cr := baseEtcdCR()
				for i := range cr.Status.Conditions {
					if cr.Status.Conditions[i].Type == druidcorev1alpha1.ConditionTypeScaleOperationComplete {
						cr.Status.Conditions[i].Status = druidcorev1alpha1.ConditionFalse
						cr.Status.Conditions[i].Reason = druidcorev1alpha1.ScaleOperationReasonScalingOut
					}
				}
				etcdMain.EXPECT().Get(ctx).Return(cr, nil)
				Expect(b.SeedClientSet.Client().Create(ctx, cr)).To(Succeed())

				Expect(b.RemoveSourceEtcdMembers(ctx)).To(MatchError(ContainSubstring("scale-out")))

				updated := &druidcorev1alpha1.Etcd{}
				Expect(b.SeedClientSet.Client().Get(ctx, client.ObjectKeyFromObject(cr), updated)).To(Succeed())
				Expect(updated.Spec.Etcd.BootstrapWithExistingCluster).NotTo(BeNil())
			})

			It("should not panic and should set annotations when the Etcd CR has no annotations map", func() {
				cr := baseEtcdCR()
				cr.Annotations = nil
				etcdMain.EXPECT().Get(ctx).Return(cr, nil)
				Expect(b.SeedClientSet.Client().Create(ctx, cr)).To(Succeed())

				Expect(b.RemoveSourceEtcdMembers(ctx)).To(Succeed())

				updated := &druidcorev1alpha1.Etcd{}
				Expect(b.SeedClientSet.Client().Get(ctx, client.ObjectKeyFromObject(cr), updated)).To(Succeed())
				Expect(updated.Annotations).To(HaveKey(v1beta1constants.GardenerOperation))
			})
		})

		Describe("#WaitUntilSourceEtcdMembersRemoved", func() {
			It("should return nil when scale operation is complete and no bootstrap members remain", func() {
				cr := baseEtcdCR()
				cr.Spec.Etcd.BootstrapWithExistingCluster = nil
				etcdMain.EXPECT().Get(gomock.Any()).Return(cr, nil)

				Expect(b.WaitUntilSourceEtcdMembersRemoved(ctx)).To(Succeed())
			})

			It("should return an error when EtcdMain.Get fails and context times out", func() {
				etcdMain.EXPECT().Get(gomock.Any()).Return(nil, errors.New("get-failed")).AnyTimes()

				cancelCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
				defer cancel()

				err := b.WaitUntilSourceEtcdMembersRemoved(cancelCtx)
				Expect(err).To(MatchError(ContainSubstring("get-failed")))
			})
		})

		Describe("live migration cleanup", func() {
			It("should call SetLiveMigration(nil) then Deploy when clearing live migration config", func() {
				nilLMCalled := false
				etcdMain.EXPECT().SetLiveMigration((*etcd.LiveMigrationValues)(nil)).DoAndReturn(func(_ *etcd.LiveMigrationValues) {
					nilLMCalled = true
				})
				etcdMain.EXPECT().UnmarkAsPeerCARolloutCompleted(gomock.Any()).Return(nil)
				etcdMain.EXPECT().Deploy(gomock.Any()).DoAndReturn(func(_ context.Context) error {
					Expect(nilLMCalled).To(BeTrue(), "SetLiveMigration(nil) must be called before Deploy")
					return nil
				})
				etcdEvents.EXPECT().UnmarkAsPeerCARolloutCompleted(gomock.Any()).Return(nil)
				etcdEvents.EXPECT().Deploy(gomock.Any()).Return(nil)

				cleanFn := func(ctx context.Context) error {
					b.Shoot.Components.ControlPlane.EtcdMain.SetLiveMigration(nil)
					return b.DeployEtcd(ctx)
				}
				Expect(cleanFn(ctx)).To(Succeed())
			})
		})
	})
})
