// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package operator_test

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
	oteloperator "github.com/gardener/gardener/pkg/component/observability/opentelemetry/operator"
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

		crdDeployer, err = oteloperator.NewCRDs(c)
		Expect(err).NotTo(HaveOccurred())
	})

	JustBeforeEach(func() {
		Expect(crdDeployer.Deploy(ctx)).To(Succeed(), "opentelemetry operator crds deploy succeeds")
	})

	DescribeTable("CRD is deployed",
		func(crdName string) {
			Expect(c.Get(ctx, client.ObjectKey{Name: crdName}, &apiextensionsv1.CustomResourceDefinition{})).To(Succeed())
		},

		Entry("OpenTelemetryCollector", "opentelemetrycollectors.opentelemetry.io"),
		Entry("Instrumentation", "instrumentations.opentelemetry.io"),
		Entry("OpamBridge", "opampbridges.opentelemetry.io"),
		Entry("TargetAllocator", "targetallocators.opentelemetry.io"),
	)

	DescribeTable("should re-create CRD if it is deleted",
		func(crdName string) {
			Expect(c.Delete(ctx, &apiextensionsv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: crdName}}, &client.DeleteOptions{})).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKey{Name: crdName}, &apiextensionsv1.CustomResourceDefinition{})).To(BeNotFoundError())
			Expect(crdDeployer.Deploy(ctx)).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKey{Name: crdName}, &apiextensionsv1.CustomResourceDefinition{})).To(Succeed())
		},

		Entry("OpenTelemetryCollector", "opentelemetrycollectors.opentelemetry.io"),
		Entry("Instrumentation", "instrumentations.opentelemetry.io"),
		Entry("OpamBridge", "opampbridges.opentelemetry.io"),
		Entry("TargetAllocator", "targetallocators.opentelemetry.io"),
	)
})
