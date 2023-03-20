// Copyright 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package istio_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/version"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	chartrenderer "github.com/gardener/gardener/pkg/chartrenderer"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	. "github.com/gardener/gardener/pkg/operation/botanist/component/istio"
)

var _ = Describe("#CRDs", func() {
	var (
		ctx context.Context
		c   client.Client
		crd component.DeployWaiter
	)

	BeforeEach(func() {
		ctx = context.TODO()

		s := runtime.NewScheme()
		Expect(apiextensionsv1.AddToScheme(s)).NotTo(HaveOccurred())

		c = fake.NewClientBuilder().WithScheme(s).Build()

		mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{apiextensionsv1.SchemeGroupVersion})
		mapper.Add(apiextensionsv1.SchemeGroupVersion.WithKind("CustomResourceDefinition"), meta.RESTScopeRoot)

		renderer := chartrenderer.NewWithServerVersion(&version.Info{})

		ca := kubernetes.NewChartApplier(renderer, kubernetes.NewApplier(c, mapper))
		Expect(ca).NotTo(BeNil(), "should return chart applier")

		crd = NewCRD(ca, c)
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
