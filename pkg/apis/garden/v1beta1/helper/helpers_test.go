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

package helper_test

import (
	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	. "github.com/gardener/gardener/pkg/apis/garden/v1beta1/helper"
	"github.com/gardener/gardener/pkg/operation/common"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("helper", func() {
	Describe("#GetCondition", func() {
		It("should return the found condition", func() {
			var (
				conditionType gardenv1beta1.ConditionType = "test-1"
				condition                                 = gardenv1beta1.Condition{
					Type: conditionType,
				}
				conditions = []gardenv1beta1.Condition{condition}
			)

			cond := GetCondition(conditions, conditionType)

			Expect(cond).NotTo(BeNil())
			Expect(*cond).To(Equal(condition))
		})

		It("should return nil because the required condition could not be found", func() {
			var (
				conditionType gardenv1beta1.ConditionType = "test-1"
				conditions                                = []gardenv1beta1.Condition{}
			)

			cond := GetCondition(conditions, conditionType)

			Expect(cond).To(BeNil())
		})
	})

	Describe("#IsUsedAsSeed", func() {
		var shoot *gardenv1beta1.Shoot

		BeforeEach(func() {
			shoot = &gardenv1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:   common.GardenNamespace,
					Annotations: nil,
				},
			}
		})

		It("should return false,nil,nil because shoot is not in the garden namespace", func() {
			shoot.Namespace = "default"

			useAsSeed, protected, visible := IsUsedAsSeed(shoot)

			Expect(useAsSeed).To(BeFalse())
			Expect(protected).To(BeNil())
			Expect(visible).To(BeNil())
		})

		It("should return false,nil,nil because annotation is not set", func() {
			useAsSeed, protected, visible := IsUsedAsSeed(shoot)

			Expect(useAsSeed).To(BeFalse())
			Expect(protected).To(BeNil())
			Expect(visible).To(BeNil())
		})

		It("should return false,nil,nil because annotation is set with no usages", func() {
			shoot.Annotations = map[string]string{
				common.ShootUseAsSeed: "",
			}

			useAsSeed, protected, visible := IsUsedAsSeed(shoot)

			Expect(useAsSeed).To(BeFalse())
			Expect(protected).To(BeNil())
			Expect(visible).To(BeNil())
		})

		It("should return true,nil,nil because annotation is set with normal usage", func() {
			shoot.Annotations = map[string]string{
				common.ShootUseAsSeed: "true",
			}

			useAsSeed, protected, visible := IsUsedAsSeed(shoot)

			Expect(useAsSeed).To(BeTrue())
			Expect(protected).To(BeNil())
			Expect(visible).To(BeNil())
		})

		It("should return true,true,true because annotation is set with protected and visible usage", func() {
			shoot.Annotations = map[string]string{
				common.ShootUseAsSeed: "true,protected,visible",
			}

			useAsSeed, protected, visible := IsUsedAsSeed(shoot)

			Expect(useAsSeed).To(BeTrue())
			Expect(protected).NotTo(BeNil())
			Expect(visible).NotTo(BeNil())
			Expect(*protected).To(BeTrue())
			Expect(*visible).To(BeTrue())
		})

		It("should return true,true,true because annotation is set with unprotected and invisible usage", func() {
			shoot.Annotations = map[string]string{
				common.ShootUseAsSeed: "true,unprotected,invisible",
			}

			useAsSeed, protected, visible := IsUsedAsSeed(shoot)

			Expect(useAsSeed).To(BeTrue())
			Expect(protected).NotTo(BeNil())
			Expect(visible).NotTo(BeNil())
			Expect(*protected).To(BeFalse())
			Expect(*visible).To(BeFalse())
		})
	})
})
