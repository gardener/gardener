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

package kubeapiserverexposure_test

import (
	"context"

	"github.com/Masterminds/semver"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/gardener/gardener/pkg/component"
	. "github.com/gardener/gardener/pkg/component/kubeapiserverexposure"
	"github.com/gardener/gardener/pkg/utils"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	retryfake "github.com/gardener/gardener/pkg/utils/retry/fake"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("#Service", func() {
	var (
		log logr.Logger
		ctx context.Context
		c   client.Client

		defaultDepWaiter component.DeployWaiter
		expected         *corev1.Service

		ingressIP        string
		clusterIP        string
		clusterIPFunc    func(string)
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
		clusterIPFunc = func(c string) { clusterIP = c }
		ingressIPFunc = func(c string) { ingressIP = c }

		expected = &corev1.Service{
			TypeMeta: metav1.TypeMeta{
				APIVersion: corev1.SchemeGroupVersion.String(),
				Kind:       "Service",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      expectedName,
				Namespace: namespace,
				Labels: map[string]string{
					"app":                                    "kubernetes",
					"role":                                   "apiserver",
					"core.gardener.cloud/apiserver-exposure": "gardener-managed",
				},
			},
			Spec: corev1.ServiceSpec{
				Type: corev1.ServiceTypeClusterIP,
				Ports: []corev1.ServicePort{{
					Name:       "kube-apiserver",
					Port:       443,
					Protocol:   corev1.ProtocolTCP,
					TargetPort: intstr.FromInt(443),
				}},
				Selector: map[string]string{
					"app":  "kubernetes",
					"role": "apiserver",
				},
				ClusterIP: "1.1.1.1",
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
			&ServiceValues{
				AnnotationsFunc: func() map[string]string { return map[string]string{"foo": "bar"} },
				NamePrefix:      namePrefix,
			},
			func() client.ObjectKey { return sniServiceObjKey },
			&retryfake.Ops{MaxAttempts: 1},
			clusterIPFunc,
			ingressIPFunc,
			"",
		)
	})

	var assertService = func() {
		It("deploys service", func() {
			Expect(defaultDepWaiter.Deploy(ctx)).To(Succeed())

			actual := &corev1.Service{}
			Expect(c.Get(ctx, kubernetesutils.Key(namespace, expectedName), actual)).To(Succeed())

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

			Expect(c.Get(ctx, kubernetesutils.Key(namespace, expectedName), &corev1.Service{})).To(BeNotFoundError())
		})

		It("waits for deletion service", func() {
			Expect(defaultDepWaiter.Destroy(ctx)).To(Succeed())
			Expect(defaultDepWaiter.WaitCleanup(ctx)).To(Succeed())

			Expect(c.Get(ctx, kubernetesutils.Key(namespace, expectedName), &corev1.Service{})).To(BeNotFoundError())
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

	Describe("#Deploy", func() {
		Context("when TopologyAwareRoutingEnabled=true", func() {
			It("should successfully deploy with expected kube-apiserver service annotations and labels", func() {
				defaultDepWaiter = NewService(
					log,
					c,
					namespace,
					&ServiceValues{
						AnnotationsFunc:             func() map[string]string { return map[string]string{"foo": "bar"} },
						NamePrefix:                  namePrefix,
						TopologyAwareRoutingEnabled: true,
						RuntimeKubernetesVersion:    semver.MustParse("1.26.1"),
					},
					func() client.ObjectKey { return sniServiceObjKey },
					&retryfake.Ops{MaxAttempts: 1},
					clusterIPFunc,
					ingressIPFunc,
					"",
				)

				Expect(defaultDepWaiter.Deploy(ctx)).To(Succeed())

				actual := &corev1.Service{}
				Expect(c.Get(ctx, kubernetesutils.Key(namespace, expectedName), actual)).To(Succeed())

				expected.Annotations = map[string]string{
					"foo":                          "bar",
					"networking.istio.io/exportTo": "*",
					"networking.resources.gardener.cloud/from-all-scrape-targets-allowed-ports": `[{"protocol":"TCP","port":443}]`,
					"networking.resources.gardener.cloud/namespace-selectors":                   `[{"matchLabels":{"gardener.cloud/role":"istio-ingress"}},{"matchLabels":{"networking.gardener.cloud/access-target-apiserver":"allowed"}}]`,
					"service.kubernetes.io/topology-aware-hints":                                "auto",
				}
				expected.Labels = map[string]string{
					"app": "kubernetes",
					"endpoint-slice-hints.resources.gardener.cloud/consider": "true",
					"core.gardener.cloud/apiserver-exposure":                 "gardener-managed",
					"role":                                                   "apiserver",
				}
				expected.Spec.Type = corev1.ServiceTypeClusterIP
				Expect(actual).To(DeepEqual(expected))
			})
		})

		Context("when cluster IP is provided", func() {
			clusterIP := "1.2.3.4"

			JustBeforeEach(func() {
				defaultDepWaiter = NewService(
					log,
					c,
					namespace,
					&ServiceValues{
						AnnotationsFunc: func() map[string]string { return nil },
						NamePrefix:      namePrefix,
					},
					func() client.ObjectKey { return sniServiceObjKey },
					&retryfake.Ops{MaxAttempts: 1},
					clusterIPFunc,
					ingressIPFunc,
					clusterIP,
				)
			})

			Context("when cluster IP is already set", func() {
				It("should not change existing cluster IP", func() {
					Expect(defaultDepWaiter.Deploy(ctx)).To(Succeed())

					actual := &corev1.Service{}
					Expect(c.Get(ctx, kubernetesutils.Key(namespace, expectedName), actual)).To(Succeed())

					Expect(actual.Spec.ClusterIP).To(Equal(expected.Spec.ClusterIP))
				})
			})

			Context("when cluster IP is not yet set", func() {
				JustBeforeEach(func() {
					Expect(c.Delete(ctx, expected)).To(Succeed())
				})

				It("should successfully deploy with expected clusterIP", func() {
					Expect(defaultDepWaiter.Deploy(ctx)).To(Succeed())

					actual := &corev1.Service{}
					Expect(c.Get(ctx, kubernetesutils.Key(namespace, expectedName), actual)).To(Succeed())

					Expect(actual.Spec.ClusterIP).To(Equal(clusterIP))
				})
			})
		})
	})
})

func netpolAnnotations() map[string]string {
	return map[string]string{
		"networking.resources.gardener.cloud/from-all-scrape-targets-allowed-ports": `[{"protocol":"TCP","port":443}]`,
		"networking.resources.gardener.cloud/namespace-selectors":                   `[{"matchLabels":{"gardener.cloud/role":"istio-ingress"}},{"matchLabels":{"networking.gardener.cloud/access-target-apiserver":"allowed"}}]`,
	}
}

func shootNetpolAnnotations() map[string]string {
	return map[string]string{
		"networking.resources.gardener.cloud/from-all-scrape-targets-allowed-ports": `[{"protocol":"TCP","port":443}]`,
		"networking.resources.gardener.cloud/namespace-selectors":                   `[{"matchLabels":{"gardener.cloud/role":"istio-ingress"}},{"matchLabels":{"networking.gardener.cloud/access-target-apiserver":"allowed"}},{"matchLabels":{"kubernetes.io/metadata.name":"garden"}},{"matchExpressions":[{"key":"handler.exposureclass.gardener.cloud/name","operator":"Exists"}]},{"matchLabels":{"gardener.cloud/role":"extension"}}]`,
		"networking.resources.gardener.cloud/pod-label-selector-namespace-alias":    "all-shoots",
	}
}
