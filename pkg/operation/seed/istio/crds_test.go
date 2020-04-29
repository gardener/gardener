// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

	cr "github.com/gardener/gardener/pkg/chartrenderer"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/test"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	. "github.com/gardener/gardener/pkg/operation/seed/istio"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"

	. "github.com/gardener/gardener/test/gomega"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/helm/pkg/engine"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
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
		Expect(apiextensionsv1beta1.AddToScheme(s)).NotTo(HaveOccurred())

		c = fake.NewFakeClientWithScheme(s)
		d := &test.FakeDiscovery{}

		cap, err := cr.DiscoverCapabilities(d)
		Expect(err).ToNot(HaveOccurred())

		renderer := cr.New(engine.New(), cap)
		a, err := test.NewTestApplier(c, d)
		Expect(err).ToNot(HaveOccurred())

		ca := kubernetes.NewChartApplier(renderer, a)
		Expect(ca).NotTo(BeNil(), "should return chart applier")

		crd = NewIstioCRD(ca, chartsRootPath)

	})

	JustBeforeEach(func() {
		Expect(crd.Deploy(ctx)).ToNot(HaveOccurred(), "istio crd deploy succeeds")
	})

	DescribeTable("CRD is deployed",
		func(crdName string) {
			Expect(c.Get(
				ctx,
				client.ObjectKey{Name: crdName},
				&apiextensionsv1beta1.CustomResourceDefinition{},
			)).ToNot(HaveOccurred())
		},

		Entry("MeshPolicy", "meshpolicies.authentication.istio.io"),
		Entry("Policy", "policies.authentication.istio.io"),
		Entry("DestinationRule", "destinationrules.networking.istio.io"),
		Entry("EnvoyFilter", "envoyfilters.networking.istio.io"),
		Entry("Gateways", "gateways.networking.istio.io"),
		Entry("ServiceEntry", "serviceentries.networking.istio.io"),
		Entry("Sidecar", "sidecars.networking.istio.io"),
		Entry("VirtualServices", "virtualservices.networking.istio.io"),
		Entry("AuthorizationPolicy", "authorizationpolicies.security.istio.io"),
		Entry("PeerAuthentication", "peerauthentications.security.istio.io"),
		Entry("RequestAuthentications", "requestauthentications.security.istio.io"),
		// TODO (mvladev): Entries bellow should be moved to unused CRDs table when
		// they are no longer used by future versions of istio.
		Entry("HTTPAPISpec (DEPRECATED, but needed)", "httpapispecs.config.istio.io"),
		Entry("QuotaSpecBinding (DEPRECATED, but needed)", "quotaspecbindings.config.istio.io"),
		Entry("HTTPAPISpecBinding (DEPRECATED, but needed)", "httpapispecbindings.config.istio.io"),
		Entry("QuotaSpec (DEPRECATED, but needed)", "quotaspecs.config.istio.io"),
		Entry("ClusterRBACConfig (DEPRECATED, but needed)", "clusterrbacconfigs.rbac.istio.io"),
		Entry("RBACConfig (DEPRECATED, but needed)", "rbacconfigs.rbac.istio.io"),
		Entry("ServiceRole (DEPRECATED, but needed)", "serviceroles.rbac.istio.io"),
		Entry("ServiceRoleBindings (DEPRECATED, but needed)", "servicerolebindings.rbac.istio.io"),
	)

	DescribeTable("unused CRDs are not deployed",
		func(crdName string) {
			Expect(c.Get(
				ctx,
				client.ObjectKey{Name: crdName},
				&apiextensionsv1beta1.CustomResourceDefinition{},
			)).To(BeNotFoundError())
		},

		Entry("AttributeManifsts", "attributemanifests.config.istio.io"),
		Entry("Handlers", "handlers.config.istio.io"),
		Entry("Instances", "instances.config.istio.io"),
		Entry("Rules", "rules.config.istio.io"),
	)
})
