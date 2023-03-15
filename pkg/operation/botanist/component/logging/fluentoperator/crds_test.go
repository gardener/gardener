// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/operation/botanist/component/logging/fluentoperator"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("#CRDs", func() {
	var (
		ctx         context.Context
		c           client.Client
		crdDeployer component.DeployWaiter
	)

	BeforeEach(func() {
		ctx = context.TODO()

		s := runtime.NewScheme()
		Expect(apiextensionsv1.AddToScheme(s)).NotTo(HaveOccurred())

		c = fake.NewClientBuilder().WithScheme(s).Build()

		mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{apiextensionsv1.SchemeGroupVersion})
		mapper.Add(apiextensionsv1.SchemeGroupVersion.WithKind("CustomResourceDefinition"), meta.RESTScopeRoot)

		applier := kubernetes.NewApplier(c, mapper)

		crdDeployer = fluentoperator.NewCRDs(applier)
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

		Entry("ClusterFilter", "clusterfilters.fluentd.fluent.io"),
		Entry("ClusterFluentdConfig", "clusterfluentdconfigs.fluentd.fluent.io"),
		Entry("ClusterOutput", "clusteroutputs.fluentd.fluent.io"),
		Entry("Filter", "filters.fluentd.fluent.io"),
		Entry("FluentdConfig", "fluentdconfigs.fluentd.fluent.io"),
		Entry("Fluentd", "fluentds.fluentd.fluent.io"),
		Entry("Output", "outputs.fluentd.fluent.io"),
	)

	DescribeTable("should re-create CRD if it is deleted",
		func(crdName string) {
			Expect(c.Delete(ctx, &apiextensionsv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: crdName}}, &client.DeleteOptions{})).ToNot(HaveOccurred())
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

		Entry("ClusterFilter", "clusterfilters.fluentd.fluent.io"),
		Entry("ClusterFluentdConfig", "clusterfluentdconfigs.fluentd.fluent.io"),
		Entry("ClusterOutput", "clusteroutputs.fluentd.fluent.io"),
		Entry("Filter", "filters.fluentd.fluent.io"),
		Entry("FluentdConfig", "fluentdconfigs.fluentd.fluent.io"),
		Entry("Fluentd", "fluentds.fluentd.fluent.io"),
		Entry("Output", "outputs.fluentd.fluent.io"),
	)
})
