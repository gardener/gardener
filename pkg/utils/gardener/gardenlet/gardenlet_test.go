// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardenlet_test

import (
	"context"
	"os"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/gardener/gardener/pkg/apis/seedmanagement/encoding"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	. "github.com/gardener/gardener/pkg/utils/gardener/gardenlet"
	mockclient "github.com/gardener/gardener/third_party/mock/controller-runtime/client"
)

var _ = Describe("Gardenlet", func() {
	Describe("#SeedIsGarden", func() {
		var (
			ctx        context.Context
			mockReader *mockclient.MockReader
			ctrl       *gomock.Controller
		)

		BeforeEach(func() {
			ctx = context.Background()
			ctrl = gomock.NewController(GinkgoT())
			mockReader = mockclient.NewMockReader(ctrl)
		})

		AfterEach(func() {
			ctrl.Finish()
		})

		It("should return that seed is a garden cluster", func() {
			mockReader.EXPECT().List(ctx, gomock.AssignableToTypeOf(&metav1.PartialObjectMetadataList{}), client.Limit(1)).DoAndReturn(
				func(_ context.Context, list *metav1.PartialObjectMetadataList, _ ...client.ListOption) error {
					list.Items = []metav1.PartialObjectMetadata{{}}
					return nil
				})
			Expect(SeedIsGarden(ctx, mockReader)).To(BeTrue())
		})

		It("should return that seed is a not a garden cluster because no garden object found", func() {
			mockReader.EXPECT().List(ctx, gomock.AssignableToTypeOf(&metav1.PartialObjectMetadataList{}), client.Limit(1))
			Expect(SeedIsGarden(ctx, mockReader)).To(BeFalse())
		})

		It("should return that seed is a not a garden cluster because of a no match error", func() {
			mockReader.EXPECT().List(ctx, gomock.AssignableToTypeOf(&metav1.PartialObjectMetadataList{}), client.Limit(1)).DoAndReturn(
				func(_ context.Context, _ *metav1.PartialObjectMetadataList, _ ...client.ListOption) error {
					return &meta.NoResourceMatchError{}
				})
			Expect(SeedIsGarden(ctx, mockReader)).To(BeFalse())
		})
	})

	Describe("#SetDefaultGardenClusterAddress", func() {
		var (
			log logr.Logger

			gardenClusterAddress string
			gardenletConfig      *gardenletconfigv1alpha1.GardenletConfiguration
			gardenletConfigRaw   *runtime.RawExtension
		)

		BeforeEach(func() {
			log = logr.Discard()

			gardenClusterAddress = "foobar"
		})

		JustBeforeEach(func() {
			var err error
			gardenletConfigRaw, err = encoding.EncodeGardenletConfiguration(gardenletConfig)
			Expect(err).NotTo(HaveOccurred())
		})

		When("GardenClusterAddress is not set", func() {
			BeforeEach(func() {
				gardenletConfig = &gardenletconfigv1alpha1.GardenletConfiguration{}
			})

			It("should set the default garden cluster address", func() {
				newGardenletConfigRaw, err := SetDefaultGardenClusterAddress(log, *gardenletConfigRaw, gardenClusterAddress)
				Expect(err).NotTo(HaveOccurred())
				newGardenletConfig, err := encoding.DecodeGardenletConfiguration(&newGardenletConfigRaw, false)
				Expect(err).NotTo(HaveOccurred())
				Expect(newGardenletConfig.GardenClientConnection).NotTo(BeNil())
				Expect(newGardenletConfig.GardenClientConnection.GardenClusterAddress).To(Equal(&gardenClusterAddress))
			})
		})

		When("GardenClusterAddress is set", func() {
			BeforeEach(func() {
				gardenletConfig = &gardenletconfigv1alpha1.GardenletConfiguration{
					GardenClientConnection: &gardenletconfigv1alpha1.GardenClientConnection{
						GardenClusterAddress: ptr.To("existing-address"),
					},
				}
			})

			It("should not change the garden cluster address", func() {
				newGardenletConfigRaw, err := SetDefaultGardenClusterAddress(log, *gardenletConfigRaw, gardenClusterAddress)
				Expect(err).NotTo(HaveOccurred())
				newGardenletConfig, err := encoding.DecodeGardenletConfiguration(&newGardenletConfigRaw, false)
				Expect(err).NotTo(HaveOccurred())
				Expect(newGardenletConfig.GardenClientConnection).NotTo(BeNil())
				Expect(newGardenletConfig.GardenClientConnection.GardenClusterAddress).To(Equal(ptr.To("existing-address")))
			})
		})
	})

	Describe("#IsResponsibleForSelfHostedShoot", func() {
		It("should return true because the NAMESPACE environment variable is set to 'kube-system'", func() {
			Expect(os.Setenv("NAMESPACE", "kube-system")).To(Succeed())
			DeferCleanup(func() { Expect(os.Setenv("NAMESPACE", "")).To(Succeed()) })

			Expect(IsResponsibleForSelfHostedShoot()).To(BeTrue())
		})

		It("should return false because the NAMESPACE environment variable is not set to 'kube-system'", func() {
			Expect(IsResponsibleForSelfHostedShoot()).To(BeFalse())
		})
	})

	Describe("#ShootMetaFromBootstrapToken", func() {
		var (
			ctx        = context.Background()
			fakeClient client.Client

			bootstrapTokenSecretName string
			expectedShootNamespace   string
			expectedShootName        string
			expectedNamespacedName   types.NamespacedName
		)

		BeforeEach(func() {
			fakeClient = fake.NewClientBuilder().Build()

			bootstrapTokenSecretName = "bootstrap-token-123456"
			expectedShootNamespace = "garden-my-project"
			expectedShootName = "my-shoot"
			expectedNamespacedName = types.NamespacedName{
				Namespace: expectedShootNamespace,
				Name:      expectedShootName,
			}
		})

		It("should successfully extract shoot meta from bootstrap token", func() {
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      bootstrapTokenSecretName,
					Namespace: "kube-system",
				},
				Data: map[string][]byte{
					"description": []byte("Used for connecting the self-hosted Shoot " + expectedShootNamespace + "/" + expectedShootName + " to Gardener via 'gardenadm connect'"),
				},
			}

			Expect(fakeClient.Create(ctx, secret)).To(Succeed())

			result, err := ShootMetaFromBootstrapToken(ctx, fakeClient, bootstrapTokenSecretName)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(expectedNamespacedName))
		})

		It("should return error when bootstrap token secret is not found", func() {
			result, err := ShootMetaFromBootstrapToken(ctx, fakeClient, bootstrapTokenSecretName)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to read bootstrap token secret"))
			Expect(result).To(Equal(types.NamespacedName{}))
		})

		It("should return error when description does not start with required prefix", func() {
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      bootstrapTokenSecretName,
					Namespace: "kube-system",
				},
				Data: map[string][]byte{
					"description": []byte("Invalid prefix " + expectedShootNamespace + "/" + expectedShootName),
				},
			}

			Expect(fakeClient.Create(ctx, secret)).To(Succeed())

			result, err := ShootMetaFromBootstrapToken(ctx, fakeClient, bootstrapTokenSecretName)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("bootstrap token description does not start with"))
			Expect(result).To(Equal(types.NamespacedName{}))
		})

		It("should return error when description has no shoot meta after prefix", func() {
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      bootstrapTokenSecretName,
					Namespace: "kube-system",
				},
				Data: map[string][]byte{
					"description": []byte("Used for connecting the self-hosted Shoot "),
				},
			}

			Expect(fakeClient.Create(ctx, secret)).To(Succeed())

			result, err := ShootMetaFromBootstrapToken(ctx, fakeClient, bootstrapTokenSecretName)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("could not extract shoot meta from bootstrap token description"))
			Expect(result).To(Equal(types.NamespacedName{}))
		})

		It("should return error when description has only whitespace after prefix", func() {
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      bootstrapTokenSecretName,
					Namespace: "kube-system",
				},
				Data: map[string][]byte{
					"description": []byte("Used for connecting the self-hosted Shoot    "),
				},
			}

			Expect(fakeClient.Create(ctx, secret)).To(Succeed())

			result, err := ShootMetaFromBootstrapToken(ctx, fakeClient, bootstrapTokenSecretName)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("could not extract shoot meta from bootstrap token description"))
			Expect(result).To(Equal(types.NamespacedName{}))
		})

		It("should return error when shoot meta format is invalid (no slash)", func() {
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      bootstrapTokenSecretName,
					Namespace: "kube-system",
				},
				Data: map[string][]byte{
					"description": []byte("Used for connecting the self-hosted Shoot invalid-format-no-slash"),
				},
			}

			Expect(fakeClient.Create(ctx, secret)).To(Succeed())

			result, err := ShootMetaFromBootstrapToken(ctx, fakeClient, bootstrapTokenSecretName)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("could not extract shoot namespace and name from bootstrap token description"))
			Expect(result).To(Equal(types.NamespacedName{}))
		})

		It("should return error when shoot meta format has multiple slashes", func() {
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      bootstrapTokenSecretName,
					Namespace: "kube-system",
				},
				Data: map[string][]byte{
					"description": []byte("Used for connecting the self-hosted Shoot namespace/shoot/extra"),
				},
			}

			Expect(fakeClient.Create(ctx, secret)).To(Succeed())

			result, err := ShootMetaFromBootstrapToken(ctx, fakeClient, bootstrapTokenSecretName)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("could not extract shoot namespace and name from bootstrap token description"))
			Expect(result).To(Equal(types.NamespacedName{}))
		})

		It("should return error when shoot meta format has empty namespace", func() {
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      bootstrapTokenSecretName,
					Namespace: "kube-system",
				},
				Data: map[string][]byte{
					"description": []byte("Used for connecting the self-hosted Shoot /my-shoot"),
				},
			}

			Expect(fakeClient.Create(ctx, secret)).To(Succeed())

			result, err := ShootMetaFromBootstrapToken(ctx, fakeClient, bootstrapTokenSecretName)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(types.NamespacedName{Namespace: "", Name: "my-shoot"}))
		})

		It("should return error when shoot meta format has empty name", func() {
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      bootstrapTokenSecretName,
					Namespace: "kube-system",
				},
				Data: map[string][]byte{
					"description": []byte("Used for connecting the self-hosted Shoot my-namespace/"),
				},
			}

			Expect(fakeClient.Create(ctx, secret)).To(Succeed())

			result, err := ShootMetaFromBootstrapToken(ctx, fakeClient, bootstrapTokenSecretName)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(types.NamespacedName{Namespace: "my-namespace", Name: ""}))
		})

		It("should handle description with additional text after shoot meta", func() {
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      bootstrapTokenSecretName,
					Namespace: "kube-system",
				},
				Data: map[string][]byte{
					"description": []byte("Used for connecting the self-hosted Shoot " + expectedShootNamespace + "/" + expectedShootName + " additional text here"),
				},
			}

			Expect(fakeClient.Create(ctx, secret)).To(Succeed())

			result, err := ShootMetaFromBootstrapToken(ctx, fakeClient, bootstrapTokenSecretName)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(expectedNamespacedName))
		})

		It("should return error when description key is missing", func() {
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      bootstrapTokenSecretName,
					Namespace: "kube-system",
				},
				Data: map[string][]byte{
					"other-key": []byte("some-value"),
				},
			}

			Expect(fakeClient.Create(ctx, secret)).To(Succeed())

			result, err := ShootMetaFromBootstrapToken(ctx, fakeClient, bootstrapTokenSecretName)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("bootstrap token description does not start with"))
			Expect(result).To(Equal(types.NamespacedName{}))
		})
	})
})
