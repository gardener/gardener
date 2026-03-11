// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package persesoperator_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	. "github.com/gardener/gardener/pkg/component/observability/monitoring/persesoperator"
	"github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("CRD", func() {
	var (
		ctx          = context.TODO()
		c            client.Client
		deployWaiter component.DeployWaiter
	)

	BeforeEach(func() {
		var err error
		c = fake.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()

		deployWaiter, err = NewCRDs(c)
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("#Deploy", func() {
		It("should deploy the CRD", func() {
			Expect(deployWaiter.Deploy(ctx)).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKey{Name: "perses.perses.dev"}, &apiextensionsv1.CustomResourceDefinition{})).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKey{Name: "persesdashboards.perses.dev"}, &apiextensionsv1.CustomResourceDefinition{})).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKey{Name: "persesdatasources.perses.dev"}, &apiextensionsv1.CustomResourceDefinition{})).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKey{Name: "persesglobaldatasources.perses.dev"}, &apiextensionsv1.CustomResourceDefinition{})).To(Succeed())
		})

		// TODO(rickardsjp): Remove this test after v1.141 has been released.
		It("should delete old v1alpha1 CRDs before deploying new ones", func() {
			for _, name := range []string{
				"perses.perses.dev",
				"persesdashboards.perses.dev",
				"persesdatasources.perses.dev",
			} {
				Expect(c.Create(ctx, &apiextensionsv1.CustomResourceDefinition{
					ObjectMeta: metav1.ObjectMeta{Name: name},
					Spec: apiextensionsv1.CustomResourceDefinitionSpec{
						Group: "perses.dev",
						Names: apiextensionsv1.CustomResourceDefinitionNames{Plural: name[:len(name)-len(".perses.dev")], Kind: name[:len(name)-len(".perses.dev")]},
						Scope: apiextensionsv1.NamespaceScoped,
						Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
							{Name: "v1alpha1", Served: true, Storage: true, Schema: &apiextensionsv1.CustomResourceValidation{
								OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{Type: "object"},
							}},
						},
					},
				})).To(Succeed())
			}

			Expect(deployWaiter.Deploy(ctx)).To(Succeed())

			for _, name := range []string{
				"perses.perses.dev",
				"persesdashboards.perses.dev",
				"persesdatasources.perses.dev",
				"persesglobaldatasources.perses.dev",
			} {
				crd := &apiextensionsv1.CustomResourceDefinition{}
				Expect(c.Get(ctx, client.ObjectKey{Name: name}, crd)).To(Succeed())
				Expect(crd.Spec.Versions[0].Name).To(Equal("v1alpha2"))
			}
		})
	})

	Describe("#Destroy", func() {
		It("should delete the CRD", func() {
			Expect(deployWaiter.Destroy(ctx)).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKey{Name: "perses.perses.dev"}, &apiextensionsv1.CustomResourceDefinition{})).To(matchers.BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKey{Name: "persesdashboards.perses.dev"}, &apiextensionsv1.CustomResourceDefinition{})).To(matchers.BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKey{Name: "persesdatasources.perses.dev"}, &apiextensionsv1.CustomResourceDefinition{})).To(matchers.BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKey{Name: "persesglobaldatasources.perses.dev"}, &apiextensionsv1.CustomResourceDefinition{})).To(matchers.BeNotFoundError())
		})
	})
})
