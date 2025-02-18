// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package opentelemetryoperator_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

// var _ = Describe("CRDs", func() {
// 	var (
// 		ctx         context.Context
// 		c           client.Client
// 		crdDeployer component.DeployWaiter
// 	)
//
// 	BeforeEach(func() {
// 		ctx = context.TODO()
//
// 		s := runtime.NewScheme()
// 		Expect(apiextensionsv1.AddToScheme(s)).NotTo(HaveOccurred())
//
// 		c = fake.NewClientBuilder().WithScheme(s).Build()
//
// 		mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{apiextensionsv1.SchemeGroupVersion})
// 		mapper.Add(apiextensionsv1.SchemeGroupVersion.WithKind("CustomResourceDefinition"), meta.RESTScopeRoot)
//
// 		applier := kubernetes.NewApplier(c, mapper)
//
// 		crdDeployer = fluentoperator.NewCRDs(applier)
// 	})
//
// 	JustBeforeEach(func() {
// 		Expect(crdDeployer.Deploy(ctx)).ToNot(HaveOccurred(), "fluent operator crds deploy succeeds")
// 	})
//
// 	DescribeTable("CRD is deployed",
// 		func(crdName string) {
// 			Expect(c.Get(ctx, client.ObjectKey{Name: crdName}, &apiextensionsv1.CustomResourceDefinition{})).ToNot(HaveOccurred())
// 		},
//
// 		Entry("ClusterFilter", "clusterfilters.fluentbit.fluent.io"),
// 		Entry("ClusterFluentBitConfig", "clusterfluentbitconfigs.fluentbit.fluent.io"),
// 		Entry("ClusterInput", "clusterinputs.fluentbit.fluent.io"),
// 		Entry("ClusterOutput", "clusteroutputs.fluentbit.fluent.io"),
// 		Entry("ClusterParser", "clusterparsers.fluentbit.fluent.io"),
// 		Entry("FluentBit", "fluentbits.fluentbit.fluent.io"),
// 		Entry("Collector", "collectors.fluentbit.fluent.io"),
// 		Entry("FluentBitConfig", "fluentbitconfigs.fluentbit.fluent.io"),
// 		Entry("Filter", "filters.fluentbit.fluent.io"),
// 		Entry("Parser", "parsers.fluentbit.fluent.io"),
// 		Entry("Output", "outputs.fluentbit.fluent.io"),
// 		Entry("ClusterMultilineParser", "clustermultilineparsers.fluentbit.fluent.io"),
// 		Entry("MultilineParser", "multilineparsers.fluentbit.fluent.io"),
// 	)
//
// 	DescribeTable("should re-create CRD if it is deleted",
// 		func(crdName string) {
// 			Expect(c.Delete(ctx, &apiextensionsv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: crdName}}, &client.DeleteOptions{})).ToNot(HaveOccurred())
// 			Expect(c.Get(ctx, client.ObjectKey{Name: crdName}, &apiextensionsv1.CustomResourceDefinition{})).To(BeNotFoundError())
// 			Expect(crdDeployer.Deploy(ctx)).ToNot(HaveOccurred())
// 			Expect(c.Get(ctx, client.ObjectKey{Name: crdName}, &apiextensionsv1.CustomResourceDefinition{})).ToNot(HaveOccurred())
// 		},
//
// 		Entry("ClusterFilter", "clusterfilters.fluentbit.fluent.io"),
// 		Entry("ClusterFluentBitConfig", "clusterfluentbitconfigs.fluentbit.fluent.io"),
// 		Entry("ClusterInput", "clusterinputs.fluentbit.fluent.io"),
// 		Entry("ClusterOutput", "clusteroutputs.fluentbit.fluent.io"),
// 		Entry("ClusterParser", "clusterparsers.fluentbit.fluent.io"),
// 		Entry("FluentBit", "fluentbits.fluentbit.fluent.io"),
// 		Entry("Collectors", "collectors.fluentbit.fluent.io"),
// 		Entry("FluentBitConfig", "fluentbitconfigs.fluentbit.fluent.io"),
// 		Entry("Filter", "filters.fluentbit.fluent.io"),
// 		Entry("Parser", "parsers.fluentbit.fluent.io"),
// 		Entry("Output", "outputs.fluentbit.fluent.io"),
// 		Entry("ClusterMultilineParser", "clustermultilineparsers.fluentbit.fluent.io"),
// 		Entry("MultilineParser", "multilineparsers.fluentbit.fluent.io"),
// 	)
// })
