// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package kubernetes_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	"go.uber.org/mock/gomock"
	admissionv1 "k8s.io/api/admission/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/rest"
	fakerestclient "k8s.io/client-go/rest/fake"
	clientcmdv1 "k8s.io/client-go/tools/clientcmd/api/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	kubernetesfake "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	. "github.com/gardener/gardener/pkg/utils/kubernetes"
	mockcorev1 "github.com/gardener/gardener/third_party/mock/client-go/core/v1"
	mockclient "github.com/gardener/gardener/third_party/mock/controller-runtime/client"
	mockio "github.com/gardener/gardener/third_party/mock/go/io"
)

var _ = Describe("kubernetes", func() {
	const (
		namespace = "namespace"
		name      = "name"
	)

	var (
		ctrl *gomock.Controller
		c    *mockclient.MockClient

		ctx     = context.TODO()
		fakeErr = fmt.Errorf("fake error")
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		c = mockclient.NewMockClient(ctrl)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#ObjectKeyFromSecretRef", func() {
		It("should return an ObjectKey with namespace and name set", func() {
			Expect(ObjectKeyFromSecretRef(corev1.SecretReference{Namespace: namespace, Name: name})).To(Equal(client.ObjectKey{Namespace: namespace, Name: name}))
		})
	})

	DescribeTable("#SetMetaDataLabel",
		func(labels map[string]string, key, value string, expectedLabels map[string]string) {
			original := &metav1.ObjectMeta{Labels: labels}
			modified := original.DeepCopy()

			SetMetaDataLabel(modified, key, value)
			modifiedWithOriginalLabels := modified.DeepCopy()
			modifiedWithOriginalLabels.Labels = labels
			Expect(modifiedWithOriginalLabels).To(Equal(original), "not only labels were modified")
			Expect(modified.Labels).To(Equal(expectedLabels))
		},
		Entry("nil labels", nil, "foo", "bar", map[string]string{"foo": "bar"}),
		Entry("non-nil non-conflicting labels", map[string]string{"bar": "baz"}, "foo", "bar", map[string]string{"bar": "baz", "foo": "bar"}),
		Entry("non-nil conflicting labels", map[string]string{"foo": "baz"}, "foo", "bar", map[string]string{"foo": "bar"}),
	)

	DescribeTable("#SetMetaDataAnnotation",
		func(annotations map[string]string, key, value string, expectedAnnotations map[string]string) {
			original := &metav1.ObjectMeta{Annotations: annotations}
			modified := original.DeepCopy()

			SetMetaDataAnnotation(modified, key, value)
			modifiedWithOriginalAnnotations := modified.DeepCopy()
			modifiedWithOriginalAnnotations.Annotations = annotations
			Expect(modifiedWithOriginalAnnotations).To(Equal(original), "not only annotations were modified")
			Expect(modified.Annotations).To(Equal(expectedAnnotations))
		},
		Entry("nil annotations", nil, "foo", "bar", map[string]string{"foo": "bar"}),
		Entry("non-nil non-conflicting annotations", map[string]string{"bar": "baz"}, "foo", "bar", map[string]string{"bar": "baz", "foo": "bar"}),
		Entry("non-nil conflicting annotations", map[string]string{"foo": "baz"}, "foo", "bar", map[string]string{"foo": "bar"}),
	)

	DescribeTable("#HasMetaDataAnnotation",
		func(annotations map[string]string, key, value string, result bool) {
			meta := &metav1.ObjectMeta{
				Annotations: annotations,
			}
			Expect(HasMetaDataAnnotation(meta, key, value)).To(BeIdenticalTo(result))
		},
		Entry("no annotations", map[string]string{}, "key", "value", false),
		Entry("matching annotation", map[string]string{"key": "value"}, "key", "value", true),
		Entry("no matching key", map[string]string{"key": "value"}, "key1", "value", false),
		Entry("no matching value", map[string]string{"key": "value"}, "key", "value1", false),
	)

	Describe("#WaitUntilResourceDeleted", func() {
		var (
			namespace = "bar"
			name      = "foo"
			key       = client.ObjectKey{Namespace: namespace, Name: name}
			configMap = &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: namespace,
					Name:      name,
				},
			}
		)

		It("should wait until the resource is deleted", func() {
			gomock.InOrder(
				c.EXPECT().Get(ctx, key, configMap),
				c.EXPECT().Get(ctx, key, configMap),
				c.EXPECT().Get(ctx, key, configMap).Return(apierrors.NewNotFound(schema.GroupResource{}, name)),
			)

			Expect(WaitUntilResourceDeleted(ctx, c, configMap, time.Microsecond)).To(Succeed())
		})

		It("should timeout", func() {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			gomock.InOrder(
				c.EXPECT().Get(ctx, key, configMap),
				c.EXPECT().Get(ctx, key, configMap).DoAndReturn(func(_ context.Context, _ client.ObjectKey, _ client.Object, _ ...client.GetOption) error {
					cancel()
					return nil
				}),
			)

			Expect(WaitUntilResourceDeleted(ctx, c, configMap, time.Microsecond)).To(HaveOccurred())
		})

		It("return an unexpected error", func() {
			expectedErr := fmt.Errorf("unexpected")
			c.EXPECT().Get(ctx, key, configMap).Return(expectedErr)

			err := WaitUntilResourceDeleted(ctx, c, configMap, time.Microsecond)
			Expect(err).To(HaveOccurred())
			Expect(err).To(BeIdenticalTo(expectedErr))
		})
	})

	Describe("#WaitUntilResourcesDeleted", func() {
		var (
			configMap = corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: namespace,
					Name:      name,
				},
			}
			configMapList *corev1.ConfigMapList
		)

		BeforeEach(func() {
			configMapList = &corev1.ConfigMapList{}
		})

		It("should wait until the resources are deleted w/ empty list", func() {
			c.EXPECT().List(ctx, configMapList).Return(nil)

			Expect(WaitUntilResourcesDeleted(ctx, c, configMapList, time.Microsecond)).To(Succeed())
		})

		It("should timeout w/ remaining elements", func() {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			c.EXPECT().List(ctx, configMapList).DoAndReturn(func(_ context.Context, _ client.ObjectList, _ ...client.ListOption) error {
				cancel()
				configMapList.Items = append(configMapList.Items, configMap)
				return nil
			})

			Expect(WaitUntilResourcesDeleted(ctx, c, configMapList, time.Microsecond)).To(HaveOccurred())
		})

		It("return an unexpected error", func() {
			expectedErr := fmt.Errorf("unexpected")
			c.EXPECT().List(ctx, configMapList).Return(expectedErr)

			Expect(WaitUntilResourcesDeleted(ctx, c, configMapList, time.Microsecond)).To(BeIdenticalTo(expectedErr))
		})
	})

	DescribeTable("#TruncateLabelValue",
		func(s, expected string) {
			Expect(TruncateLabelValue(s)).To(Equal(expected))
		},
		Entry("< 63 chars", "foo", "foo"),
		Entry("= 63 chars", strings.Repeat("a", 63), strings.Repeat("a", 63)),
		Entry("> 63 chars", strings.Repeat("a", 64), strings.Repeat("a", 63)))

	Describe("#GetLoadBalancerIngress", func() {
		var (
			key     = client.ObjectKey{Namespace: namespace, Name: name}
			service = &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
			}
		)

		It("should return an unexpected client error", func() {
			expectedErr := fmt.Errorf("unexpected")

			c.EXPECT().Get(ctx, key, gomock.AssignableToTypeOf(&corev1.Service{})).Return(expectedErr)

			_, err := GetLoadBalancerIngress(ctx, c, service)

			Expect(err).To(HaveOccurred())
			Expect(err).To(BeIdenticalTo(expectedErr))
		})

		It("should return an error because no ingresses found", func() {
			c.EXPECT().Get(ctx, key, gomock.AssignableToTypeOf(&corev1.Service{}))

			_, err := GetLoadBalancerIngress(ctx, c, service)

			Expect(err).To(MatchError("`.status.loadBalancer.ingress[]` has no elements yet, i.e. external load balancer has not been created"))
		})

		It("should return an ip address", func() {
			var (
				ctx        = context.TODO()
				expectedIP = "1.2.3.4"
			)

			c.EXPECT().Get(ctx, key, gomock.AssignableToTypeOf(&corev1.Service{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, service *corev1.Service, _ ...client.GetOption) error {
				service.Status.LoadBalancer.Ingress = []corev1.LoadBalancerIngress{{IP: expectedIP}}
				return nil
			})

			ingress, err := GetLoadBalancerIngress(ctx, c, service)

			Expect(ingress).To(Equal(expectedIP))
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return an hostname address", func() {
			var (
				ctx              = context.TODO()
				expectedHostname = "cluster.local"
			)

			c.EXPECT().Get(ctx, key, gomock.AssignableToTypeOf(&corev1.Service{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, service *corev1.Service, _ ...client.GetOption) error {
				service.Status.LoadBalancer.Ingress = []corev1.LoadBalancerIngress{{Hostname: expectedHostname}}
				return nil
			})

			ingress, err := GetLoadBalancerIngress(ctx, c, service)

			Expect(ingress).To(Equal(expectedHostname))
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return an error if neither ip nor hostname were set", func() {
			c.EXPECT().Get(ctx, key, gomock.AssignableToTypeOf(&corev1.Service{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, service *corev1.Service, _ ...client.GetOption) error {
				service.Status.LoadBalancer.Ingress = []corev1.LoadBalancerIngress{{}}
				return nil
			})

			_, err := GetLoadBalancerIngress(ctx, c, service)

			Expect(err).To(MatchError("`.status.loadBalancer.ingress[]` has an element which does neither contain `.ip` nor `.hostname`"))
		})
	})

	Describe("#LookupObject", func() {
		var (
			key       = client.ObjectKey{Namespace: namespace, Name: name}
			configMap = &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: namespace,
					Name:      name,
				},
			}

			apiReader *mockclient.MockClient
		)

		BeforeEach(func() {
			apiReader = mockclient.NewMockClient(ctrl)
		})

		It("should retrieve the obj when cached client can retrieve it", func() {
			c.EXPECT().Get(ctx, key, configMap)

			err := LookupObject(context.TODO(), c, apiReader, key, configMap)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return error when cached client fails with error other than NotFound error", func() {
			expectedErr := fmt.Errorf("unexpected")
			c.EXPECT().Get(ctx, key, configMap).Return(expectedErr)

			err := LookupObject(context.TODO(), c, apiReader, key, configMap)
			Expect(err).To(BeIdenticalTo(expectedErr))
		})

		It("should retrieve the obj using the apiReader when cached client fails with NotFound error", func() {
			c.EXPECT().Get(ctx, key, configMap).Return(apierrors.NewNotFound(schema.GroupResource{}, name))
			apiReader.EXPECT().Get(ctx, key, configMap)

			err := LookupObject(context.TODO(), c, apiReader, key, configMap)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	DescribeTable("#FeatureGatesToCommandLineParameter",
		func(fg map[string]bool, matcher gomegatypes.GomegaMatcher) {
			Expect(FeatureGatesToCommandLineParameter(fg)).To(matcher)
		},
		Entry("nil map", nil, BeEmpty()),
		Entry("empty map", map[string]bool{}, BeEmpty()),
		Entry("map with one entry", map[string]bool{"foo": true}, Equal("--feature-gates=foo=true")),
		Entry("map with multiple entries", map[string]bool{"foo": true, "bar": false, "baz": true}, Equal("--feature-gates=bar=false,baz=true,foo=true")),
	)

	DescribeTable("#MapStringBoolToCommandLineParameter",
		func(m map[string]bool, param string, matcher gomegatypes.GomegaMatcher) {
			Expect(MapStringBoolToCommandLineParameter(m, param)).To(matcher)
		},
		Entry("nil map", nil, "--some-param=", BeEmpty()),
		Entry("empty map", map[string]bool{}, "--some-param=", BeEmpty()),
		Entry("map with one entry", map[string]bool{"foo": true}, "--some-param=", Equal("--some-param=foo=true")),
		Entry("map with multiple entries", map[string]bool{"foo": true, "bar": false, "baz": true}, "--some-param=", Equal("--some-param=bar=false,baz=true,foo=true")),
	)

	var (
		port1 = corev1.ServicePort{
			Name:     "port1",
			Protocol: corev1.ProtocolTCP,
			Port:     1234,
		}
		port2 = corev1.ServicePort{
			Name:       "port2",
			Port:       1234,
			TargetPort: intstr.FromInt32(5678),
		}
		port3 = corev1.ServicePort{
			Name:       "port3",
			Protocol:   corev1.ProtocolTCP,
			Port:       1234,
			TargetPort: intstr.FromInt32(5678),
			NodePort:   9012,
		}
		desiredPorts = []corev1.ServicePort{port1, port2, port3}
	)

	DescribeTable("#ReconcileServicePorts",
		func(existingPorts []corev1.ServicePort, serviceType corev1.ServiceType, matcher gomegatypes.GomegaMatcher) {
			Expect(ReconcileServicePorts(existingPorts, desiredPorts, serviceType)).To(matcher)
		},
		Entry("existing ports is nil", nil, corev1.ServiceTypeLoadBalancer, ConsistOf(port1, port2, port3)),
		Entry("no existing ports", []corev1.ServicePort{}, corev1.ServiceTypeLoadBalancer, ConsistOf(port1, port2, port3)),
		Entry("existing but undesired ports", []corev1.ServicePort{{Name: "foo"}}, corev1.ServiceTypeLoadBalancer, ConsistOf(port1, port2, port3)),
		Entry("existing and desired ports when spec.type=LoadBalancer", []corev1.ServicePort{{Name: port1.Name, NodePort: 1337}}, corev1.ServiceTypeLoadBalancer, ConsistOf(corev1.ServicePort{Name: port1.Name, Protocol: port1.Protocol, Port: port1.Port, NodePort: 1337}, port2, port3)),
		Entry("existing and desired ports when spec.type=ClusterIP", []corev1.ServicePort{{Name: port1.Name, NodePort: 1337}}, corev1.ServiceTypeClusterIP, ConsistOf(port1, port2, port3)),
		Entry("existing and both desired and undesired ports", []corev1.ServicePort{{Name: "foo"}, {Name: port1.Name, NodePort: 1337}}, corev1.ServiceTypeLoadBalancer, ConsistOf(corev1.ServicePort{Name: port1.Name, Protocol: port1.Protocol, Port: port1.Port, NodePort: 1337}, port2, port3)),
	)

	Describe("#WaitUntilLoadBalancerIsReady", func() {
		var (
			k8sShootClient kubernetes.Interface
			key            = client.ObjectKey{Namespace: metav1.NamespaceSystem, Name: "load-balancer"}
			logger         = logr.Discard()
			scheme         *runtime.Scheme
		)

		BeforeEach(func() {
			scheme = runtime.NewScheme()
			Expect(corev1.AddToScheme(scheme)).To(Succeed())
			c.EXPECT().Scheme().Return(scheme).AnyTimes()
			k8sShootClient = kubernetesfake.NewClientSetBuilder().
				WithClient(c).
				Build()
		})

		It("should return nil when the Service has .status.loadBalancer.ingress[]", func() {
			var (
				svc = &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "load-balancer",
						Namespace: metav1.NamespaceSystem,
					},
					Status: corev1.ServiceStatus{
						LoadBalancer: corev1.LoadBalancerStatus{
							Ingress: []corev1.LoadBalancerIngress{
								{
									Hostname: "cluster.local",
								},
							},
						},
					},
				}
			)

			gomock.InOrder(
				c.EXPECT().Get(gomock.Any(), key, gomock.AssignableToTypeOf(&corev1.Service{})).DoAndReturn(
					func(_ context.Context, _ client.ObjectKey, obj *corev1.Service, _ ...client.GetOption) error {
						*obj = *svc
						return nil
					}),
			)

			actual, err := WaitUntilLoadBalancerIsReady(ctx, logger, k8sShootClient.Client(), metav1.NamespaceSystem, "load-balancer", 1*time.Second)
			Expect(err).NotTo(HaveOccurred())
			Expect(actual).To(Equal("cluster.local"))
		})

		It("should return err when the Service has no .status.loadBalancer.ingress[]", func() {
			var (
				svc = &corev1.Service{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "Service",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "load-balancer",
						Namespace: metav1.NamespaceSystem,
					},
					Status: corev1.ServiceStatus{},
				}
				event = corev1.Event{
					Source:         corev1.EventSource{Component: "service-controller"},
					Message:        "Error syncing load balancer: an error occurred",
					FirstTimestamp: metav1.NewTime(time.Date(2020, time.January, 15, 0, 0, 0, 0, time.UTC)),
					LastTimestamp:  metav1.NewTime(time.Date(2020, time.January, 15, 0, 0, 0, 0, time.UTC)),
					Count:          1,
					Type:           corev1.EventTypeWarning,
				}
			)

			gomock.InOrder(
				c.EXPECT().Get(gomock.Any(), key, gomock.AssignableToTypeOf(&corev1.Service{})).DoAndReturn(
					func(_ context.Context, _ client.ObjectKey, obj *corev1.Service, _ ...client.GetOption) error {
						*obj = *svc
						return nil
					}),
				c.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&corev1.EventList{}), gomock.Any()).DoAndReturn(
					func(_ context.Context, list *corev1.EventList, _ ...client.ListOption) error {
						list.Items = append(list.Items, event)
						return nil
					}),
			)

			actual, err := WaitUntilLoadBalancerIsReady(ctx, logger, k8sShootClient.Client(), metav1.NamespaceSystem, "load-balancer", 1*time.Second)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("-> Events:\n* service-controller reported"))
			Expect(err.Error()).To(ContainSubstring("Error syncing load balancer: an error occurred"))
			Expect(actual).To(BeEmpty())
		})
	})

	Describe("#FetchEventMessages", func() {
		var (
			reader     *mockclient.MockReader
			events     []corev1.Event
			serviceObj *corev1.Service
			scheme     *runtime.Scheme
		)

		BeforeEach(func() {
			reader = mockclient.NewMockReader(ctrl)

			events = []corev1.Event{
				{
					Source:         corev1.EventSource{Component: "service-controller"},
					Message:        "Error syncing load balancer: first error occurred",
					FirstTimestamp: metav1.NewTime(time.Date(2020, time.January, 15, 0, 0, 0, 0, time.UTC)),
					LastTimestamp:  metav1.NewTime(time.Date(2020, time.January, 15, 0, 0, 0, 0, time.UTC)),
					Count:          1,
					Type:           corev1.EventTypeWarning,
				},
				{
					Source:         corev1.EventSource{Component: "service-controller"},
					Message:        "Error syncing load balancer: second error occurred",
					FirstTimestamp: metav1.NewTime(time.Date(2020, time.January, 15, 1, 0, 0, 0, time.UTC)),
					LastTimestamp:  metav1.NewTime(time.Date(2020, time.January, 15, 1, 0, 0, 0, time.UTC)),
					Count:          1,
					Type:           corev1.EventTypeWarning,
				},
			}

			serviceObj = &corev1.Service{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "Service",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
			}
		})

		Context("when only objects of available scheme are used", func() {
			BeforeEach(func() {
				scheme = runtime.NewScheme()
				Expect(corev1.AddToScheme(scheme)).To(Succeed())
			})

			It("should return an event message with only the latest event", func() {
				var listOpts []client.ListOption
				gomock.InOrder(
					reader.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&corev1.EventList{}), gomock.Any()).DoAndReturn(
						func(_ context.Context, list *corev1.EventList, listOptions ...client.ListOption) error {
							list.Items = append(list.Items, events...)
							listOpts = listOptions
							return nil
						}),
				)

				msg, err := FetchEventMessages(ctx, scheme, reader, serviceObj, corev1.EventTypeWarning, 1)

				Expect(listOpts).To(ContainElement(client.MatchingFields{
					"involvedObject.apiVersion": serviceObj.APIVersion,
					"involvedObject.kind":       serviceObj.Kind,
					"involvedObject.name":       serviceObj.Name,
					"involvedObject.namespace":  serviceObj.Namespace,
					"type":                      corev1.EventTypeWarning,
				}))
				Expect(err).To(Not(HaveOccurred()))
				Expect(msg).To(ContainSubstring("-> Events:\n* service-controller reported"))
				Expect(msg).To(ContainSubstring("second error occurred"))
			})

			It("should return an event message with all events", func() {
				var listOpts []client.ListOption
				gomock.InOrder(
					reader.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&corev1.EventList{}), gomock.Any()).DoAndReturn(
						func(_ context.Context, list *corev1.EventList, listOptions ...client.ListOption) error {
							list.Items = append(list.Items, events...)
							listOpts = listOptions
							return nil
						}),
				)

				msg, err := FetchEventMessages(ctx, scheme, reader, serviceObj, corev1.EventTypeWarning, len(events))

				Expect(listOpts).To(ContainElement(client.MatchingFields{
					"involvedObject.apiVersion": serviceObj.APIVersion,
					"involvedObject.kind":       serviceObj.Kind,
					"involvedObject.name":       serviceObj.Name,
					"involvedObject.namespace":  serviceObj.Namespace,
					"type":                      corev1.EventTypeWarning,
				}))
				Expect(err).To(Not(HaveOccurred()))
				Expect(msg).To(ContainSubstring("-> Events:\n* service-controller reported"))
				Expect(msg).To(ContainSubstring("first error occurred"))
				Expect(msg).To(ContainSubstring("second error occurred"))
			})

			It("should not return a message because no events exist", func() {
				var listOpts []client.ListOption
				gomock.InOrder(
					reader.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&corev1.EventList{}), gomock.Any()).DoAndReturn(
						func(_ context.Context, _ *corev1.EventList, listOptions ...client.ListOption) error {
							listOpts = listOptions
							return nil
						}),
				)

				msg, err := FetchEventMessages(ctx, scheme, reader, serviceObj, corev1.EventTypeWarning, len(events))

				Expect(listOpts).To(ContainElement(client.MatchingFields{
					"involvedObject.apiVersion": serviceObj.APIVersion,
					"involvedObject.kind":       serviceObj.Kind,
					"involvedObject.name":       serviceObj.Name,
					"involvedObject.namespace":  serviceObj.Namespace,
					"type":                      corev1.EventTypeWarning,
				}))
				Expect(err).To(Not(HaveOccurred()))
				Expect(msg).To(BeEmpty())
			})

			It("should not return a message because an error occurred", func() {
				var listOpts []client.ListOption
				gomock.InOrder(
					reader.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&corev1.EventList{}), gomock.Any()).DoAndReturn(
						func(_ context.Context, _ *corev1.EventList, listOptions ...client.ListOption) error {
							listOpts = listOptions
							return errors.New("foo")
						}),
				)

				msg, err := FetchEventMessages(ctx, scheme, reader, serviceObj, corev1.EventTypeWarning, len(events))

				Expect(listOpts).To(ContainElement(client.MatchingFields{
					"involvedObject.apiVersion": serviceObj.APIVersion,
					"involvedObject.kind":       serviceObj.Kind,
					"involvedObject.name":       serviceObj.Name,
					"involvedObject.namespace":  serviceObj.Namespace,
					"type":                      corev1.EventTypeWarning,
				}))
				Expect(err).To(MatchError("error 'foo' occurred while fetching more details"))
				Expect(msg).To(BeEmpty())
			})
		})

		Context("when object type is not in provided scheme", func() {
			BeforeEach(func() {
				scheme = runtime.NewScheme()
				Expect(gardencorev1beta1.AddToScheme(scheme)).To(Succeed())
			})

			It("should not return a message because type kind is not in scheme", func() {
				msg, err := FetchEventMessages(ctx, scheme, reader, &corev1.Service{}, corev1.EventTypeWarning, len(events))

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to identify GVK for object"))
				Expect(msg).To(BeEmpty())
			})
		})
	})

	Describe("#MergeOwnerReferences", func() {
		It("should merge the new references into the list of existing references", func() {
			var (
				references = []metav1.OwnerReference{
					{
						UID: types.UID("1234"),
					},
				}
				newReferences = []metav1.OwnerReference{
					{
						UID: types.UID("1234"),
					},
					{
						UID: types.UID("1235"),
					},
				}
			)

			Expect(MergeOwnerReferences(references, newReferences...)).To(ConsistOf(newReferences))
		})
	})

	DescribeTable("#OwnedBy",
		func(obj client.Object, apiVersion, kind, name string, uid types.UID, matcher gomegatypes.GomegaMatcher) {
			Expect(OwnedBy(obj, apiVersion, kind, name, uid)).To(matcher)
		},
		Entry("no owner references", &corev1.Pod{}, "apiVersion", "kind", "name", types.UID("uid"), BeFalse()),
		Entry("owner not found", &corev1.Pod{ObjectMeta: metav1.ObjectMeta{OwnerReferences: []metav1.OwnerReference{{APIVersion: "different-apiVersion", Kind: "different-kind", Name: "different-name", UID: types.UID("different-uid")}}}}, "apiVersion", "kind", "name", types.UID("uid"), BeFalse()),
		Entry("owner found", &corev1.Pod{ObjectMeta: metav1.ObjectMeta{OwnerReferences: []metav1.OwnerReference{{APIVersion: "apiVersion", Kind: "kind", Name: "name", UID: types.UID("uid")}}}}, "apiVersion", "kind", "name", types.UID("uid"), BeTrue()),
	)

	Describe("#NewestObject", func() {
		var podList *corev1.PodList

		BeforeEach(func() {
			podList = &corev1.PodList{}
		})

		It("should return an error because the List() call failed", func() {
			c.EXPECT().List(ctx, podList).Return(fakeErr)

			obj, err := NewestObject(ctx, c, podList, nil)
			Expect(err).To(MatchError(fakeErr))
			Expect(obj).To(BeNil())
		})

		It("should return nil because the list does not contain items", func() {
			c.EXPECT().List(ctx, podList)

			obj, err := NewestObject(ctx, c, podList, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(obj).To(BeNil())
		})

		var (
			obj1 = &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "obj1", CreationTimestamp: metav1.Now()}}
			obj2 = &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "obj2", CreationTimestamp: metav1.Time{Time: time.Now().Add(+time.Hour)}}}
			obj3 = &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "obj3", CreationTimestamp: metav1.Time{Time: time.Now().Add(-time.Hour)}}}
		)

		It("should return the newest object w/o filter func", func() {
			c.EXPECT().List(ctx, podList).DoAndReturn(func(_ context.Context, list *corev1.PodList, _ ...client.ListOption) error {
				*list = corev1.PodList{Items: []corev1.Pod{*obj1, *obj2, *obj3}}
				return nil
			})

			obj, err := NewestObject(ctx, c, podList, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(obj).To(Equal(obj2))
			Expect(podList.Items).To(Equal([]corev1.Pod{*obj3, *obj1, *obj2}))
		})

		It("should return the newest object w/ filter func", func() {
			filterFn := func(o client.Object) bool {
				obj := o.(*corev1.Pod)
				return obj.Name != "obj2"
			}

			c.EXPECT().List(ctx, podList).DoAndReturn(func(_ context.Context, list *corev1.PodList, _ ...client.ListOption) error {
				*list = corev1.PodList{Items: []corev1.Pod{*obj1, *obj2, *obj3}}
				return nil
			})

			obj, err := NewestObject(ctx, c, podList, filterFn)
			Expect(err).NotTo(HaveOccurred())
			Expect(obj).To(Equal(obj1))
			Expect(podList.Items).To(Equal([]corev1.Pod{*obj3, *obj1}))
		})
	})

	Describe("#NewestPodForDeployment", func() {
		var (
			name      = "deployment-name"
			namespace = "deployment-namespace"
			uid       = types.UID("deployment-uid")
			labels    = map[string]string{"foo": "bar"}

			podTemplatehashKey = "pod-template-hash"
			rs1PodTemplateHash = "11111"
			rs2PodTemplateHash = "22222"
			rs3PodTemplateHash = "33333"
			rs1Labels          = map[string]string{
				"foo":              "bar",
				podTemplatehashKey: rs1PodTemplateHash,
			}
			rs2Labels = map[string]string{
				"foo":              "bar",
				podTemplatehashKey: rs2PodTemplateHash,
			}
			rs3Labels = map[string]string{
				"foo":              "bar",
				podTemplatehashKey: rs3PodTemplateHash,
			}

			rsListOptions = []any{
				client.InNamespace(namespace),
				client.MatchingLabels(labels),
			}
			podListOptions []any

			deployment = &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
					UID:       uid,
				},
				Spec: appsv1.DeploymentSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: labels,
					},
				},
			}

			replicaSet1 = &appsv1.ReplicaSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:              name + "-" + rs1PodTemplateHash,
					Labels:            rs1Labels,
					UID:               "replicaset1",
					CreationTimestamp: metav1.Now(),
					OwnerReferences: []metav1.OwnerReference{{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
						Name:       name,
						UID:        uid,
					}},
				},
				Spec: appsv1.ReplicaSetSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: rs1Labels,
					},
				},
			}
			replicaSet2 = &appsv1.ReplicaSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:              name + "-" + rs2PodTemplateHash,
					Labels:            rs2Labels,
					UID:               "replicaset2",
					CreationTimestamp: metav1.Time{Time: time.Now().Add(+time.Hour)},
					OwnerReferences: []metav1.OwnerReference{{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
						Name:       "other-deployment",
						UID:        "other-uid",
					}},
				},
				Spec: appsv1.ReplicaSetSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: rs2Labels,
					},
				},
			}
			replicaSet3 = &appsv1.ReplicaSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:              name + "-" + rs3PodTemplateHash,
					Labels:            rs3Labels,
					UID:               "replicaset3",
					CreationTimestamp: metav1.Time{Time: time.Now().Add(-time.Hour)},
					OwnerReferences: []metav1.OwnerReference{{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
						Name:       name,
						UID:        uid,
					}},
				},
				Spec: appsv1.ReplicaSetSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: rs3Labels,
					},
				},
			}

			pod1 = &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
				Name:              "pod1",
				UID:               "pod1",
				Labels:            rs1Labels,
				CreationTimestamp: metav1.Now(),
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion: "apps/v1",
					Kind:       "ReplicaSet",
					Name:       replicaSet1.Name,
					UID:        replicaSet1.UID,
				}},
			}}
			pod2 = &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
				Name:              "pod2",
				UID:               "pod2",
				Labels:            rs1Labels,
				CreationTimestamp: metav1.Time{Time: time.Now().Add(-time.Hour)},
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion: "apps/v1",
					Kind:       "ReplicaSet",
					Name:       replicaSet1.Name,
					UID:        replicaSet1.UID,
				}},
			}}
		)

		BeforeEach(func() {
			podSelector, err := metav1.LabelSelectorAsSelector(replicaSet1.Spec.Selector)
			Expect(err).NotTo(HaveOccurred())
			podListOptions = append(rsListOptions, client.MatchingLabelsSelector{Selector: podSelector})
		})

		It("should return an error because the newest ReplicaSet determination failed", func() {
			c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&appsv1.ReplicaSetList{}), rsListOptions...).Return(fakeErr)

			pod, err := NewestPodForDeployment(ctx, c, deployment)
			Expect(err).To(MatchError(fakeErr))
			Expect(pod).To(BeNil())
		})

		It("should return nil because no replica set found", func() {
			c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&appsv1.ReplicaSetList{}), rsListOptions...)

			pod, err := NewestPodForDeployment(ctx, c, deployment)
			Expect(err).NotTo(HaveOccurred())
			Expect(pod).To(BeNil())
		})

		It("should return an error because listing pods failed", func() {
			c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&appsv1.ReplicaSetList{}), rsListOptions...).DoAndReturn(func(_ context.Context, list *appsv1.ReplicaSetList, _ ...client.ListOption) error {
				*list = appsv1.ReplicaSetList{Items: []appsv1.ReplicaSet{*replicaSet1, *replicaSet2, *replicaSet3}}
				return nil
			})
			c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&corev1.PodList{}), podListOptions...).Return(fakeErr)

			pod, err := NewestPodForDeployment(ctx, c, deployment)
			Expect(err).To(MatchError(fakeErr))
			Expect(pod).To(BeNil())
		})

		It("should return an error because the replicasSet has no pod selector", func() {
			rs := &appsv1.ReplicaSet{ObjectMeta: metav1.ObjectMeta{
				Name:              "rs",
				Labels:            rs1Labels,
				UID:               "rs",
				CreationTimestamp: metav1.Now(),
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
					Name:       name,
					UID:        uid,
				}},
			}}
			rsError := fmt.Errorf("no pod selector specified in replicaSet %s/%s", rs.Namespace, rs.Name)

			c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&appsv1.ReplicaSetList{}), rsListOptions...).DoAndReturn(func(_ context.Context, list *appsv1.ReplicaSetList, _ ...client.ListOption) error {
				*list = appsv1.ReplicaSetList{Items: []appsv1.ReplicaSet{*rs}}
				return nil
			})

			pod, err := NewestPodForDeployment(ctx, c, deployment)
			Expect(err).To(MatchError(rsError))
			Expect(pod).To(BeNil())
		})

		It("should return an error because the replicasSet has no matchLabels or matchExpressions", func() {
			rs := &appsv1.ReplicaSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "rs",
					Labels:            rs1Labels,
					UID:               "rs",
					CreationTimestamp: metav1.Now(),
					OwnerReferences: []metav1.OwnerReference{{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
						Name:       name,
						UID:        uid,
					}},
				},
				Spec: appsv1.ReplicaSetSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels:      map[string]string{},
						MatchExpressions: []metav1.LabelSelectorRequirement{},
					}},
			}
			rsError := fmt.Errorf("no matchLabels or matchExpressions specified in replicaSet %s/%s", rs.Namespace, rs.Name)

			c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&appsv1.ReplicaSetList{}), rsListOptions...).DoAndReturn(func(_ context.Context, list *appsv1.ReplicaSetList, _ ...client.ListOption) error {
				*list = appsv1.ReplicaSetList{Items: []appsv1.ReplicaSet{*rs}}
				return nil
			})

			pod, err := NewestPodForDeployment(ctx, c, deployment)
			Expect(err).To(MatchError(rsError))
			Expect(pod).To(BeNil())
		})

		It("should return an error because the matchExpressions is invalid", func() {
			rs := &appsv1.ReplicaSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "rs",
					Labels:            rs1Labels,
					UID:               "rs",
					CreationTimestamp: metav1.Now(),
					OwnerReferences: []metav1.OwnerReference{{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
						Name:       name,
						UID:        uid,
					}},
				},
				Spec: appsv1.ReplicaSetSpec{
					Selector: &metav1.LabelSelector{
						MatchExpressions: []metav1.LabelSelectorRequirement{{Key: "foo", Operator: metav1.LabelSelectorOpIn, Values: []string{}}},
					}},
			}

			c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&appsv1.ReplicaSetList{}), rsListOptions...).DoAndReturn(func(_ context.Context, list *appsv1.ReplicaSetList, _ ...client.ListOption) error {
				*list = appsv1.ReplicaSetList{Items: []appsv1.ReplicaSet{*rs}}
				return nil
			})

			pod, err := NewestPodForDeployment(ctx, c, deployment)
			Expect(err).To(MatchError(ContainSubstring("for 'in', 'notin' operators, values set can't be empty")))
			Expect(pod).To(BeNil())
		})

		It("should return nil because no pod was found", func() {
			c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&appsv1.ReplicaSetList{}), rsListOptions...).DoAndReturn(func(_ context.Context, list *appsv1.ReplicaSetList, _ ...client.ListOption) error {
				*list = appsv1.ReplicaSetList{Items: []appsv1.ReplicaSet{*replicaSet1, *replicaSet2, *replicaSet3}}
				return nil
			})
			c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&corev1.PodList{}), podListOptions...)

			pod, err := NewestPodForDeployment(ctx, c, deployment)
			Expect(err).NotTo(HaveOccurred())
			Expect(pod).To(BeNil())
		})

		It("should return the newest pod", func() {
			c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&appsv1.ReplicaSetList{}), rsListOptions...).DoAndReturn(func(_ context.Context, list *appsv1.ReplicaSetList, _ ...client.ListOption) error {
				*list = appsv1.ReplicaSetList{Items: []appsv1.ReplicaSet{*replicaSet1, *replicaSet2, *replicaSet3}}
				return nil
			})
			c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&corev1.PodList{}), podListOptions...).DoAndReturn(func(_ context.Context, list *corev1.PodList, _ ...client.ListOption) error {
				*list = corev1.PodList{Items: []corev1.Pod{*pod1, *pod2}}
				return nil
			})

			pod, err := NewestPodForDeployment(ctx, c, deployment)
			Expect(err).NotTo(HaveOccurred())
			Expect(pod).To(Equal(pod1))
		})
	})

	Describe("#MostRecentCompleteLogs", func() {
		var (
			pods   *mockcorev1.MockPodInterface
			body   *mockio.MockReadCloser
			client *http.Client

			pod           *corev1.Pod
			podName       = "pod"
			containerName = "container"
		)

		BeforeEach(func() {
			pods = mockcorev1.NewMockPodInterface(ctrl)
			body = mockio.NewMockReadCloser(ctrl)
			client = fakerestclient.CreateHTTPClient(func(_ *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusOK, Body: body}, nil
			})

			pod = &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: podName}}
		})

		It("should return an error if the log retrieval failed", func() {
			gomock.InOrder(
				pods.EXPECT().GetLogs(podName, gomock.AssignableToTypeOf(&corev1.PodLogOptions{})).Return(rest.NewRequestWithClient(&url.URL{}, "", rest.ClientContentConfig{}, client)),
				body.EXPECT().Read(gomock.Any()).Return(0, fakeErr),
				body.EXPECT().Close(),
			)

			actual, err := MostRecentCompleteLogs(ctx, pods, pod, containerName, nil, nil)
			Expect(err).To(MatchError(fakeErr))
			Expect(actual).To(BeEmpty())
		})

		DescribeTable("#OwnedBy",
			func(containerStatuses []corev1.ContainerStatus, containerName string, previous bool) {
				var (
					headBytes int64 = 1024
					tailLines int64 = 1337
					logs            = []byte("logs")
				)

				pod.Status.ContainerStatuses = containerStatuses

				tailLineOptions := &corev1.PodLogOptions{
					Container: containerName,
					TailLines: &tailLines,
					Previous:  previous,
				}

				bytesLimitOptions := &corev1.PodLogOptions{
					Container:  containerName,
					LimitBytes: &headBytes,
					Previous:   previous,
				}

				gomock.InOrder(
					pods.EXPECT().GetLogs(podName, tailLineOptions).Return(rest.NewRequestWithClient(&url.URL{}, "", rest.ClientContentConfig{}, client)),
					body.EXPECT().Read(gomock.Any()).DoAndReturn(func(data []byte) (int, error) {
						copy(data, logs)
						return len(logs), io.EOF
					}),
					body.EXPECT().Close(),

					pods.EXPECT().GetLogs(podName, bytesLimitOptions).Return(rest.NewRequestWithClient(&url.URL{}, "", rest.ClientContentConfig{}, client)),
					body.EXPECT().Read(gomock.Any()).DoAndReturn(func(data []byte) (int, error) {
						copy(data, logs)
						return len(logs), io.EOF
					}),
					body.EXPECT().Close(),
				)

				actual, err := MostRecentCompleteLogs(ctx, pods, pod, containerName, &tailLines, &headBytes)
				Expect(err).NotTo(HaveOccurred())
				Expect(actual).To(Equal(fmt.Sprintf("%s\n...\n%s", logs, logs)))
			},

			Entry("w/o container name, logs of current  container", []corev1.ContainerStatus{{State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{}}}}, "", false),
			Entry("w/o container name, logs of previous container", []corev1.ContainerStatus{{State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}}}, "", true),
			Entry("w/  container name, logs of current  container", []corev1.ContainerStatus{{}, {Name: containerName, State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{}}}}, containerName, false),
			Entry("w/  container name, logs of previous container", []corev1.ContainerStatus{{}, {Name: containerName, State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}}}, containerName, true),
		)
	})

	Describe("#NewKubeconfig", func() {
		var (
			contextName = "context"
			server      = "server"
			caCert      = []byte("ca crt")
			cluster     = clientcmdv1.Cluster{
				Server:                   server,
				CertificateAuthorityData: caCert,
			}
			authInfo = clientcmdv1.AuthInfo{Token: "foo"}
		)

		It("should return the expected kubeconfig", func() {
			Expect(NewKubeconfig(contextName, cluster, authInfo)).To(Equal(&clientcmdv1.Config{
				CurrentContext: contextName,
				Clusters: []clientcmdv1.NamedCluster{{
					Name: contextName,
					Cluster: clientcmdv1.Cluster{
						Server:                   `https://` + server,
						CertificateAuthorityData: caCert,
					},
				}},
				AuthInfos: []clientcmdv1.NamedAuthInfo{{
					Name:     contextName,
					AuthInfo: authInfo,
				}},
				Contexts: []clientcmdv1.NamedContext{{
					Name: contextName,
					Context: clientcmdv1.Context{
						Cluster:  contextName,
						AuthInfo: contextName,
					},
				}},
			}))
		})
	})

	DescribeTable("#ObjectKeyForCreateWebhooks",
		func(obj client.Object, req admission.Request, expectedKey client.ObjectKey) {
			Expect(ObjectKeyForCreateWebhooks(obj, req)).To(Equal(expectedKey))
		},

		Entry("object w/o namespace in object with generateName",
			&corev1.Pod{ObjectMeta: metav1.ObjectMeta{GenerateName: "foo"}},
			admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{Namespace: "bar"}},
			client.ObjectKey{Namespace: "bar", Name: "foo"},
		),
		Entry("object w/o namespace in object with name",
			&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "foo"}},
			admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{Namespace: "bar"}},
			client.ObjectKey{Namespace: "bar", Name: "foo"},
		),
		Entry("object w/ namespace with generateName",
			&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: "bar", GenerateName: "foo"}},
			admission.Request{},
			client.ObjectKey{Namespace: "bar", Name: "foo"},
		),
		Entry("object w/ namespace with name",
			&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: "bar", Name: "foo"}},
			admission.Request{},
			client.ObjectKey{Namespace: "bar", Name: "foo"},
		),
		Entry("non-namespaced object with generateName",
			&corev1.Node{ObjectMeta: metav1.ObjectMeta{GenerateName: "foo"}},
			admission.Request{},
			client.ObjectKey{Namespace: "", Name: "foo"},
		),
		Entry("non-namespaced object with name",
			&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "foo"}},
			admission.Request{},
			client.ObjectKey{Namespace: "", Name: "foo"},
		),
	)

	Describe("#ClientCertificateFromRESTConfig", func() {
		var (
			config *rest.Config

			certPEM = []byte(`-----BEGIN CERTIFICATE-----
MIIDBDCCAeygAwIBAgIUXutuW//tcCBAR2BKjz1N9xNosNwwDQYJKoZIhvcNAQEL
BQAwGjEYMBYGA1UEAxMPbmV3LW1pbmlrdWJlLWNhMB4XDTIyMDQxMjA3MDcwMFoX
DTI3MDQxMTA3MDcwMFowGjEYMBYGA1UEAxMPbmV3LW1pbmlrdWJlLWNhMIIBIjAN
BgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAsiOi5NGvtPtJLD4FfMUne1KcgtKs
o91WdriOJWF6mfWiB2fnbMS8EaKaU4AMXyrQpn6neZTDXeH5DOXhiQqvczRr5B4u
/SD+OXLhdrzNaIpYc7DNhbT41DdG0F+ZiNQao0rQJrvw7pcR6D+CzqMmLk34Q4VU
h0e2nXSqNS4S/0coKUomL1eMSHpMqJVGTQhlWHDU7xMyOZ2t5TleHBI+OhfXAMyV
iEcaZUeengV73RoX+ycAYb5tjZOwk0GlolQxYl4rnjro2c2i5ezK8F+xdfkbCL/D
NfUGg01JBfCc1Yb3DtOpTaQcnnwFyJjQX8aPMTtDbT/4JZsHujZdRKU9yQIDAQAB
o0IwQDAOBgNVHQ8BAf8EBAMCAQYwDwYDVR0TAQH/BAUwAwEB/zAdBgNVHQ4EFgQU
+8T3UU5pFUA8jFwy4pLioRxg1IgwDQYJKoZIhvcNAQELBQADggEBAELa5wEx7CAX
y98v2iDAQ4uXNIrVZFp3zAgL1Rvtivf85Vz6+aQMSflJG8Ftk205PbUPhcvMrdFi
NdC9mGZ1K+QoyAIP+cU6zDVhzH73Vjlg/JyMa8kRFaqJMAnExGKNuzu4Ox742XKH
ej+WbykRbnB3t/Fvw4WrA0ZhQip/314SOyF71xDHGfBQrJYItGEB7kIriTOUL0Or
Eh1pkuxLBmO/iz4iAMaaG5JuVlPtDEYLX1kBx/aPh9sjgw28AWvlA1L/HawmXLsR
Yg+zBuGRGSu1IfIIwjshOGmsz+jaTM0SEZ5AtbmOl1gvGSgj8Ylod+Qb7gXBxBO8
yUsW6+ksN+o=
-----END CERTIFICATE-----`)
			keyPEM = []byte(`-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEAsiOi5NGvtPtJLD4FfMUne1KcgtKso91WdriOJWF6mfWiB2fn
bMS8EaKaU4AMXyrQpn6neZTDXeH5DOXhiQqvczRr5B4u/SD+OXLhdrzNaIpYc7DN
hbT41DdG0F+ZiNQao0rQJrvw7pcR6D+CzqMmLk34Q4VUh0e2nXSqNS4S/0coKUom
L1eMSHpMqJVGTQhlWHDU7xMyOZ2t5TleHBI+OhfXAMyViEcaZUeengV73RoX+ycA
Yb5tjZOwk0GlolQxYl4rnjro2c2i5ezK8F+xdfkbCL/DNfUGg01JBfCc1Yb3DtOp
TaQcnnwFyJjQX8aPMTtDbT/4JZsHujZdRKU9yQIDAQABAoIBAQCacqVHyLmTq478
qeVuES2zEaQbFPeTt1LA6jBsHoECvWI3E5IlzsjUbWtqXAnd9SwkPomLszxTyJl6
4lDR1Y7azqeAh97rntBsFLuAjB93tQMNg0wd0hMvQ6HFBi4C4QsbasDf5HD3G8nt
2CrcZ72xxe4q9I2eIMIm8ECmjQTxiFiVf89TRz5Y+63IniId9Gh7WKmDR59sS31I
aEVRVRS9934tdkKx3TJd4Hmb1SNusvnx8wiTfi12nVjgVtYLLzPkd18I58wNvyj/
BE4iyiM4AqzQBqgEjc8Hw6YeR3Mwu6zyA0u7g3pXHhO4JL/eOpxWY6DAVlOt+WWC
ZkhGxs0lAoGBAN8GgPKVOX9x75CPv44ATfbZ5g7qmT5wrhHIlcF/1B1Q0xvZsrmn
2Hax96EINk93osWaiKAWoVIt0mHuoE2k5TK1cazI+DatyuXgU+3ngxoI7SPK95w2
EcXTKkFGgz5/WU2XWgYRdDy2gzb3XTlygPael+pjWYb5bRQjw6hALwQHAoGBAMx6
MWcX9FmeHvjBjXRyxz4xehqv8iXnMKIghAfCTD0zQ4OTGher5mVVCcWncKB8s/c7
5mIaKfTaoGgfVeGlrBeGLSeDWoHQMdWP1ZBMchNKpzZ9OXU2QmYrkUzFPJGTUSJe
sKLLYD2R+vwWGra508rJBKQKMnmIf7MLacB6lVuvAoGAJ/HoQoqLo9HqUIAOlQZk
8GOSmvVVwSM5aiH9AI0+lomVZhWVtz7ivE+fxI3N/Gm3E6Fb+yBSgH+IgNXWjFGO
Y4iv9XyBSHnUL1wAbEnc51rV7mU5+BaPFFl/5fUVKKpyej0zeIbDxOQDmGKxpcpm
YsWA/BATRuOBr+u/7XChex0CgYBdGG0RsPhRLQqQ2x6aG//WsxQSvnSTCTU9O2yh
U7b+Ti644uqISH13OUZftSI0D1Koh58Wny7nCfrqLQoe2B0IANDiIo28eJuXzgq/
ze5KFj0XM+BLG08T0VYwC8TNyrKv4UiudcX1glcxGqdC9kwVEXyJaxMb/ieVzuZw
+d6yhQKBgQCyR66MFetyEffnbxHng3WIG4MzJj7Bn5IewQiKe6yWgrS4xMxoUywx
xdiQxdLMPqh48D9u+bwt+roq66lt1kcF0mvIUgEYXhaPj/9moG8cfgmbmF9tsm08
bW4nbZLxXHQ4e+OOPeBUXUP9V0QcE4XixdvQuslfVxjn0Ja82gdzeA==
-----END RSA PRIVATE KEY-----`)
		)

		BeforeEach(func() {
			config = &rest.Config{}
		})

		It("should return an error because cert cannot be parsed", func() {
			cert, err := ClientCertificateFromRESTConfig(config)
			Expect(err).To(MatchError(ContainSubstring("failed to find any PEM data in certificate input")))
			Expect(cert).To(BeNil())
		})

		It("should return an error because key cannot be parsed", func() {
			config.CertData = certPEM

			cert, err := ClientCertificateFromRESTConfig(config)
			Expect(err).To(MatchError(ContainSubstring("failed to find any PEM data in key input")))
			Expect(cert).To(BeNil())
		})

		It("should return the parsed certificate", func() {
			config.KeyData = keyPEM
			config.CertData = certPEM

			cert, err := ClientCertificateFromRESTConfig(config)
			Expect(err).NotTo(HaveOccurred())
			Expect(cert.Leaf.NotAfter).To(Equal(time.Date(2027, 4, 11, 7, 7, 0, 0, time.UTC)))
		})
	})

	Describe("#TolerationForTaint", func() {
		It("should return a toleration for taint with 'Equal' operator", func() {
			taint := corev1.Taint{
				Key:       "someKey",
				Value:     "someValue",
				Effect:    corev1.TaintEffectNoSchedule,
				TimeAdded: &metav1.Time{},
			}

			Expect(TolerationForTaint(taint)).To(Equal(corev1.Toleration{
				Key:      taint.Key,
				Operator: corev1.TolerationOpEqual,
				Value:    taint.Value,
				Effect:   taint.Effect,
			}))
		})

		It("should return a toleration for taint with 'Exists' operator", func() {
			taint := corev1.Taint{
				Key:       "someKey",
				Effect:    corev1.TaintEffectNoSchedule,
				TimeAdded: &metav1.Time{},
			}

			Expect(TolerationForTaint(taint)).To(Equal(corev1.Toleration{
				Key:      taint.Key,
				Operator: corev1.TolerationOpExists,
				Effect:   taint.Effect,
			}))
		})
	})

	Describe("#ComparableTolerations", func() {
		var comparabelTolerations *ComparableTolerations

		BeforeEach(func() {
			comparabelTolerations = &ComparableTolerations{}
		})

		Describe("#Transform", func() {
			It("should be equal if toleration seconds are not set", func() {
				toleration := corev1.Toleration{
					Key:      "someKey",
					Operator: corev1.TolerationOpEqual,
					Value:    "someValue",
					Effect:   corev1.TaintEffectNoExecute,
				}

				Expect(comparabelTolerations.Transform(toleration)).To(Equal(corev1.Toleration{
					Key:      "someKey",
					Operator: corev1.TolerationOpEqual,
					Value:    "someValue",
					Effect:   corev1.TaintEffectNoExecute,
				}))
			})

			It("should reuse pointer for same value", func() {
				toleration1 := corev1.Toleration{
					Key:               "someKey",
					Operator:          corev1.TolerationOpEqual,
					Value:             "someValue",
					Effect:            corev1.TaintEffectNoExecute,
					TolerationSeconds: ptr.To[int64](300),
				}

				toleration2 := corev1.Toleration{
					Key:               "someKey",
					Operator:          corev1.TolerationOpEqual,
					Value:             "someValue",
					Effect:            corev1.TaintEffectNoExecute,
					TolerationSeconds: ptr.To[int64](300),
				}

				Expect(toleration1).ToNot(BeIdenticalTo(toleration2))

				toleration1 = comparabelTolerations.Transform(toleration1)
				toleration2 = comparabelTolerations.Transform(toleration2)

				Expect(toleration1).To(BeIdenticalTo(toleration2))
			})

			It("should not be identical if different toleration seconds are used", func() {
				toleration1 := corev1.Toleration{
					Key:               "someKey",
					Operator:          corev1.TolerationOpEqual,
					Value:             "someValue",
					Effect:            corev1.TaintEffectNoExecute,
					TolerationSeconds: ptr.To[int64](299),
				}

				toleration2 := corev1.Toleration{
					Key:               "someKey",
					Operator:          corev1.TolerationOpEqual,
					Value:             "someValue",
					Effect:            corev1.TaintEffectNoExecute,
					TolerationSeconds: ptr.To[int64](300),
				}

				toleration1 = comparabelTolerations.Transform(toleration1)
				toleration2 = comparabelTolerations.Transform(toleration2)

				Expect(toleration1).ToNot(BeIdenticalTo(toleration2))
			})
		})
	})
})
