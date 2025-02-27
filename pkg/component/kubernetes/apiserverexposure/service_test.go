// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package apiserverexposure_test

import (
	"context"

	"github.com/Masterminds/semver/v3"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/gardener/gardener/pkg/component"
	. "github.com/gardener/gardener/pkg/component/kubernetes/apiserverexposure"
	"github.com/gardener/gardener/pkg/utils"
	retryfake "github.com/gardener/gardener/pkg/utils/retry/fake"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("#Service", func() {
	var (
		log logr.Logger
		ctx context.Context
		c   client.Client

		defaultDepWaiter component.DeployWaiter
		values           *ServiceValues
		expected         *corev1.Service

		ingressIP        string
		clusterIP        string
		clusterIPsFunc   func([]string)
		ingressIPFunc    func(string)
		namePrefix       string
		namespace        string
		expectedName     string
		sniServiceObjKey client.ObjectKey
	)

	BeforeEach(func() {
		log = logr.Discard()
		ctx = context.TODO()

		s := runtime.NewScheme()
		Expect(corev1.AddToScheme(s)).To(Succeed())
		c = fake.NewClientBuilder().WithScheme(s).Build()

		ingressIP = ""
		clusterIP = ""
		namePrefix = "test-"
		namespace = "test-namespace"
		expectedName = "test-kube-apiserver"
		sniServiceObjKey = client.ObjectKey{Name: "foo", Namespace: "bar"}
		clusterIPsFunc = func(c []string) { clusterIP = c[0] }
		ingressIPFunc = func(c string) { ingressIP = c }

		values = &ServiceValues{
			AnnotationsFunc: func() map[string]string { return map[string]string{"foo": "bar"} },
			NamePrefix:      namePrefix,
		}
		expected = &corev1.Service{
			TypeMeta: metav1.TypeMeta{
				APIVersion: corev1.SchemeGroupVersion.String(),
				Kind:       "Service",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      expectedName,
				Namespace: namespace,
				Labels: map[string]string{
					"app":  "kubernetes",
					"role": "apiserver",
				},
			},
			Spec: corev1.ServiceSpec{
				Type: corev1.ServiceTypeClusterIP,
				Ports: []corev1.ServicePort{{
					Name:       "kube-apiserver",
					Port:       443,
					Protocol:   corev1.ProtocolTCP,
					TargetPort: intstr.FromInt32(443),
				}},
				Selector: map[string]string{
					"app":  "kubernetes",
					"role": "apiserver",
				},
				ClusterIP:      "1.1.1.1",
				ClusterIPs:     []string{"1.1.1.1"},
				IPFamilyPolicy: ptr.To(corev1.IPFamilyPolicyPreferDualStack),
			},
		}

		sniService := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      sniServiceObjKey.Name,
				Namespace: sniServiceObjKey.Namespace,
			},
			Status: corev1.ServiceStatus{
				LoadBalancer: corev1.LoadBalancerStatus{
					Ingress: []corev1.LoadBalancerIngress{{IP: "2.2.2.2"}},
				},
			},
		}

		Expect(c.Create(ctx, sniService)).To(Succeed())
	})

	JustBeforeEach(func() {
		Expect(c.Create(ctx, expected)).To(Succeed())
		expected.ResourceVersion = "2"

		defaultDepWaiter = NewService(
			log,
			c,
			namespace,
			values,
			func() client.ObjectKey { return sniServiceObjKey },
			&retryfake.Ops{MaxAttempts: 1},
			clusterIPsFunc,
			ingressIPFunc,
		)
	})

	var assertService = func() {
		It("deploys service", func() {
			Expect(defaultDepWaiter.Deploy(ctx)).To(Succeed())

			actual := &corev1.Service{}
			Expect(c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: expectedName}, actual)).To(Succeed())

			Expect(actual).To(DeepEqual(expected))
			Expect(clusterIP).To(Equal("1.1.1.1"))
		})

		It("waits for service", func() {
			Expect(defaultDepWaiter.Deploy(ctx)).To(Succeed())
			Expect(defaultDepWaiter.Wait(ctx)).To(Succeed())

			Expect(ingressIP).To(Equal("2.2.2.2"))
		})

		It("deletes service", func() {
			Expect(defaultDepWaiter.Destroy(ctx)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: expectedName}, &corev1.Service{})).To(BeNotFoundError())
		})

		It("waits for deletion service", func() {
			Expect(defaultDepWaiter.Destroy(ctx)).To(Succeed())
			Expect(defaultDepWaiter.WaitCleanup(ctx)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: expectedName}, &corev1.Service{})).To(BeNotFoundError())
		})
	}

	Context("when service is not in shoot namespace", func() {
		BeforeEach(func() {
			expected.Annotations = utils.MergeStringMaps(map[string]string{
				"foo":                          "bar",
				"networking.istio.io/exportTo": "*",
			}, netpolAnnotations())
		})

		assertService()
	})

	Context("when service is designed for shoots", func() {
		BeforeEach(func() {
			namespace = "shoot-" + expected.Namespace

			expected.Annotations = utils.MergeStringMaps(map[string]string{
				"foo":                          "bar",
				"networking.istio.io/exportTo": "*",
			}, shootNetpolAnnotations())
			expected.Namespace = namespace
		})

		assertService()
	})

	Context("when TopologyAwareRoutingEnabled=true", func() {
		BeforeEach(func() {
			values.TopologyAwareRoutingEnabled = true
		})

		When("runtime Kubernetes version is >= 1.32", func() {
			BeforeEach(func() {
				values.RuntimeKubernetesVersion = semver.MustParse("1.32.1")
			})

			It("should successfully deploy with expected kube-apiserver service annotation, label and spec field", func() {
				Expect(defaultDepWaiter.Deploy(ctx)).To(Succeed())

				actual := &corev1.Service{}
				Expect(c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: expectedName}, actual)).To(Succeed())

				Expect(actual.Spec.TrafficDistribution).To(PointTo(Equal(corev1.ServiceTrafficDistributionPreferClose)))
			})
		})

		When("runtime Kubernetes version is 1.31", func() {
			BeforeEach(func() {
				values.RuntimeKubernetesVersion = semver.MustParse("1.31.1")
			})

			It("should successfully deploy with expected kube-apiserver service annotation, label and spec field", func() {
				Expect(defaultDepWaiter.Deploy(ctx)).To(Succeed())

				actual := &corev1.Service{}
				Expect(c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: expectedName}, actual)).To(Succeed())

				Expect(actual.Spec.TrafficDistribution).To(PointTo(Equal(corev1.ServiceTrafficDistributionPreferClose)))
				Expect(actual.Labels).To(HaveKeyWithValue("endpoint-slice-hints.resources.gardener.cloud/consider", "true"))
			})
		})

		Context("when K8s version < 1.31", func() {
			BeforeEach(func() {
				values.RuntimeKubernetesVersion = semver.MustParse("1.30.3")
			})

			It("should successfully deploy with expected kube-apiserver service annotation, label and spec field", func() {
				Expect(defaultDepWaiter.Deploy(ctx)).To(Succeed())

				actual := &corev1.Service{}
				Expect(c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: expectedName}, actual)).To(Succeed())

				Expect(actual.Annotations).To(HaveKeyWithValue("service.kubernetes.io/topology-mode", "auto"))
				Expect(actual.Labels).To(HaveKeyWithValue("endpoint-slice-hints.resources.gardener.cloud/consider", "true"))
			})
		})
	})
})

func netpolAnnotations() map[string]string {
	return map[string]string{
		"networking.resources.gardener.cloud/from-all-garden-scrape-targets-allowed-ports": `[{"protocol":"TCP","port":443}]`,
		"networking.resources.gardener.cloud/namespace-selectors":                          `[{"matchLabels":{"gardener.cloud/role":"istio-ingress"}},{"matchLabels":{"networking.gardener.cloud/access-target-apiserver":"allowed"}}]`,
	}
}

func shootNetpolAnnotations() map[string]string {
	return map[string]string{
		"networking.resources.gardener.cloud/from-all-scrape-targets-allowed-ports": `[{"protocol":"TCP","port":443}]`,
		"networking.resources.gardener.cloud/namespace-selectors":                   `[{"matchLabels":{"gardener.cloud/role":"istio-ingress"}},{"matchLabels":{"networking.gardener.cloud/access-target-apiserver":"allowed"}},{"matchLabels":{"kubernetes.io/metadata.name":"garden"}},{"matchExpressions":[{"key":"handler.exposureclass.gardener.cloud/name","operator":"Exists"}]},{"matchLabels":{"gardener.cloud/role":"extension"}}]`,
		"networking.resources.gardener.cloud/pod-label-selector-namespace-alias":    "all-shoots",
	}
}
