// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package util_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/gardener/gardener/extensions/pkg/util"
	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
)

var _ = Describe("ImagesContext", func() {
	Describe("#NewCoreImagesContext", func() {
		var (
			imageVersion1 = core.MachineImageVersion{
				Architectures:    []string{v1beta1constants.ArchitectureAMD64},
				ExpirableVersion: core.ExpirableVersion{Version: "1.0.0"},
			}
			imageVersion2 = core.MachineImageVersion{
				Architectures:    []string{v1beta1constants.ArchitectureAMD64},
				ExpirableVersion: core.ExpirableVersion{Version: "2.0.0"},
			}
			imageVersion3 = core.MachineImageVersion{
				Architectures: []string{
					v1beta1constants.ArchitectureAMD64,
					v1beta1constants.ArchitectureARM64,
				},
				ExpirableVersion: core.ExpirableVersion{Version: "3.0.0"},
			}
		)

		It("should successfully construct an ImagesContext from core.MachineImage slice", func() {
			imagesContext := util.NewCoreImagesContext([]core.MachineImage{
				{Name: "image-1", Versions: []core.MachineImageVersion{imageVersion1, imageVersion2}},
				{Name: "image-2", Versions: []core.MachineImageVersion{imageVersion3}},
			})

			i, exists := imagesContext.GetImage("image-1")
			Expect(exists).To(BeTrue())
			Expect(i.Name).To(Equal("image-1"))
			Expect(i.Versions).To(ConsistOf(imageVersion1, imageVersion2))

			i, exists = imagesContext.GetImage("image-2")
			Expect(exists).To(BeTrue())
			Expect(i.Name).To(Equal("image-2"))
			Expect(i.Versions).To(ConsistOf(imageVersion3))

			i, exists = imagesContext.GetImage("image-99")
			Expect(exists).To(BeFalse())
			Expect(i.Name).To(Equal(""))
			Expect(i.Versions).To(BeEmpty())

			v, exists := imagesContext.GetImageVersionAnyArchitecture("image-99", "1.0.0")
			Expect(exists).To(BeFalse())
			Expect(v).To(Equal(core.MachineImageVersion{}))

			v, exists = imagesContext.GetImageVersionAnyArchitecture("image-1", "99.0.0")
			Expect(exists).To(BeFalse())
			Expect(v).To(Equal(core.MachineImageVersion{}))

			v, exists = imagesContext.GetImageVersionAnyArchitecture("image-2", "3.0.0")
			Expect(exists).To(BeTrue())
			Expect(v).To(Equal(imageVersion3))

			v, exists = imagesContext.GetImageVersion("image-1", "1.0.0", v1beta1constants.ArchitectureAMD64)
			Expect(exists).To(BeTrue())
			Expect(v).To(Equal(imageVersion1))

			v, exists = imagesContext.GetImageVersion("image-1", "1.0.0", v1beta1constants.ArchitectureARM64)
			Expect(exists).To(BeFalse())
			Expect(v).To(Equal(core.MachineImageVersion{}))

			v, exists = imagesContext.GetImageVersion("image-1", "99.0.0", v1beta1constants.ArchitectureAMD64)
			Expect(exists).To(BeFalse())
			Expect(v).To(Equal(core.MachineImageVersion{}))

			v, exists = imagesContext.GetImageVersion("image-99", "99.0.0", v1beta1constants.ArchitectureAMD64)
			Expect(exists).To(BeFalse())
			Expect(v).To(Equal(core.MachineImageVersion{}))
		})
	})

	Describe("#NewV1beta1ImagesContext", func() {
		var (
			imageVersion1 = v1beta1.MachineImageVersion{
				Architectures:    []string{v1beta1constants.ArchitectureAMD64},
				ExpirableVersion: v1beta1.ExpirableVersion{Version: "1.0.0"},
			}
			imageVersion2 = v1beta1.MachineImageVersion{
				Architectures:    []string{v1beta1constants.ArchitectureAMD64},
				ExpirableVersion: v1beta1.ExpirableVersion{Version: "2.0.0"},
			}
			imageVersion3 = v1beta1.MachineImageVersion{
				Architectures: []string{
					v1beta1constants.ArchitectureAMD64,
					v1beta1constants.ArchitectureARM64,
				},
				ExpirableVersion: v1beta1.ExpirableVersion{Version: "3.0.0"},
			}
		)

		It("should successfully construct an ImagesContext from v1beta1.MachineImage slice", func() {
			imagesContext := util.NewV1beta1ImagesContext([]v1beta1.MachineImage{
				{Name: "image-1", Versions: []v1beta1.MachineImageVersion{imageVersion1, imageVersion2}},
				{Name: "image-2", Versions: []v1beta1.MachineImageVersion{imageVersion3}},
			})

			i, exists := imagesContext.GetImage("image-1")
			Expect(exists).To(BeTrue())
			Expect(i.Name).To(Equal("image-1"))
			Expect(i.Versions).To(ConsistOf(imageVersion1, imageVersion2))

			i, exists = imagesContext.GetImage("image-2")
			Expect(exists).To(BeTrue())
			Expect(i.Name).To(Equal("image-2"))
			Expect(i.Versions).To(ConsistOf(imageVersion3))

			i, exists = imagesContext.GetImage("image-99")
			Expect(exists).To(BeFalse())
			Expect(i.Name).To(Equal(""))
			Expect(i.Versions).To(BeEmpty())

			v, exists := imagesContext.GetImageVersionAnyArchitecture("image-99", "1.0.0")
			Expect(exists).To(BeFalse())
			Expect(v).To(Equal(v1beta1.MachineImageVersion{}))

			v, exists = imagesContext.GetImageVersionAnyArchitecture("image-1", "99.0.0")
			Expect(exists).To(BeFalse())
			Expect(v).To(Equal(v1beta1.MachineImageVersion{}))

			v, exists = imagesContext.GetImageVersionAnyArchitecture("image-2", "3.0.0")
			Expect(exists).To(BeTrue())
			Expect(v).To(Equal(imageVersion3))

			v, exists = imagesContext.GetImageVersion("image-1", "1.0.0", v1beta1constants.ArchitectureAMD64)
			Expect(exists).To(BeTrue())
			Expect(v).To(Equal(imageVersion1))

			v, exists = imagesContext.GetImageVersion("image-1", "1.0.0", v1beta1constants.ArchitectureARM64)
			Expect(exists).To(BeFalse())
			Expect(v).To(Equal(v1beta1.MachineImageVersion{}))

			v, exists = imagesContext.GetImageVersion("image-1", "99.0.0", v1beta1constants.ArchitectureAMD64)
			Expect(exists).To(BeFalse())
			Expect(v).To(Equal(v1beta1.MachineImageVersion{}))

			v, exists = imagesContext.GetImageVersion("image-99", "99.0.0", v1beta1constants.ArchitectureAMD64)
			Expect(exists).To(BeFalse())
			Expect(v).To(Equal(v1beta1.MachineImageVersion{}))
		})
	})
})
