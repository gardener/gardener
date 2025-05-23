// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package util_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/gardener/gardener/extensions/pkg/util"
	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

var _ = Describe("ImagesContext", func() {
	Describe("#NewCoreImagesContext", func() {
		It("should successfully construct an ImagesContext from core.MachineImage slice", func() {
			imagesContext := util.NewCoreImagesContext([]core.MachineImage{
				{Name: "image-1", Versions: []core.MachineImageVersion{
					{ExpirableVersion: core.ExpirableVersion{Version: "1.0.0"}},
					{ExpirableVersion: core.ExpirableVersion{Version: "2.0.0"}},
				}},
				{Name: "image-2", Versions: []core.MachineImageVersion{
					{ExpirableVersion: core.ExpirableVersion{Version: "3.0.0"}},
				}},
			})

			i, exists := imagesContext.GetImage("image-1")
			Expect(exists).To(BeTrue())
			Expect(i.Name).To(Equal("image-1"))
			Expect(i.Versions).To(ConsistOf(
				core.MachineImageVersion{ExpirableVersion: core.ExpirableVersion{Version: "1.0.0"}},
				core.MachineImageVersion{ExpirableVersion: core.ExpirableVersion{Version: "2.0.0"}},
			))

			i, exists = imagesContext.GetImage("image-2")
			Expect(exists).To(BeTrue())
			Expect(i.Name).To(Equal("image-2"))
			Expect(i.Versions).To(ConsistOf(
				core.MachineImageVersion{ExpirableVersion: core.ExpirableVersion{Version: "3.0.0"}},
			))

			i, exists = imagesContext.GetImage("image-99")
			Expect(exists).To(BeFalse())
			Expect(i.Name).To(Equal(""))
			Expect(i.Versions).To(BeEmpty())

			v, exists := imagesContext.GetImageVersion("image-1", "1.0.0")
			Expect(exists).To(BeTrue())
			Expect(v).To(Equal(core.MachineImageVersion{ExpirableVersion: core.ExpirableVersion{Version: "1.0.0"}}))

			v, exists = imagesContext.GetImageVersion("image-1", "99.0.0")
			Expect(exists).To(BeFalse())
			Expect(v).To(Equal(core.MachineImageVersion{}))

			v, exists = imagesContext.GetImageVersion("image-99", "99.0.0")
			Expect(exists).To(BeFalse())
			Expect(v).To(Equal(core.MachineImageVersion{}))
		})
	})

	Describe("#NewV1beta1ImagesContext", func() {
		It("should successfully construct an ImagesContext from v1beta1.MachineImage slice", func() {
			imagesContext := util.NewV1beta1ImagesContext([]v1beta1.MachineImage{
				{Name: "image-1", Versions: []v1beta1.MachineImageVersion{
					{ExpirableVersion: v1beta1.ExpirableVersion{Version: "1.0.0"}},
					{ExpirableVersion: v1beta1.ExpirableVersion{Version: "2.0.0"}},
				}},
				{Name: "image-2", Versions: []v1beta1.MachineImageVersion{
					{ExpirableVersion: v1beta1.ExpirableVersion{Version: "3.0.0"}},
				}},
			})

			i, exists := imagesContext.GetImage("image-1")
			Expect(exists).To(BeTrue())
			Expect(i.Name).To(Equal("image-1"))
			Expect(i.Versions).To(ConsistOf(
				v1beta1.MachineImageVersion{ExpirableVersion: v1beta1.ExpirableVersion{Version: "1.0.0"}},
				v1beta1.MachineImageVersion{ExpirableVersion: v1beta1.ExpirableVersion{Version: "2.0.0"}},
			))

			i, exists = imagesContext.GetImage("image-2")
			Expect(exists).To(BeTrue())
			Expect(i.Name).To(Equal("image-2"))
			Expect(i.Versions).To(ConsistOf(
				v1beta1.MachineImageVersion{ExpirableVersion: v1beta1.ExpirableVersion{Version: "3.0.0"}},
			))

			i, exists = imagesContext.GetImage("image-99")
			Expect(exists).To(BeFalse())
			Expect(i.Name).To(Equal(""))
			Expect(i.Versions).To(BeEmpty())

			v, exists := imagesContext.GetImageVersion("image-1", "1.0.0")
			Expect(exists).To(BeTrue())
			Expect(v).To(Equal(v1beta1.MachineImageVersion{ExpirableVersion: v1beta1.ExpirableVersion{Version: "1.0.0"}}))

			v, exists = imagesContext.GetImageVersion("image-1", "99.0.0")
			Expect(exists).To(BeFalse())
			Expect(v).To(Equal(v1beta1.MachineImageVersion{}))

			v, exists = imagesContext.GetImageVersion("image-99", "99.0.0")
			Expect(exists).To(BeFalse())
			Expect(v).To(Equal(v1beta1.MachineImageVersion{}))
		})
	})
})
