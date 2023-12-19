// SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	kubernetesfake "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	gardenletfeatures "github.com/gardener/gardener/pkg/gardenlet/features"
	"github.com/gardener/gardener/pkg/operation"
	"github.com/gardener/gardener/pkg/operation/garden"
	"github.com/gardener/gardener/pkg/operation/seed"
	"github.com/gardener/gardener/pkg/operation/shoot"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("KubeAPIServerExposure", func() {
	var (
		ctrl   *gomock.Controller
		scheme *runtime.Scheme
		c      client.Client

		botanist *Botanist

		ctx       = context.TODO()
		namespace = "shoot--foo--bar"
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())

		scheme = runtime.NewScheme()
		Expect(corev1.AddToScheme(scheme)).NotTo(HaveOccurred())
		Expect(networkingv1.AddToScheme(scheme)).NotTo(HaveOccurred())
		c = fake.NewClientBuilder().WithScheme(scheme).Build()

		fakeClientSet := kubernetesfake.NewClientSetBuilder().
			WithAPIReader(c).
			WithClient(c).
			Build()

		botanist = &Botanist{
			Operation: &operation.Operation{
				SeedClientSet: fakeClientSet,
				Shoot: &shoot.Shoot{
					SeedNamespace: namespace,
				},
				Garden: &garden.Garden{},
				Logger: logr.Discard(),
			},
		}
		botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{})
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#Ingress", func() {
		var (
			ingress *networkingv1.Ingress
			secret  *corev1.Secret
		)

		BeforeEach(func() {
			gardenletfeatures.RegisterFeatureGates()

			botanist.Shoot.Components = &shoot.Components{
				ControlPlane: &shoot.ControlPlane{},
			}

			kubernetesVersion := "1.26.0"
			botanist.Seed = &seed.Seed{}
			botanist.Seed.SetInfo(&gardencorev1beta1.Seed{
				Status: gardencorev1beta1.SeedStatus{
					KubernetesVersion: &kubernetesVersion,
				},
			})

			ingress = &networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "kube-apiserver",
					Namespace: namespace,
				},
			}

			secret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "wildcard-secret",
					Namespace: namespace,
					Labels: map[string]string{
						"gardener.cloud/role": "controlplane-cert",
					},
				},
			}
		})

		It("should create the ingress if there is a wildcard certificate", func() {
			botanist.ControlPlaneWildcardCert = secret
			botanist.Shoot.Components.ControlPlane.KubeAPIServerIngress = botanist.DefaultKubeAPIServerIngress()
			Expect(botanist.DeployKubeAPIServerIngress(ctx)).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(ingress), ingress)).To(Succeed())
		})

		It("should not create the ingress if there is no wildcard certificate", func() {
			botanist.Shoot.Components.ControlPlane.KubeAPIServerIngress = botanist.DefaultKubeAPIServerIngress()
			Expect(botanist.DeployKubeAPIServerIngress(ctx)).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(ingress), ingress)).To(BeNotFoundError())
		})

		It("should delete an existing ingress if there is no wildcard certificate", func() {
			Expect(c.Create(ctx, ingress)).To(Succeed())
			botanist.Shoot.Components.ControlPlane.KubeAPIServerIngress = botanist.DefaultKubeAPIServerIngress()
			Expect(botanist.DeployKubeAPIServerIngress(ctx)).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(ingress), ingress)).To(BeNotFoundError())
		})
	})
})
