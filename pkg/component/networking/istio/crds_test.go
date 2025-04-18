// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package istio_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	. "github.com/gardener/gardener/pkg/component/networking/istio"
)

var _ = Describe("#CRDs", func() {
	var (
		ctx context.Context
		c   client.Client
		crd component.DeployWaiter
	)

	BeforeEach(func() {
		ctx = context.TODO()
		var err error

		s := runtime.NewScheme()
		Expect(apiextensionsv1.AddToScheme(s)).NotTo(HaveOccurred())

		c = fake.NewClientBuilder().WithScheme(s).Build()

		mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{apiextensionsv1.SchemeGroupVersion})
		mapper.Add(apiextensionsv1.SchemeGroupVersion.WithKind("CustomResourceDefinition"), meta.RESTScopeRoot)

		applier := kubernetes.NewApplier(c, mapper)
		Expect(applier).NotTo(BeNil(), "should return applier")

		crd, err = NewCRD(c, applier)
		Expect(err).NotTo(HaveOccurred())
	})

	JustBeforeEach(func() {
		Expect(crd.Deploy(ctx)).ToNot(HaveOccurred(), "istio crd deploy succeeds")
	})

	DescribeTable("CRD is deployed",
		func(crdName string) {
			Expect(c.Get(
				ctx,
				client.ObjectKey{Name: crdName},
				&apiextensionsv1.CustomResourceDefinition{},
			)).ToNot(HaveOccurred())
		},
		Entry("DestinationRule", "destinationrules.networking.istio.io"),
		Entry("EnvoyFilter", "envoyfilters.networking.istio.io"),
		Entry("Gateways", "gateways.networking.istio.io"),
		Entry("ServiceEntry", "serviceentries.networking.istio.io"),
		Entry("Sidecar", "sidecars.networking.istio.io"),
		Entry("VirtualServices", "virtualservices.networking.istio.io"),
		Entry("AuthorizationPolicy", "authorizationpolicies.security.istio.io"),
		Entry("PeerAuthentication", "peerauthentications.security.istio.io"),
		Entry("RequestAuthentications", "requestauthentications.security.istio.io"),
		Entry("WorkloadEntries", "workloadentries.networking.istio.io"),
		Entry("WorkloadGroups", "workloadgroups.networking.istio.io"),
		Entry("Telemetry", "telemetries.telemetry.istio.io"),
		Entry("WasmPlugin", "wasmplugins.extensions.istio.io"),
	)
})
