// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package extensions_test

import (
	"context"
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	. "github.com/gardener/gardener/pkg/extensions"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Cluster", func() {
	var (
		ctx              = context.TODO()
		fakeGardenClient client.Client
		fakeSeedClient   client.Client

		clusterName          string
		cluster              *extensionsv1alpha1.Cluster
		expectedCloudProfile *gardencorev1beta1.CloudProfile
		expectedSeed         *gardencorev1beta1.Seed
		expectedShoot        *gardencorev1beta1.Shoot
		expectedShootState   *gardencorev1beta1.ShootState
		expectedCluster      *Cluster
	)

	BeforeEach(func() {
		fakeGardenClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).Build()
		fakeSeedClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
	})

	Describe("#GenericTokenKubeconfigSecretNameFromCluster", func() {
		var cluster *Cluster

		BeforeEach(func() {
			cluster = &Cluster{}
		})

		It("should return the deprecated constant name due to missing annotation", func() {
			Expect(GenericTokenKubeconfigSecretNameFromCluster(cluster)).To(Equal("generic-token-kubeconfig"))
		})

		It("should return the name provided in the annotation value", func() {
			name := "generic-token-kubeconfig-12345"
			metav1.SetMetaDataAnnotation(&cluster.ObjectMeta, "generic-token-kubeconfig.secret.gardener.cloud/name", name)

			Expect(GenericTokenKubeconfigSecretNameFromCluster(cluster)).To(Equal(name))
		})
	})

	Describe("#SyncClusterResourceToSeed", func() {
		BeforeEach(func() {
			expectedCloudProfile = &gardencorev1beta1.CloudProfile{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "core.gardener.cloud/v1beta1",
					Kind:       "CloudProfile",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
			}

			expectedSeed = &gardencorev1beta1.Seed{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "core.gardener.cloud/v1beta1",
					Kind:       "Seed",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
			}

			expectedShoot = &gardencorev1beta1.Shoot{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "core.gardener.cloud/v1beta1",
					Kind:       "Shoot",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "garden-bar",
				},
			}

			clusterName = "shoot--" + expectedShoot.Namespace + "--" + expectedShoot.Name
			cluster = &extensionsv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: clusterName,
				},
			}
		})

		It("should sync cloudprofile, shoot and seed to cluster", func() {
			Expect(SyncClusterResourceToSeed(ctx, fakeSeedClient, cluster.Name, expectedShoot, expectedCloudProfile, expectedSeed)).To(Succeed())
			Expect(fakeSeedClient.Get(ctx, client.ObjectKeyFromObject(cluster), cluster)).To(Succeed())

			Expect(cluster.Spec.CloudProfile.Raw).NotTo(BeNil())
			Expect(cluster.Spec.Seed.Raw).NotTo(BeNil())
			Expect(cluster.Spec.Shoot.Raw).NotTo(BeNil())
		})
	})

	Describe("#GetCluster", func() {
		BeforeEach(func() {
			expectedCloudProfile = &gardencorev1beta1.CloudProfile{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "core.gardener.cloud/v1beta1",
					Kind:       "CloudProfile",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
			}

			expectedSeed = &gardencorev1beta1.Seed{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "core.gardener.cloud/v1beta1",
					Kind:       "Seed",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
			}

			expectedShoot = &gardencorev1beta1.Shoot{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "core.gardener.cloud/v1beta1",
					Kind:       "Shoot",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "garden-bar",
				},
			}

			clusterName = "shoot--" + expectedShoot.Namespace + "--" + expectedShoot.Name
			cluster = &extensionsv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: clusterName,
				},
				Spec: extensionsv1alpha1.ClusterSpec{
					CloudProfile: runtime.RawExtension{
						Object: expectedCloudProfile,
					},
					Seed: runtime.RawExtension{
						Object: expectedSeed,
					},
					Shoot: runtime.RawExtension{
						Object: expectedShoot,
					},
				},
			}

			expectedCluster = &Cluster{
				ObjectMeta:   cluster.ObjectMeta,
				CloudProfile: expectedCloudProfile,
				Seed:         expectedSeed,
				Shoot:        expectedShoot,
			}
		})

		It("should return error if cluster is not found", func() {
			cluster, err := GetCluster(ctx, fakeSeedClient, "foo")
			Expect(err).To(MatchError(ContainSubstring("clusters.extensions.gardener.cloud \"foo\" not found")))
			Expect(cluster).To(BeNil())
		})

		It("should get the cluster", func() {
			Expect(fakeSeedClient.Create(ctx, cluster)).To(Succeed())

			cluster, err := GetCluster(ctx, fakeSeedClient, cluster.Name)
			Expect(err).NotTo(HaveOccurred())
			expectedCluster.ObjectMeta.ResourceVersion = "1"
			Expect(cluster).To(Equal(expectedCluster))
		})
	})

	Describe("#CloudProfileFromCluster", func() {
		BeforeEach(func() {
			expectedCloudProfile = &gardencorev1beta1.CloudProfile{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "core.gardener.cloud/v1beta1",
					Kind:       "CloudProfile",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
			}

			cluster = &extensionsv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
				Spec: extensionsv1alpha1.ClusterSpec{
					CloudProfile: runtime.RawExtension{
						Raw: encode(expectedCloudProfile),
					},
				},
			}
		})

		It("should retrieve cloudprofile from cluster", func() {
			cloudProfile, err := CloudProfileFromCluster(cluster)

			Expect(err).NotTo(HaveOccurred())
			Expect(cloudProfile).To(Equal(expectedCloudProfile))
		})

		It("should return an error because the cloudprofile cannot be decoded from the cluster", func() {
			cluster.Spec.CloudProfile.Raw = []byte(`{`)

			cloudProfile, err := CloudProfileFromCluster(cluster)
			Expect(err).To(MatchError(ContainSubstring("unexpected end of JSON input")))
			Expect(cloudProfile).To(BeNil())
		})

		It("should return nil because the cloudprofile is not in raw format in the cluster", func() {
			cluster.Spec.CloudProfile.Raw = nil

			cloudProfile, err := CloudProfileFromCluster(cluster)
			Expect(err).ToNot(HaveOccurred())
			Expect(cloudProfile).To(BeNil())
		})
	})

	Describe("#SeedFromCluster", func() {
		BeforeEach(func() {
			expectedSeed = &gardencorev1beta1.Seed{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "core.gardener.cloud/v1beta1",
					Kind:       "Seed",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
			}

			cluster = &extensionsv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
				Spec: extensionsv1alpha1.ClusterSpec{
					Seed: runtime.RawExtension{
						Raw: encode(expectedSeed),
					},
				},
			}
		})

		It("should retrieve seed from cluster", func() {
			seed, err := SeedFromCluster(cluster)

			Expect(err).NotTo(HaveOccurred())
			Expect(seed).To(Equal(expectedSeed))
		})

		It("should return an error because the seed cannot be decoded from the cluster", func() {
			cluster.Spec.Seed.Raw = []byte(`{`)

			seed, err := SeedFromCluster(cluster)
			Expect(err).To(MatchError(ContainSubstring("unexpected end of JSON input")))
			Expect(seed).To(BeNil())
		})

		It("should return nil because the seed is not in raw format in the cluster", func() {
			cluster.Spec.Seed.Raw = nil

			seed, err := SeedFromCluster(cluster)
			Expect(err).ToNot(HaveOccurred())
			Expect(seed).To(BeNil())
		})
	})

	Describe("#ShootFromCluster", func() {
		BeforeEach(func() {
			expectedShoot = &gardencorev1beta1.Shoot{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "core.gardener.cloud/v1beta1",
					Kind:       "Shoot",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
			}

			cluster = &extensionsv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
				Spec: extensionsv1alpha1.ClusterSpec{
					Shoot: runtime.RawExtension{
						Raw: encode(expectedShoot),
					},
				},
			}
		})

		It("should retrieve shoot from cluster", func() {
			shoot, err := ShootFromCluster(cluster)

			Expect(err).NotTo(HaveOccurred())
			Expect(shoot).To(Equal(expectedShoot))
		})

		It("should return an error because the shoot cannot be decoded from the cluster", func() {
			cluster.Spec.Shoot.Raw = []byte(`{`)

			shoot, err := ShootFromCluster(cluster)
			Expect(err).To(MatchError(ContainSubstring("unexpected end of JSON input")))
			Expect(shoot).To(BeNil())
		})

		It("should return nil because the shoot is not in raw format in the cluster", func() {
			cluster.Spec.Shoot.Raw = nil

			shoot, err := ShootFromCluster(cluster)
			Expect(err).ToNot(HaveOccurred())
			Expect(shoot).To(BeNil())
		})
	})

	Describe("#GetShootStateForCluster", func() {
		BeforeEach(func() {
			expectedShoot = &gardencorev1beta1.Shoot{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "core.gardener.cloud/v1beta1",
					Kind:       "Shoot",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "garden-bar",
				},
			}
			expectedShootState = &gardencorev1beta1.ShootState{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "core.gardener.cloud/v1beta1",
					Kind:       "ShootState",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      expectedShoot.Name,
					Namespace: expectedShoot.Namespace,
				},
			}

			clusterName = "shoot--" + expectedShoot.Namespace + "--" + expectedShoot.Name
			cluster = &extensionsv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: clusterName,
				},
				Spec: extensionsv1alpha1.ClusterSpec{
					Shoot: runtime.RawExtension{
						Object: expectedShoot,
					},
				},
			}
		})

		It("should retrieve both shootstate and shoot", func() {
			Expect(fakeGardenClient.Create(ctx, expectedShoot)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, expectedShootState)).To(Succeed())
			Expect(fakeSeedClient.Create(ctx, cluster)).To(Succeed())

			shootState, shoot, err := GetShootStateForCluster(ctx, fakeGardenClient, fakeSeedClient, clusterName)
			Expect(err).NotTo(HaveOccurred())
			Expect(shootState).To(Equal(expectedShootState))
			Expect(shoot).To(Equal(expectedShoot))
		})

		It("should return an error because the cluster object is not found", func() {
			shootState, shoot, err := GetShootStateForCluster(ctx, fakeGardenClient, fakeSeedClient, clusterName)
			Expect(err).To(BeNotFoundError())
			Expect(shootState).To(BeNil())
			Expect(shoot).To(BeNil())
		})

		It("should return an error because the shoot cannot be decoded from the cluster", func() {
			cluster.Spec.Shoot.Object = nil
			cluster.Spec.Shoot.Raw = []byte(`{`)
			Expect(fakeSeedClient.Create(ctx, cluster)).To(Succeed())

			shootState, shoot, err := GetShootStateForCluster(ctx, fakeGardenClient, fakeSeedClient, clusterName)
			Expect(err).To(MatchError(ContainSubstring("unexpected end of JSON input")))
			Expect(shootState).To(BeNil())
			Expect(shoot).To(BeNil())
		})

		It("should return an error because the shoot is not in raw format the cluster", func() {
			cluster.Spec.Shoot.Object = nil
			Expect(fakeSeedClient.Create(ctx, cluster)).To(Succeed())

			shootState, shoot, err := GetShootStateForCluster(ctx, fakeGardenClient, fakeSeedClient, clusterName)
			Expect(err).To(MatchError(ContainSubstring("doesn't contain shoot resource in raw format")))
			Expect(shootState).To(BeNil())
			Expect(shoot).To(BeNil())
		})

		It("should return an error because the shootstate object is not found", func() {
			Expect(fakeGardenClient.Create(ctx, expectedShoot)).To(Succeed())
			Expect(fakeSeedClient.Create(ctx, cluster)).To(Succeed())

			shootState, shoot, err := GetShootStateForCluster(ctx, fakeGardenClient, fakeSeedClient, clusterName)
			Expect(err).To(BeNotFoundError())
			Expect(shootState).To(BeNil())
			Expect(shoot).To(BeNil())
		})
	})
})

func encode(obj runtime.Object) []byte {
	bytes, err := json.Marshal(obj)
	Expect(err).NotTo(HaveOccurred())
	return bytes
}
