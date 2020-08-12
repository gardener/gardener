// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package infodata_test

import (
	"encoding/json"

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	gardencorev1alpha1helper "github.com/gardener/gardener/pkg/apis/core/v1alpha1/helper"
	"k8s.io/apimachinery/pkg/runtime"

	. "github.com/gardener/gardener/pkg/utils/infodata"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type TestInfoData struct {
	Name string `json:"name"`
}

func (t *TestInfoData) Marshal() ([]byte, error) {
	return json.Marshal(t)
}

const TestInfoDataType = "testInfoDataType"

func (t *TestInfoData) TypeVersion() TypeVersion {
	return TypeVersion(TestInfoDataType)
}

var _ = Describe("InfoData", func() {
	Describe("Register and Get Unmarshaller", func() {
		It("should register and then return unmarshaller properly", func() {
			typeVersion := TypeVersion("testRegisterAndUnregister")
			passed := false

			unmarshaller := func(data []byte) (InfoData, error) {
				passed = true
				return nil, nil
			}

			Register(typeVersion, unmarshaller)
			registeredUnmarshaller := GetUnmarshaller(typeVersion)
			_, err := registeredUnmarshaller(nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(passed).To(BeTrue())
		})
	})

	Context("InfoData marshalling and unmarshalling", func() {
		var expectedInfoData *TestInfoData

		Register(TestInfoDataType, func(data []byte) (InfoData, error) {
			infoData := &TestInfoData{}
			err := json.Unmarshal(data, infoData)
			return infoData, err
		})

		BeforeEach(func() {
			expectedInfoData = &TestInfoData{
				Name: "value",
			}
		})

		Describe("#Unmarshal", func() {
			It("should unmarshal gardener data entry", func() {
				gardenerResourceDataEntry := &gardencorev1alpha1.GardenerResourceData{
					Name: "testResourceData",
					Type: TestInfoDataType,
					Data: runtime.RawExtension{Raw: []byte("{\"name\":\"value\"}")},
				}

				infoData, err := Unmarshal(gardenerResourceDataEntry)
				Expect(err).NotTo(HaveOccurred())
				Expect(infoData).To(Equal(expectedInfoData))
			})

			It("should return error if there is no unmarshaller for gardener data entry type", func() {
				gardenerResourceDataEntry := &gardencorev1alpha1.GardenerResourceData{
					Name: "testResourceData",
					Type: "invalidType",
					Data: runtime.RawExtension{Raw: []byte("{\"name\":\"value\"}")},
				}

				_, err := Unmarshal(gardenerResourceDataEntry)
				Expect(err).To(HaveOccurred())
			})

			It("should return error if gardener data entry is not the correct format", func() {
				gardenerResourceDataEntry := &gardencorev1alpha1.GardenerResourceData{
					Name: "testResourceData",
					Type: "testInfoDataType",
					Data: runtime.RawExtension{Raw: []byte("\"name\":\"value\"")},
				}

				_, err := Unmarshal(gardenerResourceDataEntry)
				Expect(err).To(HaveOccurred())
			})

		})

		Describe("#GetInfoData", func() {
			It("should return unmarshalled infodata", func() {
				gardenerResourceDataList := []gardencorev1alpha1.GardenerResourceData{
					{
						Name: "testResourceData",
						Type: "testInfoDataType",
						Data: runtime.RawExtension{Raw: []byte("{\"name\":\"value\"}")},
					},
				}

				infoData, err := GetInfoData(gardenerResourceDataList, "testResourceData")
				Expect(err).NotTo(HaveOccurred())
				Expect(infoData).To(Equal(expectedInfoData))
			})

			It("should return nil if gardener entry cannot be found", func() {
				gardenerResourceDataList := []gardencorev1alpha1.GardenerResourceData{}

				infoData, err := GetInfoData(gardenerResourceDataList, "testResourceData")
				Expect(err).NotTo(HaveOccurred())
				Expect(infoData).To(BeNil())
			})
		})

		Describe("#UpsertInfoData", func() {
			It("should marshal and upsert infodata into gardener resource data list  ", func() {
				gardenerResourceDataList := gardencorev1alpha1helper.GardenerResourceDataList([]gardencorev1alpha1.GardenerResourceData{})

				err := UpsertInfoData(&gardenerResourceDataList, "testResourceData", expectedInfoData)
				Expect(err).NotTo(HaveOccurred())
				Expect(len(gardenerResourceDataList)).To(Equal(1))
			})

			It("should not do anything if provided infodata is emptyInfoData", func() {
				gardenerResourceDataList := gardencorev1alpha1helper.GardenerResourceDataList([]gardencorev1alpha1.GardenerResourceData{})

				err := UpsertInfoData(&gardenerResourceDataList, "emptyData", EmptyInfoData)
				Expect(err).NotTo(HaveOccurred())
				Expect(len(gardenerResourceDataList)).To(Equal(0))
			})
		})
	})
})
