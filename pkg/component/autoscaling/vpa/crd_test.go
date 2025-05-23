// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package vpa_test

import (
	"context"
	_ "embed"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	. "github.com/gardener/gardener/pkg/component/autoscaling/vpa"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("CRD", func() {
	var (
		ctx         = context.Background()
		crdDeployer component.Deployer
	)

	JustBeforeEach(func() {
		Expect(crdDeployer.Deploy(ctx)).To(Succeed(), "VPA CRD deployment succeeds")
	})

	Context("with applier", func() {
		var c client.Client

		BeforeEach(func() {
			var err error
			c = fake.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()

			mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{apiextensionsv1.SchemeGroupVersion})
			mapper.Add(apiextensionsv1.SchemeGroupVersion.WithKind("CustomResourceDefinition"), meta.RESTScopeRoot)
			applier := kubernetes.NewApplier(c, mapper)

			crdDeployer, err = NewCRD(c, applier, nil)
			Expect(err).NotTo(HaveOccurred())
		})

		DescribeTable("CRD is deployed",
			func(crdName string) {
				Expect(c.Get(ctx, client.ObjectKey{Name: crdName}, &apiextensionsv1.CustomResourceDefinition{})).To(Succeed())
			},

			Entry("VerticalPodAutoscaler", "verticalpodautoscalers.autoscaling.k8s.io"),
			Entry("VerticalPodAutoscalerCheckpoints", "verticalpodautoscalercheckpoints.autoscaling.k8s.io"),
		)

		DescribeTable("should re-create CRD if it is deleted",
			func(crdName string) {
				Expect(c.Delete(ctx, &apiextensionsv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: crdName}})).To(Succeed())
				Expect(c.Get(ctx, client.ObjectKey{Name: crdName}, &apiextensionsv1.CustomResourceDefinition{})).To(BeNotFoundError())
				Expect(crdDeployer.Deploy(ctx)).To(Succeed())
				Expect(c.Get(ctx, client.ObjectKey{Name: crdName}, &apiextensionsv1.CustomResourceDefinition{})).To(Succeed())
			},

			Entry("VerticalPodAutoscaler", "verticalpodautoscalers.autoscaling.k8s.io"),
			Entry("VerticalPodAutoscalerCheckpoints", "verticalpodautoscalercheckpoints.autoscaling.k8s.io"),
		)
	})

	Context("with registry", func() {
		var (
			registry *managedresources.Registry
			c        client.Client
		)

		BeforeEach(func() {
			var err error
			c = fake.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
			registry = managedresources.NewRegistry(kubernetes.SeedScheme, kubernetes.SeedCodec, kubernetes.SeedSerializer)

			crdDeployer, err = NewCRD(c, nil, registry)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should ensure CRDs are included", func() {
			dataMap, err := registry.SerializedObjects()
			Expect(err).NotTo(HaveOccurred())
			Expect(dataMap).To(HaveKey("data.yaml.br"))

			compressedData := dataMap["data.yaml.br"]
			data, err := test.BrotliDecompression(compressedData)
			Expect(err).NotTo(HaveOccurred())

			Expect(data).To(ContainSubstring("listKind: VerticalPodAutoscalerList"))
			Expect(data).To(ContainSubstring("listKind: VerticalPodAutoscalerCheckpointList"))
		})
	})
})
