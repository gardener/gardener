// Copyright 2024 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package webhook_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/extensions/pkg/webhook"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

var _ = Describe("Utils", func() {

	Describe("#EnsureFileWithPath", func() {
		var files []extensionsv1alpha1.File

		BeforeEach(func() {
			files = []extensionsv1alpha1.File{
				{
					Path: "/foo.txt",
					Content: extensionsv1alpha1.FileContent{
						Inline: &extensionsv1alpha1.FileContentInline{
							Data: "foo",
						},
					},
				},
				{
					Path: "/bar.txt",
					Content: extensionsv1alpha1.FileContent{
						Inline: &extensionsv1alpha1.FileContentInline{
							Data: "bar",
						},
					},
				},
			}
		})

		It("should append file when file with such path does not exist", func() {
			newFile := extensionsv1alpha1.File{
				Path: "/baz.txt",
				Content: extensionsv1alpha1.FileContent{
					Inline: &extensionsv1alpha1.FileContentInline{
						Data: "baz",
					},
				},
			}

			actual := webhook.EnsureFileWithPath(files, newFile)
			Expect(actual).To(Equal(append(files, newFile)))
		})

		It("should update file when file with such path exists", func() {
			newFile := extensionsv1alpha1.File{
				Path: "/foo.txt",
				Content: extensionsv1alpha1.FileContent{
					Inline: &extensionsv1alpha1.FileContentInline{
						Data: "baz",
					},
				},
			}

			actual := webhook.EnsureFileWithPath(files, newFile)
			Expect(actual).To(Equal([]extensionsv1alpha1.File{
				{
					Path: "/foo.txt",
					Content: extensionsv1alpha1.FileContent{
						Inline: &extensionsv1alpha1.FileContentInline{
							Data: "baz",
						},
					},
				},
				{
					Path: "/bar.txt",
					Content: extensionsv1alpha1.FileContent{
						Inline: &extensionsv1alpha1.FileContentInline{
							Data: "bar",
						},
					},
				},
			}))
		})

		It("should do nothing when the new file is exactly the same as the existing one", func() {
			newFile := files[0]

			actual := webhook.EnsureFileWithPath(files, newFile)
			Expect(actual).To(Equal(files))
		})
	})

	Describe("#EnsureUnitWithName", func() {
		var units []extensionsv1alpha1.Unit

		BeforeEach(func() {
			units = []extensionsv1alpha1.Unit{
				{
					Name:    "foo.service",
					Content: ptr.To("foo"),
				},
				{
					Name:    "bar.service",
					Content: ptr.To("bar"),
				},
			}
		})

		It("should append unit when unit with such name does not exist", func() {
			newUnit := extensionsv1alpha1.Unit{
				Name:    "baz.service",
				Content: ptr.To("bar"),
			}

			actual := webhook.EnsureUnitWithName(units, newUnit)
			Expect(actual).To(Equal(append(units, newUnit)))
		})

		It("should update unit when unit with such name exists", func() {
			newUnit := extensionsv1alpha1.Unit{
				Name:    "foo.service",
				Content: ptr.To("baz"),
			}

			actual := webhook.EnsureUnitWithName(units, newUnit)
			Expect(actual).To(Equal([]extensionsv1alpha1.Unit{
				{
					Name:    "foo.service",
					Content: ptr.To("baz"),
				},
				{
					Name:    "bar.service",
					Content: ptr.To("bar"),
				},
			}))
		})

		It("should do nothing when the new unit is exactly the same as the existing one", func() {
			newUnit := units[0]

			actual := webhook.EnsureUnitWithName(units, newUnit)
			Expect(actual).To(Equal(units))
		})
	})
})
