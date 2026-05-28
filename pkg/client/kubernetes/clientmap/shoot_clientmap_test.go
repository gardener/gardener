// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package clientmap_test

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientcmdlatest "k8s.io/client-go/tools/clientcmd/api/latest"
	clientcmdv1 "k8s.io/client-go/tools/clientcmd/api/v1"
	componentbaseconfigv1alpha1 "k8s.io/component-base/config/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	. "github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	. "github.com/gardener/gardener/pkg/client/kubernetes/test"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

var _ = Describe("ShootClientMap", func() {
	var (
		ctx              context.Context
		fakeGardenClient client.Client
		fakeSeedClient   client.Client

		cm                     ClientMap
		key                    ClientSetKey
		factory                *ShootClientSetFactory
		clientConnectionConfig componentbaseconfigv1alpha1.ClientConnectionConfiguration
		clientOptions          client.Options

		shoot *gardencorev1beta1.Shoot
	)

	BeforeEach(func() {
		ctx = context.TODO()
		fakeGardenClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).WithStatusSubresource(&gardencorev1beta1.Shoot{}).Build()
		fakeSeedClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()

		shoot = &gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "garden-eden",
				Name:      "forbidden-fruit",
			},
			Spec: gardencorev1beta1.ShootSpec{
				SeedName: new("apple-seed"),
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
			GardenClient:           fakeGardenClient,
			SeedClient:             fakeSeedClient,
			ClientConnectionConfig: clientConnectionConfig,
		}
		cm = NewShootClientMap(logr.Discard(), factory)
	})

	Context("#GetClient", func() {
		It("should fail if ClientSetKey type is unsupported", func() {
			key = fakeKey{}
			cs, err := cm.GetClient(ctx, key)
			Expect(cs).To(BeNil())
			Expect(err).To(MatchError(ContainSubstring("unsupported ClientSetKey")))
		})

		It("should fail if it cannot get Shoot object", func() {
			cs, err := cm.GetClient(ctx, key)
			Expect(cs).To(BeNil())
			Expect(err).To(MatchError(ContainSubstring("failed to get Shoot object")))
		})

		It("should fail if Shoot is not scheduled yet", func() {
			shoot.Spec.SeedName = nil
			Expect(fakeGardenClient.Create(ctx, shoot)).To(Succeed())

			cs, err := cm.GetClient(ctx, key)
			Expect(cs).To(BeNil())
			Expect(err).To(MatchError(ContainSubstring(fmt.Sprintf("shoot %q is not scheduled yet", key.Key()))))
		})

		It("should fail constructing a new ClientSet (in-cluster) because token is not populated", func() {
			fakeCS := fakekubernetes.NewClientSet()

			Expect(fakeGardenClient.Create(ctx, shoot)).To(Succeed())
			Expect(fakeSeedClient.Create(ctx, &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "gardener-internal",
					Namespace: shoot.Status.TechnicalID,
				},
			})).To(Succeed())

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
			fakeCS := fakekubernetes.NewClientSet()
			changedTechnicalID := "foo"

			Expect(fakeGardenClient.Create(ctx, shoot)).To(Succeed())
			Expect(fakeSeedClient.Create(ctx, &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "gardener-internal",
					Namespace: shoot.Status.TechnicalID,
				},
				Data: dataWithPopulatedToken(),
			})).To(Succeed())

			// Also pre-create the secret for changedTechnicalID namespace (used after InvalidateClient)
			Expect(fakeSeedClient.Create(ctx, &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "gardener-internal",
					Namespace: changedTechnicalID,
				},
				Data: dataWithPopulatedToken(),
			})).To(Succeed())

			NewClientFromSecretObject = func(secret *corev1.Secret, fns ...kubernetes.ConfigFunc) (kubernetes.Interface, error) {
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

			// Update the shoot's TechnicalID for the second call
			updatedShoot := shoot.DeepCopy()
			updatedShoot.Status.TechnicalID = changedTechnicalID
			Expect(fakeGardenClient.Status().Update(ctx, updatedShoot)).To(Succeed())

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
			Expect(fakeGardenClient.Create(ctx, shoot)).To(Succeed())
			// Do NOT create the seed secret - this triggers the error

			hash, err := factory.CalculateClientSetHash(ctx, key)
			Expect(hash).To(BeEmpty())
			Expect(err).To(MatchError("secrets \"gardener-internal\" not found"))
		})

		It("should correctly calculate hash", func() {
			changedTechnicalID := "foo"

			Expect(fakeGardenClient.Create(ctx, shoot)).To(Succeed())
			Expect(fakeSeedClient.Create(ctx, &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "gardener-internal",
					Namespace: shoot.Status.TechnicalID,
				},
			})).To(Succeed())
			Expect(fakeSeedClient.Create(ctx, &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "gardener-internal",
					Namespace: changedTechnicalID,
				},
			})).To(Succeed())

			hash, err := factory.CalculateClientSetHash(ctx, key)
			Expect(hash).To(Equal("e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"))
			Expect(err).NotTo(HaveOccurred())

			Expect(factory.InvalidateClient(key)).To(Succeed())

			// Update shoot's TechnicalID
			updatedShoot := shoot.DeepCopy()
			updatedShoot.Status.TechnicalID = changedTechnicalID
			Expect(fakeGardenClient.Update(ctx, updatedShoot)).To(Succeed())

			hash, err = factory.CalculateClientSetHash(ctx, keys.ForShoot(updatedShoot))
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
