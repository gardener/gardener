// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package internal_test

import (
	"context"
	"fmt"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/internal"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	fakeclientset "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/logger"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	baseconfig "k8s.io/component-base/config"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("SeedClientMap", func() {
	var (
		ctx              context.Context
		ctrl             *gomock.Controller
		c                *mockclient.MockClient
		fakeGardenClient *fakeclientset.ClientSet

		cm                     clientmap.ClientMap
		key                    clientmap.ClientSetKey
		factory                *internal.SeedClientSetFactory
		clientConnectionConfig baseconfig.ClientConnectionConfiguration

		seed *gardencorev1beta1.Seed
	)

	BeforeEach(func() {
		ctx = context.TODO()
		ctrl = gomock.NewController(GinkgoT())
		c = mockclient.NewMockClient(ctrl)
		fakeGardenClient = fakeclientset.NewClientSetBuilder().WithClient(c).Build()

		seed = &gardencorev1beta1.Seed{
			ObjectMeta: metav1.ObjectMeta{
				Name: "potato-seed",
			},
			Spec: gardencorev1beta1.SeedSpec{
				SecretRef: &corev1.SecretReference{
					Namespace: "backyard",
					Name:      "potato-secret",
				},
			},
		}

		key = keys.ForSeed(seed)

		clientConnectionConfig = baseconfig.ClientConnectionConfiguration{
			Kubeconfig:         "/var/run/secrets/kubeconfig",
			AcceptContentTypes: "application/vnd.kubernetes.protobuf;application/json",
			ContentType:        "application/vnd.kubernetes.protobuf",
			QPS:                42,
			Burst:              43,
		}
		factory = &internal.SeedClientSetFactory{
			GetGardenClient: func(ctx context.Context) (kubernetes.Interface, error) {
				return fakeGardenClient, nil
			},
			ClientConnectionConfig: clientConnectionConfig,
		}
		cm = internal.NewSeedClientMap(factory, logger.NewNopLogger())
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

		It("should fail if NewClientFromFile fails (in-cluster)", func() {
			factory.InCluster = true
			fakeErr := fmt.Errorf("fake")
			internal.NewClientFromFile = func(masterURL, kubeconfigPath string, fns ...kubernetes.ConfigFunc) (kubernetes.Interface, error) {
				return nil, fakeErr
			}

			cs, err := cm.GetClient(ctx, key)
			Expect(cs).To(BeNil())
			Expect(err).To(MatchError(fmt.Sprintf("error creating new ClientSet for key %q: fake", key.Key())))
		})

		It("should correctly construct a new ClientSet (in-cluster)", func() {
			factory.InCluster = true
			fakeCS := fakeclientset.NewClientSet()
			internal.NewClientFromFile = func(masterURL, kubeconfigPath string, fns ...kubernetes.ConfigFunc) (kubernetes.Interface, error) {
				Expect(masterURL).To(BeEmpty())
				Expect(kubeconfigPath).To(Equal(clientConnectionConfig.Kubeconfig))
				Expect(fns).To(kubernetes.ConsistOfConfigFuncs(
					kubernetes.WithClientConnectionOptions(clientConnectionConfig),
					kubernetes.WithClientOptions(client.Options{Scheme: kubernetes.SeedScheme}),
				))
				return fakeCS, nil
			}

			cs, err := cm.GetClient(ctx, key)
			Expect(err).NotTo(HaveOccurred())
			Expect(cs).To(BeIdenticalTo(fakeCS))
		})

		It("should fail if GetGardenClient fails", func() {
			fakeErr := fmt.Errorf("fake")
			factory.GetGardenClient = func(ctx context.Context) (kubernetes.Interface, error) {
				return nil, fakeErr
			}

			cs, err := cm.GetClient(ctx, key)
			Expect(cs).To(BeNil())
			Expect(err).To(MatchError(ContainSubstring("failed to get garden client: fake")))
		})

		It("should fail if it cannot get Seed object", func() {
			c.EXPECT().Get(ctx, client.ObjectKey{Name: seed.Name}, gomock.AssignableToTypeOf(&gardencorev1beta1.Seed{})).
				Return(apierrors.NewNotFound(gardencorev1beta1.Resource("seed"), seed.Name))

			cs, err := cm.GetClient(ctx, key)
			Expect(cs).To(BeNil())
			Expect(err).To(MatchError(ContainSubstring("failed to get Seed object")))
		})

		It("should fail if Seed object does not have a secretRef", func() {
			seed.Spec.SecretRef = nil
			c.EXPECT().Get(ctx, client.ObjectKey{Name: seed.Name}, gomock.AssignableToTypeOf(&gardencorev1beta1.Seed{})).
				DoAndReturn(func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
					seed.DeepCopyInto(obj.(*gardencorev1beta1.Seed))
					return nil
				})

			cs, err := cm.GetClient(ctx, key)
			Expect(cs).To(BeNil())
			Expect(err).To(MatchError(ContainSubstring("does not have a secretRef")))
		})

		It("should fail if NewClientFromSecret fails", func() {
			fakeErr := fmt.Errorf("fake")
			c.EXPECT().Get(ctx, client.ObjectKey{Name: seed.Name}, gomock.AssignableToTypeOf(&gardencorev1beta1.Seed{})).
				DoAndReturn(func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
					seed.DeepCopyInto(obj.(*gardencorev1beta1.Seed))
					return nil
				})
			internal.NewClientFromSecret = func(ctx context.Context, c client.Client, namespace, secretName string, fns ...kubernetes.ConfigFunc) (kubernetes.Interface, error) {
				return nil, fakeErr
			}

			cs, err := cm.GetClient(ctx, key)
			Expect(cs).To(BeNil())
			Expect(err).To(MatchError(fmt.Sprintf("error creating new ClientSet for key %q: fake", key.Key())))
		})

		It("should correctly construct a new ClientSet", func() {
			fakeCS := fakeclientset.NewClientSet()
			c.EXPECT().Get(ctx, client.ObjectKey{Name: seed.Name}, gomock.AssignableToTypeOf(&gardencorev1beta1.Seed{})).
				DoAndReturn(func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
					seed.DeepCopyInto(obj.(*gardencorev1beta1.Seed))
					return nil
				}).Times(2)
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: seed.Spec.SecretRef.Namespace, Name: seed.Spec.SecretRef.Name}, gomock.AssignableToTypeOf(&corev1.Secret{})).
				DoAndReturn(func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
					return nil
				})
			internal.NewClientFromSecret = func(ctx context.Context, c client.Client, namespace, secretName string, fns ...kubernetes.ConfigFunc) (kubernetes.Interface, error) {
				Expect(c).To(BeIdenticalTo(fakeGardenClient.Client()))
				Expect(namespace).To(Equal(seed.Spec.SecretRef.Namespace))
				Expect(secretName).To(Equal(seed.Spec.SecretRef.Name))
				Expect(fns).To(kubernetes.ConsistOfConfigFuncs(
					kubernetes.WithClientConnectionOptions(clientConnectionConfig),
					kubernetes.WithClientOptions(client.Options{Scheme: kubernetes.SeedScheme}),
				))
				return fakeCS, nil
			}

			cs, err := cm.GetClient(ctx, key)
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

		It("should fail if getSeedSecretRef fails", func() {
			fakeErr := fmt.Errorf("fake")
			factory.GetGardenClient = func(ctx context.Context) (kubernetes.Interface, error) {
				return nil, fakeErr
			}

			hash, err := factory.CalculateClientSetHash(ctx, key)
			Expect(hash).To(BeEmpty())
			Expect(err).To(MatchError(ContainSubstring("failed to get garden client: fake")))
		})

		It("should fail if Get Seed Secret fails", func() {
			fakeErr := fmt.Errorf("fake")
			c.EXPECT().Get(ctx, client.ObjectKey{Name: seed.Name}, gomock.AssignableToTypeOf(&gardencorev1beta1.Seed{})).
				DoAndReturn(func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
					seed.DeepCopyInto(obj.(*gardencorev1beta1.Seed))
					return nil
				})
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: seed.Spec.SecretRef.Namespace, Name: seed.Spec.SecretRef.Name}, gomock.AssignableToTypeOf(&corev1.Secret{})).
				DoAndReturn(func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
					return fakeErr
				})

			hash, err := factory.CalculateClientSetHash(ctx, key)
			Expect(hash).To(BeEmpty())
			Expect(err).To(MatchError("fake"))
		})

		It("should correctly calculate hash", func() {
			c.EXPECT().Get(ctx, client.ObjectKey{Name: seed.Name}, gomock.AssignableToTypeOf(&gardencorev1beta1.Seed{})).
				DoAndReturn(func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
					seed.DeepCopyInto(obj.(*gardencorev1beta1.Seed))
					return nil
				})
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: seed.Spec.SecretRef.Namespace, Name: seed.Spec.SecretRef.Name}, gomock.AssignableToTypeOf(&corev1.Secret{})).
				DoAndReturn(func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
					(&corev1.Secret{}).DeepCopyInto(obj.(*corev1.Secret))
					return nil
				})

			hash, err := factory.CalculateClientSetHash(ctx, key)
			Expect(hash).To(Equal("e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"))
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
