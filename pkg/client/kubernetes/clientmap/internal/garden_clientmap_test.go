// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package internal_test

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	componentbaseconfig "k8s.io/component-base/config"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/internal"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	kubernetesfake "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	. "github.com/gardener/gardener/pkg/client/kubernetes/test"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	operatorclient "github.com/gardener/gardener/pkg/operator/client"
)

var _ = Describe("GardenClientMap", func() {
	var (
		ctx               context.Context
		ctrl              *gomock.Controller
		mockRuntimeClient *mockclient.MockClient

		cm                     clientmap.ClientMap
		key                    clientmap.ClientSetKey
		factory                *internal.GardenClientSetFactory
		clientConnectionConfig componentbaseconfig.ClientConnectionConfiguration
		clientOptions          client.Options

		garden *operatorv1alpha1.Garden
	)

	BeforeEach(func() {
		ctx = context.TODO()
		ctrl = gomock.NewController(GinkgoT())
		mockRuntimeClient = mockclient.NewMockClient(ctrl)

		garden = &operatorv1alpha1.Garden{
			ObjectMeta: metav1.ObjectMeta{
				Name: "garden-eden",
			},
		}

		internal.LookupHost = func(host string) ([]string, error) {
			Expect(host).To(Equal("virtual-garden-kube-apiserver.garden.svc.cluster.local"))
			return []string{"10.0.1.1"}, nil
		}

		key = keys.ForGarden(garden)

		clientConnectionConfig = componentbaseconfig.ClientConnectionConfiguration{
			Kubeconfig:         "/var/run/secrets/kubeconfig",
			AcceptContentTypes: "application/vnd.kubernetes.protobuf;application/json",
			ContentType:        "application/vnd.kubernetes.protobuf",
			QPS:                42,
			Burst:              43,
		}
		clientOptions = client.Options{Scheme: operatorclient.VirtualScheme}
		factory = &internal.GardenClientSetFactory{
			RuntimeClient:          mockRuntimeClient,
			ClientConnectionConfig: clientConnectionConfig,
		}
		cm = internal.NewGardenClientMap(logr.Discard(), factory)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Context("#GetClient", func() {
		It("should fail if ClientSetKey type is unsupported", func() {
			key = fakeKey{}
			cs, err := cm.GetClient(ctx, key)
			Expect(cs).To(BeNil())
			Expect(err).To(MatchError(ContainSubstring("unsupported ClientSetKey")))
		})

		It("should use external kubeconfig if LookupHost fails (out-of-cluster), failing because of unpopulated token", func() {
			fakeErr := fmt.Errorf("fake")
			mockRuntimeClient.EXPECT().Get(ctx, client.ObjectKey{Namespace: "garden", Name: "gardener"}, gomock.AssignableToTypeOf(&corev1.Secret{})).
				DoAndReturn(func(ctx context.Context, key client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
					return nil
				})
			internal.LookupHost = func(host string) ([]string, error) {
				return nil, fakeErr
			}

			internal.NewClientFromSecretObject = func(secret *corev1.Secret, fns ...kubernetes.ConfigFunc) (kubernetes.Interface, error) {
				Expect(secret.Namespace).To(Equal("garden"))
				Expect(secret.Name).To(Equal("gardener"))
				return nil, fakeErr
			}

			cs, err := cm.GetClient(ctx, key)
			Expect(cs).To(BeNil())
			Expect(err).To(MatchError(ContainSubstring("token for virtual garden kubeconfig was not populated yet")))
		})

		It("should use external kubeconfig if LookupHost fails (out-of-cluster), failing because NewClientFromSecretObject fails", func() {
			fakeErr := fmt.Errorf("fake")
			mockRuntimeClient.EXPECT().Get(ctx, client.ObjectKey{Namespace: "garden", Name: "gardener"}, gomock.AssignableToTypeOf(&corev1.Secret{})).
				DoAndReturn(func(ctx context.Context, key client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
					(&corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name:      key.Name,
							Namespace: key.Namespace,
						},
						Data: dataWithPopulatedToken(),
					}).DeepCopyInto(obj.(*corev1.Secret))
					return nil
				})
			internal.LookupHost = func(host string) ([]string, error) {
				return nil, fakeErr
			}

			internal.NewClientFromSecretObject = func(secret *corev1.Secret, fns ...kubernetes.ConfigFunc) (kubernetes.Interface, error) {
				Expect(secret.Namespace).To(Equal("garden"))
				Expect(secret.Name).To(Equal("gardener"))
				return nil, fakeErr
			}

			cs, err := cm.GetClient(ctx, key)
			Expect(cs).To(BeNil())
			Expect(err).To(MatchError(fmt.Sprintf("error creating new ClientSet for key %q: fake", key.Key())))
		})

		It("should fail constructing a new ClientSet (in-cluster) because token is not populated", func() {
			fakeCS := kubernetesfake.NewClientSet()

			mockRuntimeClient.EXPECT().Get(ctx, client.ObjectKey{Namespace: "garden", Name: "gardener-internal"}, gomock.AssignableToTypeOf(&corev1.Secret{})).
				DoAndReturn(func(ctx context.Context, key client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
					return nil
				})

			internal.NewClientFromSecretObject = func(secret *corev1.Secret, fns ...kubernetes.ConfigFunc) (kubernetes.Interface, error) {
				Expect(secret.Namespace).To(Equal("garden"))
				Expect(secret.Name).To(Equal("gardener-internal"))
				Expect(fns).To(ConsistOfConfigFuncs(
					kubernetes.WithClientConnectionOptions(clientConnectionConfig),
					kubernetes.WithClientOptions(clientOptions),
					kubernetes.WithDisabledCachedClient(),
				))
				return fakeCS, nil
			}

			cs, err := cm.GetClient(ctx, key)
			Expect(cs).To(BeNil())
			Expect(err).To(MatchError(ContainSubstring("token for virtual garden kubeconfig was not populated yet")))
		})

		It("should correctly construct a new ClientSet (in-cluster)", func() {
			fakeCS := kubernetesfake.NewClientSet()
			gomock.InOrder(
				mockRuntimeClient.EXPECT().Get(ctx, client.ObjectKey{Namespace: "garden", Name: "gardener-internal"}, gomock.AssignableToTypeOf(&corev1.Secret{})).
					DoAndReturn(func(ctx context.Context, key client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
						(&corev1.Secret{
							ObjectMeta: metav1.ObjectMeta{
								Name:      key.Name,
								Namespace: key.Namespace,
							},
							Data: dataWithPopulatedToken(),
						}).DeepCopyInto(obj.(*corev1.Secret))
						return nil
					}),
				mockRuntimeClient.EXPECT().Get(ctx, client.ObjectKey{Namespace: "garden", Name: "gardener-internal"}, gomock.AssignableToTypeOf(&corev1.Secret{})).
					DoAndReturn(func(ctx context.Context, key client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
						(&corev1.Secret{
							ObjectMeta: metav1.ObjectMeta{
								Name:      key.Name,
								Namespace: key.Namespace,
							},
							Data: dataWithPopulatedToken(),
						}).DeepCopyInto(obj.(*corev1.Secret))
						return nil
					}),
			)

			internal.NewClientFromSecretObject = func(secret *corev1.Secret, fns ...kubernetes.ConfigFunc) (kubernetes.Interface, error) {
				Expect(secret.Namespace).To(Equal("garden"))
				Expect(secret.Name).To(Equal("gardener-internal"))
				Expect(fns).To(ConsistOfConfigFuncs(
					kubernetes.WithClientConnectionOptions(clientConnectionConfig),
					kubernetes.WithClientOptions(clientOptions),
					kubernetes.WithDisabledCachedClient(),
				))
				return fakeCS, nil
			}

			cs, err := cm.GetClient(ctx, key)
			Expect(err).NotTo(HaveOccurred())
			Expect(cs).To(BeIdenticalTo(fakeCS))

			Expect(cm.InvalidateClient(key)).To(Succeed())

			cs, err = cm.GetClient(ctx, key)
			Expect(err).NotTo(HaveOccurred())
			Expect(cs).To(BeIdenticalTo(fakeCS))
		})
	})

	Context("#CalculateClientSetHash", func() {
		It("should fail if ClientSetKey type is unsupported", func() {
			key = fakeKey{}
			hash, err := factory.CalculateClientSetHash(ctx, key)
			Expect(hash).To(BeEmpty())
			Expect(err).To(MatchError(ContainSubstring("unsupported ClientSetKey")))
		})

		It("should fail if Get gardener-internal Secret fails", func() {
			fakeErr := fmt.Errorf("fake")
			mockRuntimeClient.EXPECT().Get(ctx, client.ObjectKey{Namespace: "garden", Name: "gardener-internal"}, gomock.AssignableToTypeOf(&corev1.Secret{})).
				DoAndReturn(func(ctx context.Context, key client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
					return fakeErr
				})

			hash, err := factory.CalculateClientSetHash(ctx, key)
			Expect(hash).To(BeEmpty())
			Expect(err).To(MatchError("fake"))
		})

		Context("correctly calculate hash", func() {
			test := func(secretName string) {
				gomock.InOrder(
					mockRuntimeClient.EXPECT().Get(ctx, client.ObjectKey{Namespace: "garden", Name: secretName}, gomock.AssignableToTypeOf(&corev1.Secret{})).
						DoAndReturn(func(ctx context.Context, key client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
							(&corev1.Secret{}).DeepCopyInto(obj.(*corev1.Secret))
							return nil
						}),
					mockRuntimeClient.EXPECT().Get(ctx, client.ObjectKey{Namespace: "garden", Name: secretName}, gomock.AssignableToTypeOf(&corev1.Secret{})).
						DoAndReturn(func(ctx context.Context, key client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
							(&corev1.Secret{}).DeepCopyInto(obj.(*corev1.Secret))
							return nil
						}),
				)

				hash, err := factory.CalculateClientSetHash(ctx, key)
				Expect(hash).To(Equal("e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"))
				Expect(err).NotTo(HaveOccurred())

				Expect(factory.InvalidateClient(key)).To(Succeed())

				hash, err = factory.CalculateClientSetHash(ctx, keys.ForGarden(garden))
				Expect(hash).To(Equal("e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"))
				Expect(err).NotTo(HaveOccurred())
			}

			It("when in-cluster", func() {
				test("gardener-internal")
			})

			It("when out-of-cluster", func() {
				internal.LookupHost = func(host string) ([]string, error) {
					return nil, nil
				}
				test("gardener")
			})
		})
	})
})
