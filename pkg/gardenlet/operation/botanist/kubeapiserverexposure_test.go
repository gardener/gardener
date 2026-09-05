// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package botanist_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/apis/config/gardenlet/v1alpha1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/component/kubernetes/apiserver"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	. "github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
	gardenpkg "github.com/gardener/gardener/pkg/gardenlet/operation/garden"
	seedpkg "github.com/gardener/gardener/pkg/gardenlet/operation/seed"
	shootpkg "github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	fakesecretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager/fake"
)

var _ = Describe("KubeAPIServerExposure", func() {
	var (
		botanist *Botanist
	)

	BeforeEach(func() {
		botanist = &Botanist{
			Operation: &operation.Operation{
				Config: &gardenletconfigv1alpha1.GardenletConfiguration{
					SNI: &gardenletconfigv1alpha1.SNI{
						Ingress: &gardenletconfigv1alpha1.SNIIngress{
							Namespace:   new(v1beta1constants.DefaultSNIIngressNamespace),
							ServiceName: new(v1beta1constants.DefaultSNIIngressServiceName),
							Labels: map[string]string{
								v1beta1constants.LabelApp: v1beta1constants.DefaultIngressGatewayAppLabelValue,
								"istio":                   "ingressgateway",
							},
						},
					},
				},
				Shoot: &shootpkg.Shoot{
					ControlPlaneNamespace: "shoot--foo--bar",
					Components: &shootpkg.Components{
						ControlPlane: &shootpkg.ControlPlane{},
					},
				},
			},
		}
		botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
			Spec: gardencorev1beta1.ShootSpec{
				Networking: &gardencorev1beta1.Networking{
					IPFamilies: []gardencorev1beta1.IPFamily{gardencorev1beta1.IPFamilyIPv4},
				},
			},
		})
		botanist.Seed = &seedpkg.Seed{}
		botanist.Seed.SetInfo(&gardencorev1beta1.Seed{})

		botanist.SeedClientSet = fake.NewClientSetBuilder().WithClient(fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()).Build()
		Expect(botanist.SeedClientSet.Client().Create(context.TODO(), &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: botanist.Shoot.ControlPlaneNamespace}})).To(Succeed())

		botanist.SecretsManager = fakesecretsmanager.New(botanist.SeedClientSet.Client(), botanist.Shoot.ControlPlaneNamespace)
	})

	Describe("#setAPIServerServiceClusterIPs", func() {
		BeforeEach(func() {
			botanist.Shoot.InternalClusterDomain = new("internal.foo.bar")

			By("Create secrets managed outside of this function for which secretsmanager.Get() will be called")
			for _, name := range []string{
				v1beta1constants.SecretNameCACluster,
				v1beta1constants.SecretNameCAClient,
				apiserver.SecretNameServerCert + "-current",
			} {
				Expect(botanist.SeedClientSet.Client().Create(context.TODO(), &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: botanist.Shoot.ControlPlaneNamespace}})).To(Succeed())
			}

			Expect(botanist.SeedClientSet.Client().Create(context.TODO(), &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      v1beta1constants.DeploymentNameKubeAPIServer,
					Namespace: botanist.Shoot.ControlPlaneNamespace,
				},
				Spec: corev1.ServiceSpec{
					ClusterIPs: []string{"10.0.0.1"},
				},
			})).To(Succeed())

			Expect(botanist.SeedClientSet.Client().Create(context.TODO(), &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      v1beta1constants.DefaultSNIIngressServiceName,
					Namespace: v1beta1constants.DefaultSNIIngressNamespace,
				},
				Status: corev1.ServiceStatus{
					LoadBalancer: corev1.LoadBalancerStatus{
						Ingress: []corev1.LoadBalancerIngress{{IP: "1.2.3.4"}},
					},
				},
			})).To(Succeed())
		})

		It("should not panic when ExternalClusterDomain is nil", func() {
			botanist.Shoot.ExternalClusterDomain = nil

			Expect(botanist.DefaultKubeAPIServerService().Deploy(context.TODO())).To(Succeed())

			Expect(botanist.Shoot.Components.ControlPlane.KubeAPIServerSNI).NotTo(BeNil())

			// We call Deploy to trigger the valuesFunc and ensure it doesn't panic.
			Expect(botanist.Shoot.Components.ControlPlane.KubeAPIServerSNI.Deploy(context.TODO())).To(Succeed())
		})

		It("should not panic when ExternalClusterDomain is not nil", func() {
			botanist.Shoot.ExternalClusterDomain = new("external.foo.bar")

			Expect(botanist.DefaultKubeAPIServerService().Deploy(context.TODO())).To(Succeed())

			Expect(botanist.Shoot.Components.ControlPlane.KubeAPIServerSNI).NotTo(BeNil())

			// We call Deploy to trigger the valuesFunc and ensure it doesn't panic.
			Expect(botanist.Shoot.Components.ControlPlane.KubeAPIServerSNI.Deploy(context.TODO())).To(Succeed())
		})

		DescribeTable("should map the cluster IP of the kube-apiserver service",
			func(ipFamilies []gardencorev1beta1.IPFamily, clusterIPs []string, expectedClusterIP string) {
				botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
					Spec: gardencorev1beta1.ShootSpec{
						Networking: &gardencorev1beta1.Networking{
							IPFamilies: ipFamilies,
						},
					},
				})

				service := &corev1.Service{}
				Expect(botanist.SeedClientSet.Client().Get(context.TODO(), client.ObjectKey{
					Name:      v1beta1constants.DeploymentNameKubeAPIServer,
					Namespace: botanist.Shoot.ControlPlaneNamespace,
				}, service)).To(Succeed())
				service.Spec.ClusterIPs = clusterIPs
				Expect(botanist.SeedClientSet.Client().Update(context.TODO(), service)).To(Succeed())

				Expect(botanist.DefaultKubeAPIServerService().Deploy(context.TODO())).To(Succeed())

				Expect(botanist.APIServerClusterIP).To(Equal(expectedClusterIP))
			},

			// The real cluster IP must not leak into the shoot, so IPv4 addresses are mapped into
			// the reserved 240.0.0.0/8 range with the last three octets preserved.
			Entry("IPv4 single-stack", []gardencorev1beta1.IPFamily{gardencorev1beta1.IPFamilyIPv4},
				[]string{"10.0.0.1"}, "240.0.0.1"),
			Entry("IPv4 single-stack, all octets preserved", []gardencorev1beta1.IPFamily{gardencorev1beta1.IPFamilyIPv4},
				[]string{"192.168.102.23"}, "240.168.102.23"),
			Entry("IPv6-first shoot with IPv4 cluster IP is prefixed for address translation",
				[]gardencorev1beta1.IPFamily{gardencorev1beta1.IPFamilyIPv6, gardencorev1beta1.IPFamilyIPv4},
				[]string{"10.0.0.1"}, "64:ff9b:1::10.0.0.1"),
			Entry("IPv4 single-stack falls back to the second cluster IP if the first one is not IPv4",
				[]gardencorev1beta1.IPFamily{gardencorev1beta1.IPFamilyIPv4},
				[]string{"fd00::1", "10.0.0.1"}, "240.0.0.1"),
			Entry("IPv6 shoot with IPv6 cluster IP is used as is",
				[]gardencorev1beta1.IPFamily{gardencorev1beta1.IPFamilyIPv6},
				[]string{"fd00::1"}, "fd00::1"),
		)
	})

	Describe("#ShootUsesDNS", func() {
		DescribeTable("should return whether the shoot uses both internal and external DNS",
			func(needsInternalDNS, needsExternalDNS, expected bool) {
				botanist.Garden = nil
				botanist.Shoot.ExternalClusterDomain = nil
				botanist.Shoot.ExternalDomain = nil
				botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{})

				if needsInternalDNS {
					botanist.Garden = &gardenpkg.Garden{
						InternalDomain: &gardenerutils.Domain{Provider: "some-provider"},
					}
				}

				if needsExternalDNS {
					botanist.Shoot.ExternalClusterDomain = new("external.foo.bar")
					botanist.Shoot.ExternalDomain = &gardenerutils.Domain{Provider: "some-provider"}
					botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
						Spec: gardencorev1beta1.ShootSpec{
							DNS: &gardencorev1beta1.DNS{Domain: new("external.foo.bar")},
						},
					})
				}

				Expect(botanist.ShootUsesDNS()).To(Equal(expected))
			},

			Entry("internal and external DNS are needed", true, true, true),
			Entry("only internal DNS is needed", true, false, false),
			Entry("only external DNS is needed", false, true, false),
			Entry("neither internal nor external DNS is needed", false, false, false),
		)
	})
})
