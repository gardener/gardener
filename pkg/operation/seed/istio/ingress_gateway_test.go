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
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	. "github.com/gardener/gardener/pkg/operation/seed/istio"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/version"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("ingress", func() {
	const (
		deployNS = "test-ingress"
	)

	var (
		ctx context.Context
		c   client.Client
		igw component.DeployWaiter
	)

	BeforeEach(func() {
		ctx = context.TODO()

		s := runtime.NewScheme()
		Expect(corev1.AddToScheme(s)).ToNot(HaveOccurred())
		Expect(appsv1.AddToScheme(s)).ToNot(HaveOccurred())
		Expect(policyv1beta1.AddToScheme(s)).ToNot(HaveOccurred())

		c = fake.NewFakeClientWithScheme(s)
		renderer := cr.NewWithServerVersion(&version.Info{})

		ca := kubernetes.NewChartApplier(renderer, kubernetes.NewApplier(c, meta.NewDefaultRESTMapper([]schema.GroupVersion{})))
		Expect(ca).NotTo(BeNil(), "should return chart applier")
		igw = NewIngressGateway(
			&IngressValues{
				Image:           "foo/bar",
				TrustDomain:     "foo.bar",
				IstiodNamespace: "istio-test-system",
				Annotations:     map[string]string{"foo": "bar"},
				Ports: []corev1.ServicePort{
					{Name: "foo", Port: 999, TargetPort: intstr.FromInt(999)},
				},
			},
			deployNS,
			ca,
			chartsRootPath,
			c,
		)
	})

	JustBeforeEach(func() {
		Expect(igw.Deploy(ctx)).ToNot(HaveOccurred(), "ingress gateway deploy succeeds")
	})

	It("deploys istio-system namespace", func() {
		actualNS := &corev1.Namespace{}

		Expect(c.Get(ctx, client.ObjectKey{Name: deployNS}, actualNS)).To(Succeed())

		Expect(actualNS.Labels).To(HaveKeyWithValue("istio-operator-managed", "Reconcile"))
		Expect(actualNS.Labels).To(HaveKeyWithValue("istio-injection", "disabled"))
	})

	DescribeTable("ingress gateway deployment has correct environment variables",
		func(env corev1.EnvVar) {
			actualDeploy := &appsv1.Deployment{}

			Expect(c.Get(ctx, client.ObjectKey{Name: "istio-ingressgateway", Namespace: deployNS}, actualDeploy)).ToNot(HaveOccurred())
			Expect(actualDeploy.Spec.Template.Spec.Containers[0].Env).To(ContainElement(env))
		},
		Entry("NODE_NAME is projected", fieldEnv("NODE_NAME", "spec.nodeName")),
		Entry("POD_NAME is projected", fieldEnv("POD_NAME", "metadata.name")),
		Entry("POD_NAMESPACE is projected", fieldEnv("POD_NAMESPACE", "metadata.namespace")),
		Entry("INSTANCE_IP is projected", fieldEnv("INSTANCE_IP", "status.podIP")),
		Entry("HOST_IP is projected", fieldEnv("HOST_IP", "status.hostIP")),
		Entry("SERVICE_ACCOUNT is projected", fieldEnv("SERVICE_ACCOUNT", "spec.serviceAccountName")),
		Entry("ISTIO_META_POD_NAME is projected", fieldEnv("ISTIO_META_POD_NAME", "metadata.name")),
		Entry("ISTIO_META_CONFIG_NAMESPACE is projected", fieldEnv("ISTIO_META_CONFIG_NAMESPACE", "metadata.namespace")),
		Entry("JWT policy is third-party", simplEnv("JWT_POLICY", "third-party-jwt")),
		Entry("Cert provider is istiod", simplEnv("PILOT_CERT_PROVIDER", "istiod")),
		Entry("Use SDS", simplEnv("ISTIO_META_USER_SDS", "true")),
		Entry("istiod address is set", simplEnv("CA_ADDR", "istiod.istio-test-system.svc:15012")),
		Entry("workload name is set", simplEnv("ISTIO_META_WORKLOAD_NAME", "istio-ingressgateway")),
		Entry("meta owner is igw",
			simplEnv("ISTIO_META_OWNER", "kubernetes://apis/apps/v1/namespaces/test-ingress/deployments/istio-ingressgateway")),
		Entry("auto mTLS is enabled", simplEnv("ISTIO_AUTO_MTLS_ENABLED", "true")),
		Entry("router mode is standard", simplEnv("ISTIO_META_ROUTER_MODE", "standard")),
		Entry("ISTIO_META_CLUSTER_ID is Kubernetes", simplEnv("ISTIO_META_CLUSTER_ID", "Kubernetes")),
		Entry("ISTIO_BOOTSTRAP_OVERRIDE is set to override",
			simplEnv("ISTIO_BOOTSTRAP_OVERRIDE", "/etc/istio/custom-bootstrap/custom_bootstrap.yaml")),
	)

	It("ingress gateway deployment has correct amount of environment variables", func() {
		actualDeploy := &appsv1.Deployment{}

		Expect(c.Get(ctx, client.ObjectKey{Name: "istio-ingressgateway", Namespace: deployNS}, actualDeploy)).ToNot(HaveOccurred())
		Expect(actualDeploy.Spec.Template.Spec.Containers[0].Env).To(HaveLen(18))
	})

	It("ingress gateway service has load balancer annotations", func() {
		svc := &corev1.Service{}

		Expect(c.Get(ctx, client.ObjectKey{Name: "istio-ingressgateway", Namespace: deployNS}, svc)).To(Succeed())
		Expect(svc.Annotations).To(HaveKeyWithValue("foo", "bar"))
	})

	Describe("poddisruption budget", func() {
		var pdb *policyv1beta1.PodDisruptionBudget

		JustBeforeEach(func() {
			pdb = &policyv1beta1.PodDisruptionBudget{}

			Expect(c.Get(
				ctx,
				client.ObjectKey{Name: "istio-ingressgateway", Namespace: deployNS},
				pdb),
			).ToNot(HaveOccurred(), "pdp get succeeds")
		})

		It("matches deployment labels", func() {
			actualDeploy := &appsv1.Deployment{}

			Expect(c.Get(
				ctx,
				client.ObjectKey{Name: "istio-ingressgateway", Namespace: deployNS},
				actualDeploy),
			).ToNot(HaveOccurred(), "igw deployment get succeeds")

			s, err := metav1.LabelSelectorAsSelector(pdb.Spec.Selector)
			Expect(err).ToNot(HaveOccurred(), "selector can be parsed")

			Expect(s.Matches(labels.Set(actualDeploy.Labels))).To(BeTrue())
		})

		It("requires minimum one replica", func() {
			Expect(pdb.Spec.MinAvailable.IntValue()).To(Equal(1))
		})
	})
})
