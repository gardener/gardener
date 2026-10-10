// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package botanist_test

import (
	"context"

	"github.com/Masterminds/semver/v3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	. "github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
	shootpkg "github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
	operatorclient "github.com/gardener/gardener/pkg/operator/client"
)

var _ = Describe("Istio", func() {
	const shootName = "test-shoot"

	var (
		botanist *Botanist
		ctx      = context.Background()
	)

	newShoot := func(selfHosted bool, lb *gardencorev1beta1.ControlPlaneLoadBalancerServices) *gardencorev1beta1.Shoot {
		spec := gardencorev1beta1.ShootSpec{
			Kubernetes: gardencorev1beta1.Kubernetes{Version: "1.35.1"},
			Networking: &gardencorev1beta1.Networking{
				IPFamilies: []gardencorev1beta1.IPFamily{gardencorev1beta1.IPFamilyIPv4},
			},
		}
		if selfHosted {
			spec.Provider.Workers = []gardencorev1beta1.Worker{
				{
					Name:  "cp",
					Zones: []string{"zone-a"},
					ControlPlane: &gardencorev1beta1.WorkerControlPlane{
						LoadBalancerServices: lb,
					},
				},
			}
		}
		return &gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{Name: shootName},
			Spec:       spec,
		}
	}

	BeforeEach(func() {
		fakeClient := fakeclient.NewClientBuilder().WithScheme(operatorclient.RuntimeScheme).Build()

		botanist = &Botanist{Operation: &operation.Operation{}}
		botanist.SeedClientSet = fakekubernetes.NewClientSetBuilder().WithClient(fakeClient).Build()
		botanist.Shoot = &shootpkg.Shoot{KubernetesVersion: semver.MustParse("1.35.1")}
	})

	Describe("#DefaultIstio", func() {
		It("should return nil for a non-self-hosted shoot", func() {
			botanist.Shoot.SetInfo(newShoot(false, nil))

			deployer, err := botanist.DefaultIstio(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(deployer).To(BeNil())
		})

		It("should return a deployer with istiod enabled when the shoot is neither garden nor seed", func() {
			externalTrafficPolicy := corev1.ServiceExternalTrafficPolicyLocal
			lbClass := "my-lb-class"
			lb := &gardencorev1beta1.ControlPlaneLoadBalancerServices{
				Annotations:           map[string]string{"foo": "bar"},
				ExternalTrafficPolicy: &externalTrafficPolicy,
				Class:                 &lbClass,
				ProxyProtocol:         &gardencorev1beta1.LoadBalancerServicesProxyProtocol{Allowed: true},
			}
			botanist.Shoot.SetInfo(newShoot(true, lb))

			deployer, err := botanist.DefaultIstio(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(deployer).NotTo(BeNil())

			values := deployer.GetValues()
			Expect(values.NamePrefix).To(Equal("self-hosted-shoot-"))
			Expect(values.Istiod.Enabled).To(BeTrue())
			Expect(values.Istiod.Zones).To(ConsistOf("zone-a"))

			Expect(values.IngressGateway).To(HaveLen(1))
			gw := values.IngressGateway[0]
			Expect(gw.Annotations).To(Equal(map[string]string{"foo": "bar"}))
			Expect(gw.LoadBalancerClass).To(HaveValue(Equal(lbClass)))
			Expect(gw.ExternalTrafficPolicy).To(HaveValue(Equal(externalTrafficPolicy)))
			Expect(gw.TerminateLoadBalancerProxyProtocol).To(BeTrue())
			Expect(gw.VPNEnabled).To(BeFalse())
		})

		It("should defer istiod but keep the ingress gateway when the shoot is also the garden cluster", func() {
			Expect(botanist.SeedClientSet.Client().Create(ctx, &operatorv1alpha1.Garden{
				ObjectMeta: metav1.ObjectMeta{Name: "garden"},
			})).To(Succeed())
			botanist.Shoot.SetInfo(newShoot(true, nil))

			deployer, err := botanist.DefaultIstio(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(deployer).NotTo(BeNil())

			values := deployer.GetValues()
			Expect(values.NamePrefix).To(Equal("self-hosted-shoot-"))
			Expect(values.Istiod.Enabled).To(BeFalse())
			Expect(values.IngressGateway).To(HaveLen(1))
		})

		It("should keep istiod enabled when the shoot is also a seed cluster (self-hosted gardenlet outranks the seed)", func() {
			botanist.Shoot.SetInfo(newShoot(true, nil))

			deployer, err := botanist.DefaultIstio(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(deployer).NotTo(BeNil())

			values := deployer.GetValues()
			Expect(values.NamePrefix).To(Equal("self-hosted-shoot-"))
			Expect(values.Istiod.Enabled).To(BeTrue())
			Expect(values.IngressGateway).To(HaveLen(1))
		})

		It("should tolerate a nil LoadBalancerServices field for a self-hosted shoot", func() {
			botanist.Shoot.SetInfo(newShoot(true, nil))

			deployer, err := botanist.DefaultIstio(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(deployer).NotTo(BeNil())

			values := deployer.GetValues()
			Expect(values.NamePrefix).To(Equal("self-hosted-shoot-"))
			Expect(values.Istiod.Enabled).To(BeTrue())
			Expect(values.IngressGateway[0].Annotations).To(BeNil())
			Expect(values.IngressGateway[0].LoadBalancerClass).To(BeNil())
			Expect(values.IngressGateway[0].ExternalTrafficPolicy).To(BeNil())
			Expect(values.IngressGateway[0].TerminateLoadBalancerProxyProtocol).To(BeFalse())
		})
	})
})
