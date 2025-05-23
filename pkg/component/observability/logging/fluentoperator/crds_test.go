// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package fluentoperator_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/observability/logging/fluentoperator"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("CRDs", func() {
	var (
		ctx         context.Context
		c           client.Client
		crdDeployer component.DeployWaiter
	)

	BeforeEach(func() {
		var err error
		ctx = context.TODO()

		s := runtime.NewScheme()
		Expect(apiextensionsv1.AddToScheme(s)).NotTo(HaveOccurred())

		c = fake.NewClientBuilder().WithScheme(s).Build()

		mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{apiextensionsv1.SchemeGroupVersion})
		mapper.Add(apiextensionsv1.SchemeGroupVersion.WithKind("CustomResourceDefinition"), meta.RESTScopeRoot)

		applier := kubernetes.NewApplier(c, mapper)

		crdDeployer, err = fluentoperator.NewCRDs(c, applier)
		Expect(err).NotTo(HaveOccurred())
	})

	JustBeforeEach(func() {
		Expect(crdDeployer.Deploy(ctx)).ToNot(HaveOccurred(), "fluent operator crds deploy succeeds")
	})

	DescribeTable("CRD is deployed",
		func(crdName string) {
			Expect(c.Get(ctx, client.ObjectKey{Name: crdName}, &apiextensionsv1.CustomResourceDefinition{})).ToNot(HaveOccurred())
		},

		Entry("ClusterFilter", "clusterfilters.fluentbit.fluent.io"),
		Entry("ClusterFluentBitConfig", "clusterfluentbitconfigs.fluentbit.fluent.io"),
		Entry("ClusterInput", "clusterinputs.fluentbit.fluent.io"),
		Entry("ClusterOutput", "clusteroutputs.fluentbit.fluent.io"),
		Entry("ClusterParser", "clusterparsers.fluentbit.fluent.io"),
		Entry("FluentBit", "fluentbits.fluentbit.fluent.io"),
		Entry("Collector", "collectors.fluentbit.fluent.io"),
		Entry("FluentBitConfig", "fluentbitconfigs.fluentbit.fluent.io"),
		Entry("Filter", "filters.fluentbit.fluent.io"),
		Entry("Parser", "parsers.fluentbit.fluent.io"),
		Entry("Output", "outputs.fluentbit.fluent.io"),
		Entry("ClusterMultilineParser", "clustermultilineparsers.fluentbit.fluent.io"),
		Entry("MultilineParser", "multilineparsers.fluentbit.fluent.io"),
	)

	DescribeTable("should re-create CRD if it is deleted",
		func(crdName string) {
			Expect(c.Delete(ctx, &apiextensionsv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: crdName}})).ToNot(HaveOccurred())
			Expect(c.Get(ctx, client.ObjectKey{Name: crdName}, &apiextensionsv1.CustomResourceDefinition{})).To(BeNotFoundError())
			Expect(crdDeployer.Deploy(ctx)).ToNot(HaveOccurred())
			Expect(c.Get(ctx, client.ObjectKey{Name: crdName}, &apiextensionsv1.CustomResourceDefinition{})).ToNot(HaveOccurred())
		},

		Entry("ClusterFilter", "clusterfilters.fluentbit.fluent.io"),
		Entry("ClusterFluentBitConfig", "clusterfluentbitconfigs.fluentbit.fluent.io"),
		Entry("ClusterInput", "clusterinputs.fluentbit.fluent.io"),
		Entry("ClusterOutput", "clusteroutputs.fluentbit.fluent.io"),
		Entry("ClusterParser", "clusterparsers.fluentbit.fluent.io"),
		Entry("FluentBit", "fluentbits.fluentbit.fluent.io"),
		Entry("Collectors", "collectors.fluentbit.fluent.io"),
		Entry("FluentBitConfig", "fluentbitconfigs.fluentbit.fluent.io"),
		Entry("Filter", "filters.fluentbit.fluent.io"),
		Entry("Parser", "parsers.fluentbit.fluent.io"),
		Entry("Output", "outputs.fluentbit.fluent.io"),
		Entry("ClusterMultilineParser", "clustermultilineparsers.fluentbit.fluent.io"),
		Entry("MultilineParser", "multilineparsers.fluentbit.fluent.io"),
	)
})
