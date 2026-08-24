// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist_test

import (
	"context"
	"errors"

	druidcorev1alpha1 "github.com/gardener/etcd-druid/api/core/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/apis/config/gardenlet/v1alpha1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/etcd/etcd"
	etcdconstants "github.com/gardener/gardener/pkg/component/etcd/etcd/constants"
	"github.com/gardener/gardener/pkg/component/etcd/peerexposure"
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

		peerExposureComponent *mockcomponent.MockDeployWaiter

		actualNamespace string
		actualValues    peerexposure.Values

		b *Botanist
	)

	Describe("#DeployEtcdPeerExposure", func() {
		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())
			peerExposureComponent = mockcomponent.NewMockDeployWaiter(ctrl)

			actualNamespace = ""
			actualValues = peerexposure.Values{}
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
		})

		AfterEach(func() {
			ctrl.Finish()
		})

		It("should deploy with correct namespace, members, client host and Istio config", func() {
			peerExposureComponent.EXPECT().Deploy(ctx)

			DeferCleanup(test.WithVar(&NewPeerExposure, func(_ client.Client, ns string, vals peerexposure.Values) component.DeployWaiter {
				actualNamespace = ns
				actualValues = vals
				return peerExposureComponent
			}))

			Expect(b.DeployEtcdPeerExposure(ctx)).To(Succeed())

			Expect(actualNamespace).To(Equal(namespace))
			Expect(actualValues.Role).To(Equal(v1beta1constants.ETCDRoleMain))
			Expect(actualValues.Members).To(HaveLen(1))
			Expect(actualValues.Members[0].SNIHost).To(Equal("src-seed-etcd-main-0-shoot--p1--foo.ingress.seed.example.com"))
			Expect(actualValues.Members[0].PodFQDN).To(Equal("etcd-main-0.etcd-main-peer.shoot--p1--foo.svc.cluster.local"))
			Expect(actualValues.Members[0].ExternalPort).To(Equal(uint32(etcdconstants.PortEtcdPeerExternal)))
			Expect(actualValues.ClientHost).To(Equal("src-seed-shoot--p1--foo-etcd-main-client.ingress.seed.example.com"))
			Expect(actualValues.IstioIngressGatewayNamespace).To(Equal("istio-ingress"))
			Expect(actualValues.IstioIngressGatewayLabels).To(HaveKeyWithValue("istio", "ingressgateway"))
		})

		It("should return an error when Deploy fails", func() {
			peerExposureComponent.EXPECT().Deploy(ctx).Return(testErr)

			DeferCleanup(test.WithVar(&NewPeerExposure, func(_ client.Client, _ string, _ peerexposure.Values) component.DeployWaiter {
				return peerExposureComponent
			}))

			Expect(b.DeployEtcdPeerExposure(ctx)).To(MatchError(ContainSubstring(testErr.Error())))
		})

		It("should compute 3 members for an HA shoot", func() {
			peerExposureComponent.EXPECT().Deploy(ctx)

			DeferCleanup(test.WithVar(&NewPeerExposure, func(_ client.Client, _ string, vals peerexposure.Values) component.DeployWaiter {
				actualValues = vals
				return peerExposureComponent
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

			Expect(b.DeployEtcdPeerExposure(ctx)).To(Succeed())

			Expect(actualValues.Members).To(HaveLen(int(etcdconstants.HAReplicaCount)))
			for i := 0; i < int(etcdconstants.HAReplicaCount); i++ {
				Expect(actualValues.Members[i].ExternalPort).To(Equal(uint32(etcdconstants.PortEtcdPeerExternal) + uint32(i)))
			}
		})
	})

	Describe("#DestroyEtcdPeerExposure", func() {
		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())
			peerExposureComponent = mockcomponent.NewMockDeployWaiter(ctrl)

			actualNamespace = ""
			actualValues = peerexposure.Values{}
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
		})

		AfterEach(func() {
			ctrl.Finish()
		})

		It("should destroy with correct namespace, members and Istio config (no ClientHost)", func() {
			peerExposureComponent.EXPECT().Destroy(ctx)

			DeferCleanup(test.WithVar(&NewPeerExposure, func(_ client.Client, ns string, vals peerexposure.Values) component.DeployWaiter {
				actualNamespace = ns
				actualValues = vals
				return peerExposureComponent
			}))

			Expect(b.DestroyEtcdPeerExposure(ctx)).To(Succeed())

			Expect(actualNamespace).To(Equal(namespace))
			Expect(actualValues.Role).To(Equal(v1beta1constants.ETCDRoleMain))
			Expect(actualValues.Members).To(HaveLen(1))
			Expect(actualValues.ClientHost).To(BeEmpty())
			Expect(actualValues.IstioIngressGatewayNamespace).To(Equal("istio-ingress"))
		})

		It("should return an error when Destroy fails", func() {
			peerExposureComponent.EXPECT().Destroy(ctx).Return(testErr)

			DeferCleanup(test.WithVar(&NewPeerExposure, func(_ client.Client, _ string, _ peerexposure.Values) component.DeployWaiter {
				return peerExposureComponent
			}))

			Expect(b.DestroyEtcdPeerExposure(ctx)).To(MatchError(ContainSubstring(testErr.Error())))
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

			It("should set AdditionalAdvertisePeerURLs, ExtraClientServiceDNSNames, ExtraPeerServiceDNSNames and SkipClientSANVerification", func() {
				Expect(SetLiveMigrationEtcdValues(b, ctx, values, v1beta1constants.ETCDRoleMain)).To(Succeed())

				Expect(values.SkipClientSANVerification).To(BeTrue())
				Expect(values.AdditionalAdvertisePeerURLs).To(Equal(ComputeMemberPeerURLs(seedName, namespace, ingress, v1beta1constants.ETCDRoleMain, 1)))
				Expect(values.ExtraClientServiceDNSNames).To(ConsistOf(
					LiveMigrationEtcdClientHost(seedName, namespace, ingress, v1beta1constants.ETCDRoleMain),
				))
				Expect(values.ExtraPeerServiceDNSNames).To(Equal(CrossSeedPeerHostnames(
					seedName, ingress, dstSeedName, dstIngress, namespace, v1beta1constants.ETCDRoleMain, 1,
				)))
				Expect(values.BootstrapWithExistingCluster).To(BeNil())
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

			It("should set BootstrapWithExistingCluster, ExtraPeerServiceDNSNames, AdditionalAdvertisePeerURLs and SkipClientSANVerification", func() {
				Expect(SetLiveMigrationEtcdValues(b, ctx, values, v1beta1constants.ETCDRoleMain)).To(Succeed())

				Expect(values.SkipClientSANVerification).To(BeTrue())
				Expect(values.AdditionalAdvertisePeerURLs).To(Equal(ComputeMemberPeerURLs(dstSeedName, namespace, dstIngress, v1beta1constants.ETCDRoleMain, 1)))
				Expect(values.ExtraPeerServiceDNSNames).To(Equal(CrossSeedPeerHostnames(
					seedName, ingress, dstSeedName, dstIngress, namespace, v1beta1constants.ETCDRoleMain, 1,
				)))
				Expect(values.ExtraClientServiceDNSNames).To(BeNil())
				Expect(values.BootstrapWithExistingCluster).NotTo(BeNil())
				Expect(values.BootstrapWithExistingCluster.Members).To(HaveLen(1))
				Expect(values.BootstrapWithExistingCluster.Members[0].Name).To(Equal("src-seed-etcd-main-0"))
				Expect(values.BootstrapWithExistingCluster.ClientEndpoints).To(ConsistOf(
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
		It("should compose the cross-seed SNI host from seed name, etcd name, ordinal, shoot namespace and ingress domain", func() {
			Expect(LiveMigrationEtcdPeerHost("src-seed", "shoot--p1--foo", "ingress.seed.example.com", "main", 0)).To(Equal("src-seed-etcd-main-0-shoot--p1--foo.ingress.seed.example.com"))
			Expect(LiveMigrationEtcdPeerHost("src-seed", "shoot--p1--foo", "ingress.seed.example.com", "events", 2)).To(Equal("src-seed-etcd-events-2-shoot--p1--foo.ingress.seed.example.com"))
		})
	})

	Describe("#LiveMigrationEtcdClientHost", func() {
		It("should compose the cross-seed etcd client SNI host from seed name, shoot namespace, ingress domain and role", func() {
			Expect(LiveMigrationEtcdClientHost("src-seed", "shoot--p1--foo", "ingress.seed.example.com", "main")).
				To(Equal("src-seed-shoot--p1--foo-etcd-main-client.ingress.seed.example.com"))
		})
	})

	Describe("#ComputeMemberPeerURLs", func() {
		It("should compute one entry per member with distinct peer port URLs", func() {
			Expect(ComputeMemberPeerURLs("src-seed", "shoot--p1--foo", "ingress.seed.example.com", "main", 3)).To(Equal([]druidcorev1alpha1.MemberPeerURLs{
				{MemberName: "src-seed-etcd-main-0", URLs: []string{"https://src-seed-etcd-main-0-shoot--p1--foo.ingress.seed.example.com:2380"}},
				{MemberName: "src-seed-etcd-main-1", URLs: []string{"https://src-seed-etcd-main-1-shoot--p1--foo.ingress.seed.example.com:2381"}},
				{MemberName: "src-seed-etcd-main-2", URLs: []string{"https://src-seed-etcd-main-2-shoot--p1--foo.ingress.seed.example.com:2382"}},
			}))
		})

		It("should return an empty slice for zero replicas", func() {
			Expect(ComputeMemberPeerURLs("src-seed", "shoot--p1--foo", "ingress.seed.example.com", "main", 0)).To(BeEmpty())
		})
	})

	Describe("#CrossSeedPeerHostnames", func() {
		It("should list source hostnames first then destination hostnames", func() {
			Expect(CrossSeedPeerHostnames(
				"src-seed", "ingress.src.example.com",
				"dst-seed", "ingress.dst.example.com",
				"shoot--p1--foo", "main", 2,
			)).To(Equal([]string{
				"src-seed-etcd-main-0-shoot--p1--foo.ingress.src.example.com",
				"src-seed-etcd-main-1-shoot--p1--foo.ingress.src.example.com",
				"dst-seed-etcd-main-0-shoot--p1--foo.ingress.dst.example.com",
				"dst-seed-etcd-main-1-shoot--p1--foo.ingress.dst.example.com",
			}))
		})

		It("should return an empty slice for zero replicas", func() {
			Expect(CrossSeedPeerHostnames("src-seed", "ingress.src.example.com", "dst-seed", "ingress.dst.example.com", "shoot--p1--foo", "main", 0)).To(BeEmpty())
		})
	})
})
