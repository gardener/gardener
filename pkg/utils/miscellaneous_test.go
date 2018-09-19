// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
package utils_test

import (
	"time"

	. "github.com/gardener/gardener/pkg/utils"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("utils", func() {
	Describe("#ParseMaintenanceTime", func() {
		It("should return the time object in UTC", func() {
			time, err := ParseMaintenanceTime("222200+0100")

			Expect(err).NotTo(HaveOccurred())
			Expect(time.String()).To(ContainSubstring("21:22:00 +0000"))
		})

		It("should return an error", func() {
			_, err := ParseMaintenanceTime("abcinvalidformat")

			Expect(err).To(HaveOccurred())
		})
	})

	Describe("#FormatMaintenanceTime", func() {
		It("should return the formatted time", func() {
			cet, _ := time.LoadLocation("CET")
			t := time.Date(1970, 1, 1, 14, 0, 0, 0, cet)

			val := FormatMaintenanceTime(t)

			Expect(val).To(Equal("140000+0100"))
		})
	})

	Describe("#MergeStringMaps", func() {
		It("should return nil", func() {
			result := MergeStringMaps(nil, nil)

			Expect(result).To(BeNil())
		})

		It("should return an empty map", func() {
			emptyMap := map[string]string{}

			result := MergeStringMaps(emptyMap, nil)

			Expect(result).To(Equal(emptyMap))
		})

		It("should return a merged map", func() {
			var (
				oldMap = map[string]string{
					"a": "1",
					"b": "2",
				}
				newMap = map[string]string{
					"b": "20",
					"c": "3",
				}
			)

			result := MergeStringMaps(oldMap, newMap)

			Expect(result).To(Equal(map[string]string{
				"a": "1",
				"b": "20",
				"c": "3",
			}))
		})
	})
})
