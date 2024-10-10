// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package hvpa_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	. "github.com/gardener/gardener/pkg/component/autoscaling/hvpa"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("CRD", func() {
	var (
		ctx         = context.TODO()
		c           client.Client
		crdDeployer component.Deployer
	)

	BeforeEach(func() {
		c = fake.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()

		mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{apiextensionsv1.SchemeGroupVersion})
		mapper.Add(apiextensionsv1.SchemeGroupVersion.WithKind("CustomResourceDefinition"), meta.RESTScopeRoot)
		applier := kubernetes.NewApplier(c, mapper)

		crdDeployer = NewCRD(c, applier)
	})

	Describe("#Deploy", func() {
		It("should deploy the CRD", func() {
			Expect(crdDeployer.Deploy(ctx)).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKey{Name: "hvpas.autoscaling.k8s.io"}, &apiextensionsv1.CustomResourceDefinition{})).To(Succeed())
		})
	})

	Describe("#Destroy", func() {
		It("should delete the CRD", func() {
			Expect(crdDeployer.Destroy(ctx)).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKey{Name: "hvpas.autoscaling.k8s.io"}, &apiextensionsv1.CustomResourceDefinition{})).To(BeNotFoundError())
		})
	})
})
