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
	"time"

	cr "github.com/gardener/gardener/pkg/chartrenderer"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	. "github.com/gardener/gardener/pkg/operation/botanist/component/istio"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"

	"github.com/gogo/protobuf/types"
	meshv1alpha1 "istio.io/api/mesh/v1alpha1"
	"istio.io/api/networking/v1alpha3"
	networkingv1alpha3 "istio.io/client-go/pkg/apis/networking/v1alpha3"
	networkingv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/version"
	autoscalingv1beta2 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("istiod", func() {
	const (
		deployNS = "test"
	)

	var (
		ctx    context.Context
		c      client.Client
		istiod component.DeployWaiter
	)

	BeforeEach(func() {
		ctx = context.TODO()

		s := runtime.NewScheme()
		Expect(corev1.AddToScheme(s)).ToNot(HaveOccurred())
		Expect(appsv1.AddToScheme(s)).ToNot(HaveOccurred())
		Expect(policyv1beta1.AddToScheme(s)).ToNot(HaveOccurred())
		Expect(networkingv1beta1.AddToScheme(s)).NotTo(HaveOccurred())
		Expect(networkingv1alpha3.AddToScheme(s)).NotTo(HaveOccurred())
		Expect(autoscalingv1beta2.AddToScheme(s)).NotTo(HaveOccurred())
		Expect(autoscalingv1.AddToScheme(s)).NotTo(HaveOccurred())

		c = fake.NewClientBuilder().WithScheme(s).Build()
		renderer := cr.NewWithServerVersion(&version.Info{GitVersion: "v1.21.4"})

		ca := kubernetes.NewChartApplier(renderer, kubernetes.NewApplier(c, meta.NewDefaultRESTMapper([]schema.GroupVersion{})))
		Expect(ca).NotTo(BeNil(), "should return chart applier")

		istiod = NewIstiod(
			&IstiodValues{Image: "foo/bar", TrustDomain: "foo.local"},
			deployNS,
			ca,
			chartsRootPath,
			c,
		)
	})

	JustBeforeEach(func() {
		Expect(istiod.Deploy(ctx)).ToNot(HaveOccurred(), "istiod deploy succeeds")
	})

	It("deploys istiod namespace", func() {
		actualNS := &corev1.Namespace{}

		Expect(c.Get(ctx, client.ObjectKey{Name: deployNS}, actualNS)).ToNot(HaveOccurred())

		Expect(actualNS.Labels).To(HaveKeyWithValue("istio-operator-managed", "Reconcile"))
		Expect(actualNS.Labels).To(HaveKeyWithValue("istio-injection", "disabled"))
	})

	DescribeTable("istiod deployment has correct environment variables", func(env corev1.EnvVar) {
		actualDeploy := &appsv1.Deployment{}

		Expect(c.Get(ctx, client.ObjectKey{Name: "istiod", Namespace: deployNS}, actualDeploy)).ToNot(HaveOccurred())
		envs := actualDeploy.Spec.Template.Spec.Containers[0].Env

		Expect(envs).To(ContainElement(env))
	},
		Entry("JWT policy is third-party", simplEnv("JWT_POLICY", "third-party-jwt")),
		Entry("Cert provider is istiod", simplEnv("PILOT_CERT_PROVIDER", "istiod")),
		Entry("Trace sampling should be less that 1%", simplEnv("PILOT_TRACE_SAMPLING", "0.1")),
		Entry("POD_NAME is projected", fieldEnv("POD_NAME", "metadata.name")),
		Entry("POD_NAMESPACE is projected", fieldEnv("POD_NAMESPACE", "metadata.namespace")),
		Entry("SERVICE_ACCOUNT is projected", fieldEnv("SERVICE_ACCOUNT", "spec.serviceAccountName")),
		Entry("No protocol sniffing for outbout traffic", simplEnv("PILOT_ENABLE_PROTOCOL_SNIFFING_FOR_OUTBOUND", "false")),
		Entry("No protocol sniffing for inbound traffic", simplEnv("PILOT_ENABLE_PROTOCOL_SNIFFING_FOR_INBOUND", "false")),
		Entry("Injection webhook", simplEnv("INJECTION_WEBHOOK_CONFIG_NAME", "istio-sidecar-injector")),
		Entry("Advertised address includes NS", simplEnv("ISTIOD_ADDR", "istiod.test.svc:15012")),
		Entry("Validation webhook", simplEnv("VALIDATION_WEBHOOK_CONFIG_NAME", "istiod")),
		Entry("External Galley is disabled", simplEnv("PILOT_EXTERNAL_GALLEY", "false")),
		Entry("CLUSTER_ID is Kubernetes", simplEnv("CLUSTER_ID", "Kubernetes")),
	)

	It("istiod deployment has correct number of environment variables", func() {
		actualDeploy := &appsv1.Deployment{}

		Expect(c.Get(ctx, client.ObjectKey{Name: "istiod", Namespace: deployNS}, actualDeploy)).ToNot(HaveOccurred())
		Expect(actualDeploy.Spec.Template.Spec.Containers[0].Env).To(HaveLen(15))
	})

	Describe("poddisruption budget", func() {
		var pdb *policyv1beta1.PodDisruptionBudget

		JustBeforeEach(func() {
			pdb = &policyv1beta1.PodDisruptionBudget{}

			Expect(c.Get(
				ctx,
				client.ObjectKey{Name: "istiod", Namespace: deployNS},
				pdb),
			).ToNot(HaveOccurred(), "pdp get succeeds")
		})

		It("matches deployment labels", func() {
			actualDeploy := &appsv1.Deployment{}
			Expect(c.Get(ctx, client.ObjectKey{Name: "istiod", Namespace: deployNS}, actualDeploy)).ToNot(HaveOccurred())

			s, err := metav1.LabelSelectorAsSelector(pdb.Spec.Selector)
			Expect(err).ToNot(HaveOccurred(), "selector can be parsed")

			Expect(s.Matches(labels.Set(actualDeploy.Labels))).To(BeTrue())
		})

		It("requires minimum one replica", func() {
			Expect(pdb.Spec.MinAvailable.IntValue()).To(Equal(1))
		})
	})

	Describe("vertical pod autoscaler", func() {
		var vpa *autoscalingv1beta2.VerticalPodAutoscaler

		JustBeforeEach(func() {
			vpa = &autoscalingv1beta2.VerticalPodAutoscaler{}

			Expect(c.Get(
				ctx,
				client.ObjectKey{Name: "istiod", Namespace: deployNS},
				vpa),
			).To(Succeed(), "VPA get succeeds")
		})

		It("targets correct deployment", func() {
			Expect(vpa.Spec.TargetRef).To(PointTo(Equal(autoscalingv1.CrossVersionObjectReference{
				Kind:       "Deployment",
				Name:       "istiod",
				APIVersion: "apps/v1",
			})))
		})

		It("has auto policy", func() {
			Expect(vpa.Spec.UpdatePolicy).ToNot(BeNil())
			Expect(vpa.Spec.UpdatePolicy.UpdateMode).To(PointTo(Equal(autoscalingv1beta2.UpdateModeAuto)))
		})

		It("has only works on memory", func() {
			Expect(vpa.Spec.ResourcePolicy).ToNot(BeNil())
			Expect(vpa.Spec.ResourcePolicy.ContainerPolicies).To(ConsistOf(autoscalingv1beta2.ContainerResourcePolicy{
				ContainerName: "discovery",
				MinAllowed: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse("128Mi"),
					corev1.ResourceCPU:    resource.MustParse("100m"),
				},
			}))
		})
	})

	It("has correct mesh configuration", func() {
		meshConfig := &corev1.ConfigMap{}
		Expect(c.Get(ctx, client.ObjectKey{Name: "istio", Namespace: deployNS}, meshConfig)).ToNot(HaveOccurred())

		Expect(meshConfig.Data).To(HaveKey("mesh"))
		Expect(meshConfig.Data).To(HaveKey("meshNetworks"))

		mc := &meshv1alpha1.MeshConfig{}

		Expect(applyYAML([]byte(meshConfig.Data["mesh"]), mc)).ToNot(HaveOccurred(), "mesh config conversion is successful")

		expectedMC := &meshv1alpha1.MeshConfig{
			// default values start
			// see https://github.com/istio/istio/blob/06abc5460c44912254f032fe12f119f33ab790b4/pkg/config/mesh/mesh.go#L57-L86
			// this is no referenced directly so istio is not addeded as dependency
			RootNamespace:            "istio-system",
			ProxyListenPort:          0,
			IngressService:           "istio-ingressgateway",
			AccessLogFile:            "",
			TrustDomainAliases:       nil,
			ProtocolDetectionTimeout: types.DurationProto(100 * time.Millisecond),
			EnableAutoMtls:           &types.BoolValue{Value: true},
			// default values end
			EnableTracing:               false,
			AccessLogFormat:             "",
			AccessLogEncoding:           meshv1alpha1.MeshConfig_TEXT,
			EnableEnvoyAccessLogService: false,
			Certificates:                []*meshv1alpha1.Certificate{},
			IngressClass:                "istio",
			IngressControllerMode:       meshv1alpha1.MeshConfig_OFF,
			TrustDomain:                 "foo.local",
			OutboundTrafficPolicy: &meshv1alpha1.MeshConfig_OutboundTrafficPolicy{
				Mode: meshv1alpha1.MeshConfig_OutboundTrafficPolicy_REGISTRY_ONLY,
			},
			LocalityLbSetting: &v1alpha3.LocalityLoadBalancerSetting{
				Enabled: &types.BoolValue{Value: true},
			},
			DefaultServiceExportTo:         []string{"-"},
			DefaultVirtualServiceExportTo:  []string{"-"},
			DefaultDestinationRuleExportTo: []string{"-"},
			DefaultConfig: &meshv1alpha1.ProxyConfig{
				ConfigPath:             "/etc/istio/proxy",
				ServiceCluster:         "istio-proxy",
				DrainDuration:          &types.Duration{Seconds: 45},
				ParentShutdownDuration: &types.Duration{Seconds: 60},
				ProxyAdminPort:         15000,
				Concurrency:            &types.Int32Value{Value: 2},
				ControlPlaneAuthPolicy: meshv1alpha1.AuthenticationPolicy_NONE,
				DiscoveryAddress:       "istiod.test.svc:15012",
			},
			EnablePrometheusMerge: &types.BoolValue{Value: true},
		}

		Expect(mc).To(BeEquivalentTo(expectedMC))
	})
})

func simplEnv(env, val string) corev1.EnvVar {
	return corev1.EnvVar{Name: env, Value: val}
}

func fieldEnv(env, fieldPath string) corev1.EnvVar {
	return corev1.EnvVar{
		Name:      env,
		ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{APIVersion: "v1", FieldPath: fieldPath}},
	}
}
