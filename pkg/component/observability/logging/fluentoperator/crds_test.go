// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package fluentoperator_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/observability/logging/fluentoperator"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("CRDs", func() {
	var (
		ctx          context.Context
		c            client.Client
		crdDeployer  component.DeployWaiter
		expectedCRDs []string
	)

	BeforeEach(func() {
		ctx = context.TODO()

		s := runtime.NewScheme()
		Expect(apiextensionsv1.AddToScheme(s)).To(Succeed())

		c = fake.NewClientBuilder().WithScheme(s).Build()

		var err error
		crdDeployer, err = fluentoperator.NewCRDs(c)
		Expect(err).NotTo(HaveOccurred())

		expectedCRDs = []string{
			"clusterfilters.fluentbit.fluent.io",
			"clusterfluentbitconfigs.fluentbit.fluent.io",
			"clusterinputs.fluentbit.fluent.io",
			"clusteroutputs.fluentbit.fluent.io",
			"clusterparsers.fluentbit.fluent.io",
			"fluentbits.fluentbit.fluent.io",
			"collectors.fluentbit.fluent.io",
			"fluentbitconfigs.fluentbit.fluent.io",
			"filters.fluentbit.fluent.io",
			"parsers.fluentbit.fluent.io",
			"outputs.fluentbit.fluent.io",
			"clustermultilineparsers.fluentbit.fluent.io",
			"multilineparsers.fluentbit.fluent.io",
		}
	})

	JustBeforeEach(func() {
		Expect(crdDeployer.Deploy(ctx)).To(Succeed(), "fluent operator crds deploy succeeds")
	})

	Describe("#Deploy", func() {
		It("should deploy CRDs", func() {
			for _, crdName := range expectedCRDs {
				Expect(c.Get(ctx, client.ObjectKey{Name: crdName}, &apiextensionsv1.CustomResourceDefinition{})).To(Succeed(), crdName+" should get created")
			}
		})

		It("should re-create CRDs if they are deleted", func() {
			for _, crdName := range expectedCRDs {
				Expect(c.Delete(ctx, &apiextensionsv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: crdName}})).To(Succeed())
				Expect(c.Get(ctx, client.ObjectKey{Name: crdName}, &apiextensionsv1.CustomResourceDefinition{})).To(BeNotFoundError())
			}

			Expect(crdDeployer.Deploy(ctx)).To(Succeed())

			for _, crdName := range expectedCRDs {
				Expect(c.Get(ctx, client.ObjectKey{Name: crdName}, &apiextensionsv1.CustomResourceDefinition{})).To(Succeed(), crdName+" should get recreated")
			}
		})
	})

	Describe("#Destroy", func() {
		It("should destroy CRDs", func() {
			Expect(crdDeployer.Destroy(ctx)).To(Succeed())
			for _, crdName := range expectedCRDs {
				Expect(c.Get(ctx, client.ObjectKey{Name: crdName}, &apiextensionsv1.CustomResourceDefinition{})).To(BeNotFoundError(), crdName+" should get deleted")
			}
		})
	})
})
