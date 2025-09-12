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
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

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

	Describe("#IsResponsibleForAutonomousShoot", func() {
		It("should return true because the NAMESPACE environment variable is set to 'kube-system'", func() {
			Expect(os.Setenv("NAMESPACE", "kube-system")).To(Succeed())
			DeferCleanup(func() { Expect(os.Setenv("NAMESPACE", "")).To(Succeed()) })

			Expect(IsResponsibleForAutonomousShoot()).To(BeTrue())
		})

		It("should return false because the NAMESPACE environment variable is not set to 'kube-system'", func() {
			Expect(IsResponsibleForAutonomousShoot()).To(BeFalse())
		})
	})
})
