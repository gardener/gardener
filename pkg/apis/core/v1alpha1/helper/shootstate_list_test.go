// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	. "github.com/gardener/gardener/pkg/apis/core/v1alpha1/helper"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var _ = Describe("ShootStateList", func() {

	Describe("ExtensionResourceStateList", func() {
		fooString := "foo"
		var (
			shootState       *gardencorev1alpha1.ShootState
			extensionKind    = fooString
			extensionName    = &fooString
			extensionPurpose = &fooString
			extensionData    = runtime.RawExtension{Raw: []byte("data")}
		)

		BeforeEach(func() {
			shootState = &gardencorev1alpha1.ShootState{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "foo",
				},
				Spec: gardencorev1alpha1.ShootStateSpec{
					Extensions: []gardencorev1alpha1.ExtensionResourceState{
						{
							Kind:    extensionKind,
							Name:    extensionName,
							Purpose: extensionPurpose,
							State:   extensionData,
						},
					},
				},
			}
		})

		Context("#Get", func() {
			It("should return the correct extension resource state", func() {
				list := ExtensionResourceStateList(shootState.Spec.Extensions)
				resource := list.Get(extensionKind, extensionName, extensionPurpose)
				Expect(resource.Kind).To(Equal(extensionKind))
				Expect(resource.Name).To(Equal(extensionName))
				Expect(resource.Purpose).To(Equal(extensionPurpose))
				Expect(resource.State).To(Equal(extensionData))
			})

			It("should return nil if extension resource state cannot be found", func() {
				list := ExtensionResourceStateList(shootState.Spec.Extensions)
				barString := "bar"
				resource := list.Get(barString, &barString, &barString)
				Expect(resource).To(BeNil())
			})
		})

		Context("#Delete", func() {
			It("should delete the extension resource state when it can be found", func() {
				list := ExtensionResourceStateList(shootState.Spec.Extensions)
				list.Delete(extensionKind, extensionName, extensionPurpose)
				Expect(len(list)).To(Equal(0))
			})

			It("should do nothing if extension resource state cannot be found", func() {
				list := ExtensionResourceStateList(shootState.Spec.Extensions)
				barString := "bar"
				list.Delete(barString, &barString, &barString)
				Expect(len(list)).To(Equal(1))
			})
		})

		Context("#Upsert", func() {
			It("should append new extension resource state to the list", func() {
				list := ExtensionResourceStateList(shootState.Spec.Extensions)
				barString := "bar"
				newResource := &gardencorev1alpha1.ExtensionResourceState{
					Kind:    barString,
					Name:    &barString,
					Purpose: &barString,
					State:   runtime.RawExtension{Raw: []byte("state")},
				}
				list.Upsert(newResource)
				Expect(len(list)).To(Equal(2))
			})

			It("should update an extension resource state in the list if it already exists", func() {
				list := ExtensionResourceStateList(shootState.Spec.Extensions)
				newState := runtime.RawExtension{Raw: []byte("new state")}
				updatedResource := &gardencorev1alpha1.ExtensionResourceState{
					Kind:    extensionKind,
					Name:    extensionName,
					Purpose: extensionPurpose,
					State:   newState,
				}
				list.Upsert(updatedResource)
				Expect(len(list)).To(Equal(1))
				Expect(list[0].State).To(Equal(newState))
			})
		})
	})

	Describe("GardenerResourceDataList", func() {
		var (
			shootState           *gardencorev1alpha1.ShootState
			dataName             = "foo"
			dataType             = "foo"
			gardenerResourceData = runtime.RawExtension{Raw: []byte("data")}
		)

		BeforeEach(func() {
			shootState = &gardencorev1alpha1.ShootState{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "foo",
				},
				Spec: gardencorev1alpha1.ShootStateSpec{
					Gardener: []gardencorev1alpha1.GardenerResourceData{
						{
							Name: dataName,
							Type: dataType,
							Data: gardenerResourceData,
						},
					},
				},
			}
		})

		Context("#Get", func() {
			It("should return the correct extension resource state", func() {
				list := GardenerResourceDataList(shootState.Spec.Gardener)
				resource := list.Get(dataName)
				Expect(resource.Name).To(Equal(dataName))
				Expect(resource.Type).To(Equal(dataType))
				Expect(resource.Data).To(Equal(gardenerResourceData))
			})

			It("should return nil if extension resource state cannot be found", func() {
				list := GardenerResourceDataList(shootState.Spec.Gardener)
				resource := list.Get("bar")
				Expect(resource).To(BeNil())
			})
		})

		Context("#Delete", func() {
			It("should delete the extension resource state when it can be found", func() {
				list := GardenerResourceDataList(shootState.Spec.Gardener)
				list.Delete(dataName)
				Expect(len(list)).To(Equal(0))
			})

			It("should do nothing if extension resource state cannot be found", func() {
				list := GardenerResourceDataList(shootState.Spec.Gardener)
				list.Delete("bar")
				Expect(len(list)).To(Equal(1))
			})
		})

		Context("#Upsert", func() {
			It("should append new extension resource state to the list", func() {
				list := GardenerResourceDataList(shootState.Spec.Gardener)
				newResource := &gardencorev1alpha1.GardenerResourceData{
					Name: "bar",
					Type: "bar",
					Data: runtime.RawExtension{Raw: []byte("data")},
				}
				list.Upsert(newResource)
				Expect(len(list)).To(Equal(2))
			})

			It("should update an extension resource state in the list if it already exists", func() {
				list := GardenerResourceDataList(shootState.Spec.Gardener)
				newData := runtime.RawExtension{Raw: []byte("new data")}
				updatedResource := &gardencorev1alpha1.GardenerResourceData{
					Name: dataName,
					Type: dataType,
					Data: newData,
				}
				list.Upsert(updatedResource)
				Expect(len(list)).To(Equal(1))
				Expect(list[0].Data).To(Equal(newData))
			})
		})
	})

})
