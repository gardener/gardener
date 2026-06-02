// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/apis/config/gardenlet/v1alpha1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	. "github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
	seedpkg "github.com/gardener/gardener/pkg/gardenlet/operation/seed"
	shootpkg "github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
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
							Namespace:   ptr.To(v1beta1constants.DefaultSNIIngressNamespace),
							ServiceName: ptr.To(v1beta1constants.DefaultSNIIngressServiceName),
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
	})

	Describe("#setAPIServerServiceClusterIPs", func() {
		BeforeEach(func() {
			botanist.Shoot.InternalClusterDomain = ptr.To("internal.foo.bar")

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
			botanist.Shoot.ExternalClusterDomain = ptr.To("external.foo.bar")

			Expect(botanist.DefaultKubeAPIServerService().Deploy(context.TODO())).To(Succeed())

			Expect(botanist.Shoot.Components.ControlPlane.KubeAPIServerSNI).NotTo(BeNil())

			// We call Deploy to trigger the valuesFunc and ensure it doesn't panic.
			Expect(botanist.Shoot.Components.ControlPlane.KubeAPIServerSNI.Deploy(context.TODO())).To(Succeed())
		})
	})
})
