// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package clientmap_test

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	componentbaseconfig "k8s.io/component-base/config"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	. "github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	kubernetesfake "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	. "github.com/gardener/gardener/pkg/client/kubernetes/test"
	operatorclient "github.com/gardener/gardener/pkg/operator/client"
	mockclient "github.com/gardener/gardener/third_party/mock/controller-runtime/client"
)

var _ = Describe("GardenClientMap", func() {
	var (
		ctx               context.Context
		log               logr.Logger
		ctrl              *gomock.Controller
		mockRuntimeClient *mockclient.MockClient

		cm                     ClientMap
		key                    ClientSetKey
		factory                *GardenClientSetFactory
		clientConnectionConfig componentbaseconfig.ClientConnectionConfiguration
		clientOptions          client.Options

		garden *operatorv1alpha1.Garden
	)

	BeforeEach(func() {
		ctx = context.Background()
		log = logr.Discard()
		ctrl = gomock.NewController(GinkgoT())
		mockRuntimeClient = mockclient.NewMockClient(ctrl)

		garden = &operatorv1alpha1.Garden{
			ObjectMeta: metav1.ObjectMeta{
				Name: "garden-eden",
			},
		}

		LookupHost = func(host string) ([]string, error) {
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
		factory = &GardenClientSetFactory{
			RuntimeClient:          mockRuntimeClient,
			ClientConnectionConfig: clientConnectionConfig,
		}
		cm = NewGardenClientMap(log, factory)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#GetClient", func() {
		It("should fail if ClientSetKey type is unsupported", func() {
			key = fakeKey{}
			cs, err := cm.GetClient(ctx, key)
			Expect(cs).To(BeNil())
			Expect(err).To(MatchError(ContainSubstring("unsupported ClientSetKey")))
		})

		It("should use external kubeconfig if LookupHost fails (out-of-cluster), failing because of unpopulated token", func() {
			fakeErr := errors.New("fake")
			mockRuntimeClient.EXPECT().Get(ctx, client.ObjectKey{Namespace: "garden", Name: "gardener"}, gomock.AssignableToTypeOf(&corev1.Secret{})).
				DoAndReturn(func(_ context.Context, _ client.ObjectKey, _ client.Object, _ ...client.GetOption) error {
					return nil
				})
			LookupHost = func(_ string) ([]string, error) {
				return nil, fakeErr
			}

			NewClientFromSecretObject = func(secret *corev1.Secret, _ ...kubernetes.ConfigFunc) (kubernetes.Interface, error) {
				Expect(secret.Namespace).To(Equal("garden"))
				Expect(secret.Name).To(Equal("gardener"))
				return nil, fakeErr
			}

			cs, err := cm.GetClient(ctx, key)
			Expect(cs).To(BeNil())
			Expect(err).To(MatchError(ContainSubstring("token for virtual garden kubeconfig was not populated yet")))
		})

		It("should use external kubeconfig if LookupHost fails (out-of-cluster), failing because NewClientFromSecretObject fails", func() {
			fakeErr := errors.New("fake")
			mockRuntimeClient.EXPECT().Get(ctx, client.ObjectKey{Namespace: "garden", Name: "gardener"}, gomock.AssignableToTypeOf(&corev1.Secret{})).
				DoAndReturn(func(_ context.Context, key client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
					(&corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name:      key.Name,
							Namespace: key.Namespace,
						},
						Data: dataWithPopulatedToken(),
					}).DeepCopyInto(obj.(*corev1.Secret))
					return nil
				})
			LookupHost = func(_ string) ([]string, error) {
				return nil, fakeErr
			}

			NewClientFromSecretObject = func(secret *corev1.Secret, _ ...kubernetes.ConfigFunc) (kubernetes.Interface, error) {
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
				DoAndReturn(func(_ context.Context, _ client.ObjectKey, _ client.Object, _ ...client.GetOption) error {
					return nil
				})

			NewClientFromSecretObject = func(secret *corev1.Secret, fns ...kubernetes.ConfigFunc) (kubernetes.Interface, error) {
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
					DoAndReturn(func(_ context.Context, key client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
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
					DoAndReturn(func(_ context.Context, key client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
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

			NewClientFromSecretObject = func(secret *corev1.Secret, fns ...kubernetes.ConfigFunc) (kubernetes.Interface, error) {
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

	Describe("#GardenerSecretName", func() {
		It("should return the internal secret name if lookup is successful", func() {
			LookupHost = func(_ string) ([]string, error) {
				return []string{"10.0.1.1"}, nil
			}

			Expect(GardenerSecretName(log, "garden")).To(Equal("gardener-internal"))
		})

		It("should return the external secret name if no address is returned", func() {
			LookupHost = func(_ string) ([]string, error) {
				return []string{}, nil
			}

			Expect(GardenerSecretName(log, "garden")).To(Equal("gardener"))
		})

		It("should return the external if lookup failed", func() {
			LookupHost = func(_ string) ([]string, error) {
				return []string{"10.0.1.1"}, errors.New("fake")
			}

			Expect(GardenerSecretName(log, "garden")).To(Equal("gardener"))
		})
	})

	Describe("#CalculateClientSetHash", func() {
		It("should fail if ClientSetKey type is unsupported", func() {
			key = fakeKey{}
			hash, err := factory.CalculateClientSetHash(ctx, key)
			Expect(hash).To(BeEmpty())
			Expect(err).To(MatchError(ContainSubstring("unsupported ClientSetKey")))
		})

		It("should fail if Get gardener-internal Secret fails", func() {
			fakeErr := errors.New("fake")
			mockRuntimeClient.EXPECT().Get(ctx, client.ObjectKey{Namespace: "garden", Name: "gardener-internal"}, gomock.AssignableToTypeOf(&corev1.Secret{})).
				DoAndReturn(func(_ context.Context, _ client.ObjectKey, _ client.Object, _ ...client.GetOption) error {
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
						DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
							(&corev1.Secret{}).DeepCopyInto(obj.(*corev1.Secret))
							return nil
						}),
					mockRuntimeClient.EXPECT().Get(ctx, client.ObjectKey{Namespace: "garden", Name: secretName}, gomock.AssignableToTypeOf(&corev1.Secret{})).
						DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
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
				LookupHost = func(_ string) ([]string, error) {
					return nil, nil
				}
				test("gardener")
			})
		})
	})
})
