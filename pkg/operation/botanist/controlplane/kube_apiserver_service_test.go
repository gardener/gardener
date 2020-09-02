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

package controlplane_test

import (
	"context"

	cr "github.com/gardener/gardener/pkg/chartrenderer"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	. "github.com/gardener/gardener/pkg/operation/botanist/controlplane"
	"github.com/sirupsen/logrus"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/version"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	. "github.com/gardener/gardener/test/gomega"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("#KubeAPIServerService", func() {
	const (
		deployNS    = "test-chart-namespace"
		serviceName = "test-deploy"
	)
	var (
		ca                 kubernetes.ChartApplier
		ctx                context.Context
		c                  client.Client
		expected           *corev1.Service
		log                logrus.FieldLogger
		defaultDepWaiter   component.DeployWaiter
		ingressIP          string
		clusterIP          string
		enableSNI          bool
		enableKonnectivity bool
		clusterIPFunc      func(string)
		ingressIPFunc      func(string)
		serviceObjKey      client.ObjectKey
		sniServiceObjKey   *client.ObjectKey
	)

	BeforeEach(func() {
		ctx = context.TODO()
		log = logger.NewNopLogger()

		s := runtime.NewScheme()
		Expect(corev1.AddToScheme(s)).NotTo(HaveOccurred())

		c = fake.NewFakeClientWithScheme(s)

		ingressIP = ""
		clusterIP = ""
		enableSNI = false
		enableKonnectivity = false
		serviceObjKey = client.ObjectKey{Name: serviceName, Namespace: deployNS}
		sniServiceObjKey = &client.ObjectKey{Name: "foo", Namespace: "bar"}

		clusterIPFunc = func(c string) { clusterIP = c }
		ingressIPFunc = func(c string) { ingressIP = c }

		expected = &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      serviceName,
				Namespace: deployNS,
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
			},
		}

		renderer := cr.NewWithServerVersion(&version.Info{})
		ca = kubernetes.NewChartApplier(
			renderer,
			kubernetes.NewApplier(c, meta.NewDefaultRESTMapper([]schema.GroupVersion{})),
		)
	})

	JustBeforeEach(func() {

		if enableSNI {
			defaultDepWaiter = NewKubeAPIService(
				&KubeAPIServiceValues{
					Annotations:               map[string]string{"foo": "bar"},
					KonnectivityTunnelEnabled: enableKonnectivity,
				},
				serviceObjKey,
				sniServiceObjKey,
				ca,
				chartsRoot(),
				log,
				c,
				&fakeOps{},
				clusterIPFunc,
				ingressIPFunc,
			)
		} else {
			defaultDepWaiter = NewKubeAPIService(
				&KubeAPIServiceValues{
					Annotations:               map[string]string{"foo": "bar"},
					KonnectivityTunnelEnabled: enableKonnectivity,
				},
				serviceObjKey,
				nil,
				ca,
				chartsRoot(),
				log,
				c,
				&fakeOps{},
				clusterIPFunc,
				ingressIPFunc,
			)
		}
	})

	Context("Konnectivity enabled", func() {
		BeforeEach(func() {
			enableKonnectivity = true
			expected.Annotations = map[string]string{"foo": "bar"}
			expected.Spec.Ports = append(expected.Spec.Ports, corev1.ServicePort{
				Name:       "konnectivity-server",
				Port:       8132,
				Protocol:   corev1.ProtocolTCP,
				TargetPort: intstr.FromInt(8132),
			})
		})

		It("deploys service", func() {
			Expect(defaultDepWaiter.Deploy(ctx)).NotTo(HaveOccurred())

			actual := &corev1.Service{}
			Expect(c.Get(ctx, serviceObjKey, actual)).NotTo(HaveOccurred())

			Expect(actual).To(DeepDerivativeEqual(expected))

			Expect(ingressIP).To(BeEmpty())
			Expect(clusterIP).To(BeEmpty())
		})

	})

	Context("SNI disabled", func() {
		BeforeEach(func() {
			enableSNI = false
			expected.Annotations = map[string]string{"foo": "bar"}
		})

		It("deploys service", func() {
			Expect(defaultDepWaiter.Deploy(ctx)).NotTo(HaveOccurred())

			actual := &corev1.Service{}
			Expect(c.Get(ctx, serviceObjKey, actual)).NotTo(HaveOccurred())

			Expect(actual).To(DeepDerivativeEqual(expected))

			Expect(ingressIP).To(BeEmpty())
			Expect(clusterIP).To(BeEmpty())
		})

		It("waits for service", func() {
			Expect(defaultDepWaiter.Deploy(ctx)).NotTo(HaveOccurred())

			expected.Status = corev1.ServiceStatus{
				LoadBalancer: corev1.LoadBalancerStatus{
					Ingress: []corev1.LoadBalancerIngress{{IP: "2.2.2.2"}},
				},
			}

			key, _ := client.ObjectKeyFromObject(expected)
			Expect(c.Get(ctx, key, expected)).NotTo(HaveOccurred())
			Expect(c.Status().Update(ctx, expected)).NotTo(HaveOccurred())

			Expect(defaultDepWaiter.Wait(ctx)).NotTo(HaveOccurred())

			Expect(ingressIP).To(Equal("2.2.2.2"))
		})

		It("fails to wait for service", func() {
			Expect(defaultDepWaiter.Deploy(ctx)).NotTo(HaveOccurred())

			Expect(defaultDepWaiter.Wait(ctx)).To(HaveOccurred())

			Expect(ingressIP).To(BeEmpty())
		})

		It("deletes service", func() {
			Expect(defaultDepWaiter.Destroy(ctx)).NotTo(HaveOccurred())

			Expect(c.Get(ctx, serviceObjKey, &corev1.Service{})).To(BeNotFoundError())
		})

		It("waits for deletion service", func() {
			Expect(defaultDepWaiter.Destroy(ctx)).NotTo(HaveOccurred())
			Expect(defaultDepWaiter.WaitCleanup(ctx)).NotTo(HaveOccurred())

			Expect(c.Get(ctx, serviceObjKey, &corev1.Service{})).To(BeNotFoundError())
		})
	})

	Context("SNI enabled", func() {
		BeforeEach(func() {
			enableSNI = true
			expected.Annotations = map[string]string{
				"foo":                          "bar",
				"networking.istio.io/exportTo": "*",
			}
			expected.Spec.Type = corev1.ServiceTypeClusterIP
			expected.Spec.ClusterIP = "1.1.1.1"
			expected.Labels["core.gardener.cloud/apiserver-exposure"] = "gardener-managed"
			Expect(c.Create(ctx, expected)).NotTo(HaveOccurred())
		})

		It("deploys service", func() {
			Expect(defaultDepWaiter.Deploy(ctx)).NotTo(HaveOccurred())

			expected.ResourceVersion = "2"
			actual := &corev1.Service{}
			Expect(c.Get(ctx, serviceObjKey, actual)).NotTo(HaveOccurred())

			Expect(actual).To(DeepDerivativeEqual(expected))
			Expect(clusterIP).To(Equal("1.1.1.1"))
		})

		It("waits for service", func() {
			sniService := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "bar",
				},
				Status: corev1.ServiceStatus{
					LoadBalancer: corev1.LoadBalancerStatus{
						Ingress: []corev1.LoadBalancerIngress{{IP: "2.2.2.2"}},
					},
				},
			}

			Expect(c.Create(ctx, sniService)).NotTo(HaveOccurred())

			Expect(defaultDepWaiter.Deploy(ctx)).NotTo(HaveOccurred())
			Expect(defaultDepWaiter.Wait(ctx)).NotTo(HaveOccurred())

			Expect(ingressIP).To(Equal("2.2.2.2"))
		})

		It("fails to wait for service", func() {
			Expect(defaultDepWaiter.Deploy(ctx)).NotTo(HaveOccurred())
			Expect(defaultDepWaiter.Wait(ctx)).To(HaveOccurred())

			Expect(ingressIP).To(BeEmpty())
		})

		It("deletes service", func() {
			Expect(defaultDepWaiter.Destroy(ctx)).NotTo(HaveOccurred())

			Expect(c.Get(ctx, serviceObjKey, &corev1.Service{})).To(BeNotFoundError())
		})

		It("waits for deletion service", func() {
			Expect(defaultDepWaiter.Destroy(ctx)).NotTo(HaveOccurred())
			Expect(defaultDepWaiter.WaitCleanup(ctx)).NotTo(HaveOccurred())

			Expect(c.Get(ctx, serviceObjKey, &corev1.Service{})).To(BeNotFoundError())
		})
	})
})
