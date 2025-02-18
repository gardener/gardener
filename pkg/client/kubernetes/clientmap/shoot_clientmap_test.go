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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientcmdlatest "k8s.io/client-go/tools/clientcmd/api/latest"
	clientcmdv1 "k8s.io/client-go/tools/clientcmd/api/v1"
	componentbaseconfigv1alpha1 "k8s.io/component-base/config/v1alpha1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	. "github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	kubernetesfake "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	. "github.com/gardener/gardener/pkg/client/kubernetes/test"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	mockclient "github.com/gardener/gardener/third_party/mock/controller-runtime/client"
)

var _ = Describe("ShootClientMap", func() {
	var (
		ctx              context.Context
		ctrl             *gomock.Controller
		mockGardenClient *mockclient.MockClient
		mockSeedClient   *mockclient.MockClient

		cm                     ClientMap
		key                    ClientSetKey
		factory                *ShootClientSetFactory
		clientConnectionConfig componentbaseconfigv1alpha1.ClientConnectionConfiguration
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
				SeedName: ptr.To("apple-seed"),
			},
			Status: gardencorev1beta1.ShootStatus{
				TechnicalID: "shoot--eden--forbidden-fruit",
			},
		}

		key = keys.ForShoot(shoot)

		clientConnectionConfig = componentbaseconfigv1alpha1.ClientConnectionConfiguration{
			Kubeconfig:         "/var/run/secrets/kubeconfig",
			AcceptContentTypes: "application/vnd.kubernetes.protobuf;application/json",
			ContentType:        "application/vnd.kubernetes.protobuf",
			QPS:                42,
			Burst:              43,
		}
		clientOptions = client.Options{Scheme: kubernetes.ShootScheme}
		factory = &ShootClientSetFactory{
			GardenClient:           mockGardenClient,
			SeedClient:             mockSeedClient,
			ClientConnectionConfig: clientConnectionConfig,
		}
		cm = NewShootClientMap(logr.Discard(), factory)
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
				DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
					shoot.DeepCopyInto(obj.(*gardencorev1beta1.Shoot))
					return nil
				})

			cs, err := cm.GetClient(ctx, key)
			Expect(cs).To(BeNil())
			Expect(err).To(MatchError(ContainSubstring(fmt.Sprintf("shoot %q is not scheduled yet", key.Key()))))
		})

		It("should fail constructing a new ClientSet (in-cluster) because token is not populated", func() {
			fakeCS := kubernetesfake.NewClientSet()

			gomock.InOrder(
				mockGardenClient.EXPECT().Get(ctx, client.ObjectKey{Namespace: shoot.Namespace, Name: shoot.Name}, gomock.AssignableToTypeOf(&gardencorev1beta1.Shoot{})).
					DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
						shoot.DeepCopyInto(obj.(*gardencorev1beta1.Shoot))
						return nil
					}),
				mockSeedClient.EXPECT().Get(ctx, client.ObjectKey{Namespace: shoot.Status.TechnicalID, Name: "gardener-internal"}, gomock.AssignableToTypeOf(&corev1.Secret{})).
					DoAndReturn(func(_ context.Context, _ client.ObjectKey, _ client.Object, _ ...client.GetOption) error {
						return nil
					}),
			)

			NewClientFromSecretObject = func(secret *corev1.Secret, fns ...kubernetes.ConfigFunc) (kubernetes.Interface, error) {
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
					DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
						shoot.DeepCopyInto(obj.(*gardencorev1beta1.Shoot))
						return nil
					}),
				mockSeedClient.EXPECT().Get(ctx, client.ObjectKey{Namespace: shoot.Status.TechnicalID, Name: "gardener-internal"}, gomock.AssignableToTypeOf(&corev1.Secret{})).
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
				mockGardenClient.EXPECT().Get(ctx, client.ObjectKey{Namespace: shoot.Namespace, Name: shoot.Name}, gomock.AssignableToTypeOf(&gardencorev1beta1.Shoot{})).
					DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
						shoot.Status.TechnicalID = changedTechnicalID
						shoot.DeepCopyInto(obj.(*gardencorev1beta1.Shoot))
						return nil
					}),
				mockSeedClient.EXPECT().Get(ctx, client.ObjectKey{Namespace: changedTechnicalID, Name: "gardener-internal"}, gomock.AssignableToTypeOf(&corev1.Secret{})).
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
			fakeErr := errors.New("fake")
			mockGardenClient.EXPECT().Get(ctx, client.ObjectKey{Namespace: shoot.Namespace, Name: shoot.Name}, gomock.AssignableToTypeOf(&gardencorev1beta1.Shoot{})).
				DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
					shoot.DeepCopyInto(obj.(*gardencorev1beta1.Shoot))
					return nil
				})
			mockSeedClient.EXPECT().Get(ctx, client.ObjectKey{Namespace: shoot.Status.TechnicalID, Name: "gardener-internal"}, gomock.AssignableToTypeOf(&corev1.Secret{})).
				DoAndReturn(func(_ context.Context, _ client.ObjectKey, _ client.Object, _ ...client.GetOption) error {
					return fakeErr
				})

			hash, err := factory.CalculateClientSetHash(ctx, key)
			Expect(hash).To(BeEmpty())
			Expect(err).To(MatchError("fake"))
		})

		It("should correctly calculate hash", func() {
			changedTechnicalID := "foo"
			gomock.InOrder(
				mockGardenClient.EXPECT().Get(ctx, client.ObjectKey{Namespace: shoot.Namespace, Name: shoot.Name}, gomock.AssignableToTypeOf(&gardencorev1beta1.Shoot{})).
					DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
						shoot.DeepCopyInto(obj.(*gardencorev1beta1.Shoot))
						return nil
					}),
				mockSeedClient.EXPECT().Get(ctx, client.ObjectKey{Namespace: shoot.Status.TechnicalID, Name: "gardener-internal"}, gomock.AssignableToTypeOf(&corev1.Secret{})).
					DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
						(&corev1.Secret{}).DeepCopyInto(obj.(*corev1.Secret))
						return nil
					}),
				mockGardenClient.EXPECT().Get(ctx, client.ObjectKey{Namespace: shoot.Namespace, Name: shoot.Name}, gomock.AssignableToTypeOf(&gardencorev1beta1.Shoot{})).
					DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
						shoot.Status.TechnicalID = changedTechnicalID
						shoot.DeepCopyInto(obj.(*gardencorev1beta1.Shoot))
						return nil
					}),
				mockSeedClient.EXPECT().Get(ctx, client.ObjectKey{Namespace: changedTechnicalID, Name: "gardener-internal"}, gomock.AssignableToTypeOf(&corev1.Secret{})).
					DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
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
