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

package kubeapiserverexposure_test

import (
	"context"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/gardener/gardener/pkg/operation/botanist/component"
	. "github.com/gardener/gardener/pkg/operation/botanist/component/kubeapiserverexposure"
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
		sniPhase         component.Phase
		clusterIPFunc    func(string)
		ingressIPFunc    func(string)
		serviceObjKey    client.ObjectKey
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
		sniPhase = component.PhaseUnknown
		serviceObjKey = client.ObjectKey{Name: "test-deploy", Namespace: "test-namespace"}
		sniServiceObjKey = client.ObjectKey{Name: "foo", Namespace: "bar"}
		clusterIPFunc = func(c string) { clusterIP = c }
		ingressIPFunc = func(c string) { ingressIP = c }

		expected = &corev1.Service{
			TypeMeta: metav1.TypeMeta{
				APIVersion: corev1.SchemeGroupVersion.String(),
				Kind:       "Service",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      serviceObjKey.Name,
				Namespace: serviceObjKey.Namespace,
				Labels: map[string]string{
					"app":  "kubernetes",
					"role": "apiserver",
				},
			},
			Spec: corev1.ServiceSpec{
				Type: corev1.ServiceTypeLoadBalancer,
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
		Expect(c.Create(ctx, expected)).To(Succeed())
		expected.ResourceVersion = "2"
	})

	JustBeforeEach(func() {
		defaultDepWaiter = NewService(
			log,
			c,
			&ServiceValues{
				AnnotationsFunc: func() map[string]string { return map[string]string{"foo": "bar"} },
				SNIPhase:        sniPhase,
			},
			func() client.ObjectKey { return serviceObjKey },
			func() client.ObjectKey { return sniServiceObjKey },
			&retryfake.Ops{MaxAttempts: 1},
			clusterIPFunc,
			ingressIPFunc,
		)
	})

	var assertDisabledSNI = func() {
		It("deploys service", func() {
			Expect(defaultDepWaiter.Deploy(ctx)).To(Succeed())

			actual := &corev1.Service{}
			Expect(c.Get(ctx, serviceObjKey, actual)).To(Succeed())

			Expect(actual).To(DeepEqual(expected))

			Expect(ingressIP).To(BeEmpty())
			Expect(clusterIP).To(Equal("1.1.1.1"))
		})

		It("waits for service", func() {
			Expect(defaultDepWaiter.Deploy(ctx)).To(Succeed())

			key := client.ObjectKeyFromObject(expected)
			Expect(c.Get(ctx, key, expected)).To(Succeed())

			expected.Status = corev1.ServiceStatus{
				LoadBalancer: corev1.LoadBalancerStatus{
					Ingress: []corev1.LoadBalancerIngress{{IP: "3.3.3.3"}},
				},
			}
			Expect(c.Status().Update(ctx, expected)).To(Succeed())
			Expect(defaultDepWaiter.Wait(ctx)).To(Succeed())
			Expect(ingressIP).To(Equal("3.3.3.3"))
		})

		It("deletes service", func() {
			Expect(defaultDepWaiter.Destroy(ctx)).To(Succeed())

			Expect(c.Get(ctx, serviceObjKey, &corev1.Service{})).To(BeNotFoundError())
		})

		It("waits for deletion service", func() {
			Expect(defaultDepWaiter.Destroy(ctx)).To(Succeed())
			Expect(defaultDepWaiter.WaitCleanup(ctx)).To(Succeed())

			Expect(c.Get(ctx, serviceObjKey, &corev1.Service{})).To(BeNotFoundError())
		})
	}

	var assertEnabledSNI = func() {
		It("deploys service", func() {
			Expect(defaultDepWaiter.Deploy(ctx)).To(Succeed())

			actual := &corev1.Service{}
			Expect(c.Get(ctx, serviceObjKey, actual)).To(Succeed())

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

			Expect(c.Get(ctx, serviceObjKey, &corev1.Service{})).To(BeNotFoundError())
		})

		It("waits for deletion service", func() {
			Expect(defaultDepWaiter.Destroy(ctx)).To(Succeed())
			Expect(defaultDepWaiter.WaitCleanup(ctx)).To(Succeed())

			Expect(c.Get(ctx, serviceObjKey, &corev1.Service{})).To(BeNotFoundError())
		})
	}

	Context("SNI disabled", func() {
		BeforeEach(func() {
			sniPhase = component.PhaseDisabled
			expected.Annotations = map[string]string{
				"foo": "bar",
				"networking.resources.gardener.cloud/from-policy-pod-label-selector": "all-scrape-targets",
				"networking.resources.gardener.cloud/from-policy-allowed-ports":      `[{"protocol":"TCP","port":443}]`,
			}
		})

		assertDisabledSNI()
	})

	Context("SNI being disabled", func() {
		BeforeEach(func() {
			sniPhase = component.PhaseDisabling
			expected.Annotations = map[string]string{
				"foo":                          "bar",
				"networking.istio.io/exportTo": "*",
				"networking.resources.gardener.cloud/from-policy-pod-label-selector": "all-scrape-targets",
				"networking.resources.gardener.cloud/from-policy-allowed-ports":      `[{"protocol":"TCP","port":443}]`,
			}
			expected.Spec.Type = corev1.ServiceTypeLoadBalancer
			expected.Labels["core.gardener.cloud/apiserver-exposure"] = "gardener-managed"
		})

		assertDisabledSNI()
	})

	Context("SNI enabled", func() {
		BeforeEach(func() {
			sniPhase = component.PhaseEnabled
			expected.Annotations = map[string]string{
				"foo":                          "bar",
				"networking.istio.io/exportTo": "*",
				"networking.resources.gardener.cloud/from-policy-pod-label-selector": "all-scrape-targets",
				"networking.resources.gardener.cloud/from-policy-allowed-ports":      `[{"protocol":"TCP","port":443}]`,
			}
			expected.Spec.Type = corev1.ServiceTypeClusterIP
			expected.Labels["core.gardener.cloud/apiserver-exposure"] = "gardener-managed"
		})

		assertEnabledSNI()
	})

	Context("SNI being enabled", func() {
		BeforeEach(func() {
			sniPhase = component.PhaseEnabling
			expected.Annotations = map[string]string{
				"foo":                          "bar",
				"networking.istio.io/exportTo": "*",
				"networking.resources.gardener.cloud/from-policy-pod-label-selector": "all-scrape-targets",
				"networking.resources.gardener.cloud/from-policy-allowed-ports":      `[{"protocol":"TCP","port":443}]`,
			}
			expected.Spec.Type = corev1.ServiceTypeLoadBalancer
		})

		assertEnabledSNI()
	})
})
