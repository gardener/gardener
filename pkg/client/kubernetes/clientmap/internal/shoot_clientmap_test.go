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

package internal_test

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientcmdlatest "k8s.io/client-go/tools/clientcmd/api/latest"
	clientcmdv1 "k8s.io/client-go/tools/clientcmd/api/v1"
	componentbaseconfig "k8s.io/component-base/config"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/internal"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	kubernetesfake "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	. "github.com/gardener/gardener/pkg/client/kubernetes/test"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

var _ = Describe("ShootClientMap", func() {
	var (
		ctx              context.Context
		ctrl             *gomock.Controller
		mockGardenClient *mockclient.MockClient
		mockSeedClient   *mockclient.MockClient

		cm                     clientmap.ClientMap
		key                    clientmap.ClientSetKey
		factory                *internal.ShootClientSetFactory
		clientConnectionConfig componentbaseconfig.ClientConnectionConfiguration
		clientOptions          client.Options

		shoot *gardencorev1beta1.Shoot
	)

	BeforeEach(func() {
		ctx = context.TODO()
		ctrl = gomock.NewController(GinkgoT())
		mockGardenClient = mockclient.NewMockClient(ctrl)
		mockSeedClient = mockclient.NewMockClient(ctrl)

		shoot = &gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "garden-eden",
				Name:      "forbidden-fruit",
			},
			Spec: gardencorev1beta1.ShootSpec{
				SeedName: pointer.String("apple-seed"),
			},
			Status: gardencorev1beta1.ShootStatus{
				TechnicalID: "shoot--eden--forbidden-fruit",
			},
		}

		internal.ProjectForNamespaceFromReader = func(ctx context.Context, c client.Reader, namespaceName string) (*gardencorev1beta1.Project, error) {
			return &gardencorev1beta1.Project{
				ObjectMeta: metav1.ObjectMeta{
					Name: "eden",
				},
				Spec: gardencorev1beta1.ProjectSpec{
					Namespace: pointer.String("garden-eden"),
				}}, nil
		}
		internal.LookupHost = func(host string) ([]string, error) {
			Expect(host).To(Equal("kube-apiserver." + shoot.Status.TechnicalID + ".svc"))
			return []string{"10.0.1.1"}, nil
		}

		key = keys.ForShoot(shoot)

		clientConnectionConfig = componentbaseconfig.ClientConnectionConfiguration{
			Kubeconfig:         "/var/run/secrets/kubeconfig",
			AcceptContentTypes: "application/vnd.kubernetes.protobuf;application/json",
			ContentType:        "application/vnd.kubernetes.protobuf",
			QPS:                42,
			Burst:              43,
		}
		clientOptions = client.Options{Scheme: kubernetes.ShootScheme}
		factory = &internal.ShootClientSetFactory{
			GardenClient:           mockGardenClient,
			SeedClient:             mockSeedClient,
			ClientConnectionConfig: clientConnectionConfig,
		}
		cm = internal.NewShootClientMap(logr.Discard(), factory)
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

		It("should fail if it cannot get Shoot object", func() {
			mockGardenClient.EXPECT().Get(ctx, client.ObjectKey{Namespace: shoot.Namespace, Name: shoot.Name}, gomock.AssignableToTypeOf(&gardencorev1beta1.Shoot{})).
				Return(apierrors.NewNotFound(gardencorev1beta1.Resource("shoot"), shoot.Name))

			cs, err := cm.GetClient(ctx, key)
			Expect(cs).To(BeNil())
			Expect(err).To(MatchError(ContainSubstring("failed to get Shoot object")))
		})

		It("should fail if Shoot is not scheduled yet", func() {
			shoot.Spec.SeedName = nil
			mockGardenClient.EXPECT().Get(ctx, client.ObjectKey{Namespace: shoot.Namespace, Name: shoot.Name}, gomock.AssignableToTypeOf(&gardencorev1beta1.Shoot{})).
				DoAndReturn(func(ctx context.Context, key client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
					shoot.DeepCopyInto(obj.(*gardencorev1beta1.Shoot))
					return nil
				})

			cs, err := cm.GetClient(ctx, key)
			Expect(cs).To(BeNil())
			Expect(err).To(MatchError(ContainSubstring(fmt.Sprintf("shoot %q is not scheduled yet", key.Key()))))
		})

		It("should fail if ProjectForNamespaceFromReader fails", func() {
			shoot.Status.TechnicalID = "" // trigger retrieval of project instead of relying on shoot status

			fakeErr := fmt.Errorf("fake")
			mockGardenClient.EXPECT().Get(ctx, client.ObjectKey{Namespace: shoot.Namespace, Name: shoot.Name}, gomock.AssignableToTypeOf(&gardencorev1beta1.Shoot{})).
				DoAndReturn(func(ctx context.Context, key client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
					shoot.DeepCopyInto(obj.(*gardencorev1beta1.Shoot))
					return nil
				})
			internal.ProjectForNamespaceFromReader = func(ctx context.Context, c client.Reader, namespaceName string) (*gardencorev1beta1.Project, error) {
				return nil, fakeErr
			}

			cs, err := cm.GetClient(ctx, key)
			Expect(cs).To(BeNil())
			Expect(err).To(MatchError(ContainSubstring("failed to get Project for Shoot")))
		})

		It("should use external kubeconfig if LookupHost fails (out-of-cluster), failing because of unpopulated token", func() {
			technicalID := shoot.Status.TechnicalID
			shoot.Status.TechnicalID = "" // trigger retrieval of project instead of relying on shoot status

			fakeErr := fmt.Errorf("fake")
			mockGardenClient.EXPECT().Get(ctx, client.ObjectKey{Namespace: shoot.Namespace, Name: shoot.Name}, gomock.AssignableToTypeOf(&gardencorev1beta1.Shoot{})).
				DoAndReturn(func(ctx context.Context, key client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
					shoot.DeepCopyInto(obj.(*gardencorev1beta1.Shoot))
					return nil
				})
			mockSeedClient.EXPECT().Get(ctx, client.ObjectKey{Namespace: technicalID, Name: "gardener"}, gomock.AssignableToTypeOf(&corev1.Secret{})).
				DoAndReturn(func(ctx context.Context, key client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
					return nil
				})
			internal.LookupHost = func(host string) ([]string, error) {
				return nil, fakeErr
			}

			internal.NewClientFromSecretObject = func(secret *corev1.Secret, fns ...kubernetes.ConfigFunc) (kubernetes.Interface, error) {
				Expect(secret.Namespace).To(Equal(technicalID))
				Expect(secret.Name).To(Equal("gardener"))
				return nil, fakeErr
			}

			cs, err := cm.GetClient(ctx, key)
			Expect(cs).To(BeNil())
			Expect(err).To(MatchError(ContainSubstring("token for shoot kubeconfig was not populated yet")))
		})

		It("should use external kubeconfig if LookupHost fails (out-of-cluster), failing because NewClientFromSecretObject fails", func() {
			technicalID := shoot.Status.TechnicalID
			shoot.Status.TechnicalID = "" // trigger retrieval of project instead of relying on shoot status

			fakeErr := fmt.Errorf("fake")
			mockGardenClient.EXPECT().Get(ctx, client.ObjectKey{Namespace: shoot.Namespace, Name: shoot.Name}, gomock.AssignableToTypeOf(&gardencorev1beta1.Shoot{})).
				DoAndReturn(func(ctx context.Context, key client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
					shoot.DeepCopyInto(obj.(*gardencorev1beta1.Shoot))
					return nil
				})
			mockSeedClient.EXPECT().Get(ctx, client.ObjectKey{Namespace: technicalID, Name: "gardener"}, gomock.AssignableToTypeOf(&corev1.Secret{})).
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
				Expect(secret.Namespace).To(Equal(technicalID))
				Expect(secret.Name).To(Equal("gardener"))
				return nil, fakeErr
			}

			cs, err := cm.GetClient(ctx, key)
			Expect(cs).To(BeNil())
			Expect(err).To(MatchError(fmt.Sprintf("error creating new ClientSet for key %q: fake", key.Key())))
		})

		It("should fail constructing a new ClientSet (in-cluster) because token is not populated", func() {
			fakeCS := kubernetesfake.NewClientSet()

			gomock.InOrder(
				mockGardenClient.EXPECT().Get(ctx, client.ObjectKey{Namespace: shoot.Namespace, Name: shoot.Name}, gomock.AssignableToTypeOf(&gardencorev1beta1.Shoot{})).
					DoAndReturn(func(ctx context.Context, key client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
						shoot.DeepCopyInto(obj.(*gardencorev1beta1.Shoot))
						return nil
					}),
				mockSeedClient.EXPECT().Get(ctx, client.ObjectKey{Namespace: shoot.Status.TechnicalID, Name: "gardener-internal"}, gomock.AssignableToTypeOf(&corev1.Secret{})).
					DoAndReturn(func(ctx context.Context, key client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
						return nil
					}),
			)

			internal.NewClientFromSecretObject = func(secret *corev1.Secret, fns ...kubernetes.ConfigFunc) (kubernetes.Interface, error) {
				Expect(secret.Namespace).To(Equal(shoot.Status.TechnicalID))
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
			Expect(err).To(MatchError(ContainSubstring("token for shoot kubeconfig was not populated yet")))
		})

		It("should correctly construct a new ClientSet (in-cluster)", func() {
			fakeCS := kubernetesfake.NewClientSet()
			changedTechnicalID := "foo"
			gomock.InOrder(
				mockGardenClient.EXPECT().Get(ctx, client.ObjectKey{Namespace: shoot.Namespace, Name: shoot.Name}, gomock.AssignableToTypeOf(&gardencorev1beta1.Shoot{})).
					DoAndReturn(func(ctx context.Context, key client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
						shoot.DeepCopyInto(obj.(*gardencorev1beta1.Shoot))
						return nil
					}),
				mockSeedClient.EXPECT().Get(ctx, client.ObjectKey{Namespace: shoot.Status.TechnicalID, Name: "gardener-internal"}, gomock.AssignableToTypeOf(&corev1.Secret{})).
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
				mockGardenClient.EXPECT().Get(ctx, client.ObjectKey{Namespace: shoot.Namespace, Name: shoot.Name}, gomock.AssignableToTypeOf(&gardencorev1beta1.Shoot{})).
					DoAndReturn(func(ctx context.Context, key client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
						shoot.Status.TechnicalID = changedTechnicalID
						shoot.DeepCopyInto(obj.(*gardencorev1beta1.Shoot))
						return nil
					}),
				mockSeedClient.EXPECT().Get(ctx, client.ObjectKey{Namespace: changedTechnicalID, Name: "gardener-internal"}, gomock.AssignableToTypeOf(&corev1.Secret{})).
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
				Expect(secret.Namespace).To(Equal(shoot.Status.TechnicalID))
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
			mockGardenClient.EXPECT().Get(ctx, client.ObjectKey{Namespace: shoot.Namespace, Name: shoot.Name}, gomock.AssignableToTypeOf(&gardencorev1beta1.Shoot{})).
				DoAndReturn(func(ctx context.Context, key client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
					shoot.DeepCopyInto(obj.(*gardencorev1beta1.Shoot))
					return nil
				})
			mockSeedClient.EXPECT().Get(ctx, client.ObjectKey{Namespace: shoot.Status.TechnicalID, Name: "gardener-internal"}, gomock.AssignableToTypeOf(&corev1.Secret{})).
				DoAndReturn(func(ctx context.Context, key client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
					return fakeErr
				})

			hash, err := factory.CalculateClientSetHash(ctx, key)
			Expect(hash).To(BeEmpty())
			Expect(err).To(MatchError("fake"))
		})

		Context("correctly calculate hash", func() {
			test := func(secretName string) {
				changedTechnicalID := "foo"
				gomock.InOrder(
					mockGardenClient.EXPECT().Get(ctx, client.ObjectKey{Namespace: shoot.Namespace, Name: shoot.Name}, gomock.AssignableToTypeOf(&gardencorev1beta1.Shoot{})).
						DoAndReturn(func(ctx context.Context, key client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
							shoot.DeepCopyInto(obj.(*gardencorev1beta1.Shoot))
							return nil
						}),
					mockSeedClient.EXPECT().Get(ctx, client.ObjectKey{Namespace: shoot.Status.TechnicalID, Name: secretName}, gomock.AssignableToTypeOf(&corev1.Secret{})).
						DoAndReturn(func(ctx context.Context, key client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
							(&corev1.Secret{}).DeepCopyInto(obj.(*corev1.Secret))
							return nil
						}),
					mockGardenClient.EXPECT().Get(ctx, client.ObjectKey{Namespace: shoot.Namespace, Name: shoot.Name}, gomock.AssignableToTypeOf(&gardencorev1beta1.Shoot{})).
						DoAndReturn(func(ctx context.Context, key client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
							shoot.Status.TechnicalID = changedTechnicalID
							shoot.DeepCopyInto(obj.(*gardencorev1beta1.Shoot))
							return nil
						}),
					mockSeedClient.EXPECT().Get(ctx, client.ObjectKey{Namespace: changedTechnicalID, Name: secretName}, gomock.AssignableToTypeOf(&corev1.Secret{})).
						DoAndReturn(func(ctx context.Context, key client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
							(&corev1.Secret{}).DeepCopyInto(obj.(*corev1.Secret))
							return nil
						}),
				)

				hash, err := factory.CalculateClientSetHash(ctx, key)
				Expect(hash).To(Equal("e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"))
				Expect(err).NotTo(HaveOccurred())

				Expect(factory.InvalidateClient(key)).To(Succeed())

				hash, err = factory.CalculateClientSetHash(ctx, keys.ForShoot(shoot))
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

func dataWithPopulatedToken() map[string][]byte {
	kubeconfigRaw, err := runtime.Encode(clientcmdlatest.Codec, kubernetesutils.NewKubeconfig(
		"context",
		clientcmdv1.Cluster{Server: "server", CertificateAuthorityData: []byte("cacert")},
		clientcmdv1.AuthInfo{Token: "some-token"},
	))
	Expect(err).NotTo(HaveOccurred())

	return map[string][]byte{"kubeconfig": kubeconfigRaw}
}

type fakeKey struct{}

func (f fakeKey) Key() string {
	return "fake"
}
