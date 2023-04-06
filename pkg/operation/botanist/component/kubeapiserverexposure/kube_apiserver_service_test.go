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
	})

	JustBeforeEach(func() {
		Expect(c.Create(ctx, expected)).To(Succeed())
		expected.ResourceVersion = "2"

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
			true,
			"",
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
			expected.Annotations = utils.MergeStringMaps(map[string]string{
				"foo": "bar",
			}, netpolAnnotations())
		})

		assertDisabledSNI()
	})

	Context("SNI being disabled", func() {
		BeforeEach(func() {
			sniPhase = component.PhaseDisabling
			expected.Annotations = utils.MergeStringMaps(map[string]string{
				"foo":                          "bar",
				"networking.istio.io/exportTo": "*",
			}, netpolAnnotations())
			expected.Spec.Type = corev1.ServiceTypeLoadBalancer
			expected.Labels["core.gardener.cloud/apiserver-exposure"] = "gardener-managed"
		})

		assertDisabledSNI()
	})

	Context("SNI enabled", func() {
		BeforeEach(func() {
			sniPhase = component.PhaseEnabled
			expected.Annotations = utils.MergeStringMaps(map[string]string{
				"foo":                          "bar",
				"networking.istio.io/exportTo": "*",
			}, netpolAnnotations())
			expected.Spec.Type = corev1.ServiceTypeClusterIP
			expected.Labels["core.gardener.cloud/apiserver-exposure"] = "gardener-managed"
		})

		assertEnabledSNI()
	})

	Context("SNI being enabled", func() {
		BeforeEach(func() {
			sniPhase = component.PhaseEnabling
			expected.Annotations = utils.MergeStringMaps(map[string]string{
				"foo":                          "bar",
				"networking.istio.io/exportTo": "*",
			}, netpolAnnotations())
			expected.Spec.Type = corev1.ServiceTypeLoadBalancer
		})

		assertEnabledSNI()
	})

	Context("when service is designed for shoots", func() {
		BeforeEach(func() {
			namespace := "shoot-" + expected.Namespace

			sniPhase = component.PhaseEnabling
			serviceObjKey = client.ObjectKey{Name: serviceObjKey.Name, Namespace: namespace}
			expected.Annotations = utils.MergeStringMaps(map[string]string{
				"foo":                          "bar",
				"networking.istio.io/exportTo": "*",
			}, shootNetpolAnnotations())
			expected.Namespace = namespace
			expected.Spec.Type = corev1.ServiceTypeLoadBalancer
		})

		assertEnabledSNI()
	})

	Describe("#Deploy", func() {
		Context("when TopologyAwareRoutingEnabled=true", func() {
			It("should successfully deploy with expected kube-apiserver service annotations and labels", func() {
				sniPhase = component.PhaseEnabled
				defaultDepWaiter = NewService(
					log,
					c,
					&ServiceValues{
						AnnotationsFunc:             func() map[string]string { return map[string]string{"foo": "bar"} },
						SNIPhase:                    sniPhase,
						TopologyAwareRoutingEnabled: true,
					},
					func() client.ObjectKey { return serviceObjKey },
					func() client.ObjectKey { return sniServiceObjKey },
					&retryfake.Ops{MaxAttempts: 1},
					clusterIPFunc,
					ingressIPFunc,
					false,
					"",
				)

				Expect(defaultDepWaiter.Deploy(ctx)).To(Succeed())

				actual := &corev1.Service{}
				Expect(c.Get(ctx, serviceObjKey, actual)).To(Succeed())

				expected.Annotations = map[string]string{
					"foo":                          "bar",
					"networking.istio.io/exportTo": "*",
					"networking.resources.gardener.cloud/from-policy-pod-label-selector": "all-scrape-targets",
					"networking.resources.gardener.cloud/from-policy-allowed-ports":      `[{"protocol":"TCP","port":443}]`,
					"networking.resources.gardener.cloud/from-world-to-ports":            `[{"protocol":"TCP","port":443}]`,
					"networking.resources.gardener.cloud/namespace-selectors":            `[{"matchLabels":{"gardener.cloud/role":"istio-ingress"}}]`,
					"service.kubernetes.io/topology-aware-hints":                         "auto",
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
					&ServiceValues{
						AnnotationsFunc: func() map[string]string { return nil },
					},
					func() client.ObjectKey { return serviceObjKey },
					func() client.ObjectKey { return sniServiceObjKey },
					&retryfake.Ops{MaxAttempts: 1},
					clusterIPFunc,
					ingressIPFunc,
					false,
					clusterIP,
				)
			})

			Context("when cluster IP is already set", func() {
				It("should not change existing cluster IP", func() {
					Expect(defaultDepWaiter.Deploy(ctx)).To(Succeed())

					actual := &corev1.Service{}
					Expect(c.Get(ctx, serviceObjKey, actual)).To(Succeed())

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
					Expect(c.Get(ctx, serviceObjKey, actual)).To(Succeed())

					Expect(actual.Spec.ClusterIP).To(Equal(clusterIP))
				})
			})
		})
	})
})

func netpolAnnotations() map[string]string {
	return map[string]string{
		"networking.resources.gardener.cloud/from-policy-allowed-ports":      `[{"protocol":"TCP","port":443}]`,
		"networking.resources.gardener.cloud/from-policy-pod-label-selector": "all-scrape-targets",
		"networking.resources.gardener.cloud/from-world-to-ports":            `[{"protocol":"TCP","port":443}]`,
		"networking.resources.gardener.cloud/namespace-selectors":            `[{"matchLabels":{"gardener.cloud/role":"istio-ingress"}}]`,
	}
}

func shootNetpolAnnotations() map[string]string {
	return map[string]string{
		"networking.resources.gardener.cloud/from-policy-allowed-ports":          `[{"protocol":"TCP","port":443}]`,
		"networking.resources.gardener.cloud/from-policy-pod-label-selector":     "all-scrape-targets",
		"networking.resources.gardener.cloud/from-world-to-ports":                `[{"protocol":"TCP","port":443}]`,
		"networking.resources.gardener.cloud/namespace-selectors":                `[{"matchLabels":{"kubernetes.io/metadata.name":"garden"}},{"matchLabels":{"gardener.cloud/role":"istio-ingress"}},{"matchExpressions":[{"key":"handler.exposureclass.gardener.cloud/name","operator":"Exists"}]},{"matchLabels":{"gardener.cloud/role":"extension"}}]`,
		"networking.resources.gardener.cloud/pod-label-selector-namespace-alias": "all-shoots",
	}
}

var _ = Describe("#KubeAPIServerService", func() {
	var (
		ctx context.Context
		c   client.Client

		serviceObjKey   client.ObjectKey
		defaultDeployer component.Deployer
		namespace       string
		expected        *corev1.Service
	)

	BeforeEach(func() {
		ctx = context.TODO()

		s := runtime.NewScheme()
		Expect(corev1.AddToScheme(s)).To(Succeed())
		c = fake.NewClientBuilder().WithScheme(s).Build()

		namespace = "foobar"
		serviceObjKey = client.ObjectKey{Name: "kube-apiserver", Namespace: namespace}
		expected = &corev1.Service{
			TypeMeta: metav1.TypeMeta{
				APIVersion: corev1.SchemeGroupVersion.String(),
				Kind:       "Service",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kube-apiserver",
				Namespace: namespace,
				Labels: map[string]string{
					"app":  "kubernetes",
					"role": "apiserver",
				},
			},
			Spec: corev1.ServiceSpec{
				Type:         corev1.ServiceTypeExternalName,
				ExternalName: "kubernetes.default.svc.cluster.local",
			},
		}
	})

	JustBeforeEach(func() {
		defaultDeployer = NewKubeAPIServerService(
			c,
			namespace,
		)
	})

	Context("Deploy", func() {
		It("should create the expected service", func() {
			Expect(defaultDeployer.Deploy(ctx)).To(Succeed())

			actual := &corev1.Service{}
			Expect(c.Get(ctx, serviceObjKey, actual)).To(Succeed())
			Expect(actual.Annotations).To(DeepEqual(expected.Annotations))
			Expect(actual.Labels).To(DeepEqual(expected.Labels))
			Expect(actual.Spec).To(DeepEqual(expected.Spec))
		})
	})

	Context("Destroy", func() {
		It("should delete the ingress object", func() {
			Expect(c.Create(ctx, expected)).To(Succeed())
			Expect(c.Get(ctx, serviceObjKey, &corev1.Service{})).To(Succeed())

			Expect(defaultDeployer.Destroy(ctx)).To(Succeed())

			Expect(c.Get(ctx, serviceObjKey, &corev1.Service{})).To(BeNotFoundError())
		})
	})
})
