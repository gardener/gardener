// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/apis/config/gardenlet/v1alpha1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	seedpkg "github.com/gardener/gardener/pkg/gardenlet/operation/seed"
	shootpkg "github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
)

var _ = Describe("IstioBasicAuthServer", func() {
	var (
		botanist *Botanist
	)

	BeforeEach(func() {
		fakeClient := fakeclient.NewClientBuilder().Build()
		kubernetesClientSet := fakekubernetes.NewClientSetBuilder().WithClient(fakeClient).Build()

		botanist = &Botanist{Operation: &operation.Operation{
			Config: &gardenletconfigv1alpha1.GardenletConfiguration{
				SNI: &gardenletconfigv1alpha1.SNI{
					Ingress: &gardenletconfigv1alpha1.SNIIngress{
						Namespace: ptr.To("istio-ingress"),
					},
				},
			},
			Shoot: &shootpkg.Shoot{
				Components: &shootpkg.Components{
					ControlPlane: &shootpkg.ControlPlane{},
				},
			},
			Seed:          &seedpkg.Seed{},
			SeedClientSet: kubernetesClientSet,
		}}
		botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{})
	})

	Describe("#DefaultIstioBasicAuthServer", func() {
		It("should successfully create a istio-basic-auth-server interface", func() {
			istioBasicAuthServer, err := botanist.DefaultIstioBasicAuthServer()
			Expect(istioBasicAuthServer).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
