// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/selection"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/utils"
)

var _ = Describe("ShootStateList", func() {
	Describe("ExtensionResourceStateList", func() {
		fooString := "foo"
		var (
			shootState         *gardencorev1beta1.ShootState
			extensionKind      = fooString
			extensionName      = &fooString
			extensionPurpose   = &fooString
			extensionData      = &runtime.RawExtension{Raw: []byte("data")}
			extensionResources = []gardencorev1beta1.NamedResourceReference{
				{
					Name: "test",
					ResourceRef: autoscalingv1.CrossVersionObjectReference{
						Kind:       "Secret",
						Name:       "test-secret",
						APIVersion: "v1",
					},
				},
			}
		)

		BeforeEach(func() {
			shootState = &gardencorev1beta1.ShootState{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "foo",
				},
				Spec: gardencorev1beta1.ShootStateSpec{
					Extensions: []gardencorev1beta1.ExtensionResourceState{
						{
							Kind:      extensionKind,
							Name:      extensionName,
							Purpose:   extensionPurpose,
							State:     extensionData,
							Resources: extensionResources,
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
				Expect(resource.Resources).To(Equal(extensionResources))
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
				Expect(list).To(BeEmpty())
			})

			It("should do nothing if extension resource state cannot be found", func() {
				list := ExtensionResourceStateList(shootState.Spec.Extensions)
				barString := "bar"
				list.Delete(barString, &barString, &barString)
				Expect(list).To(HaveLen(1))
			})
		})

		Context("#Upsert", func() {
			It("should append new extension resource state to the list", func() {
				list := ExtensionResourceStateList(shootState.Spec.Extensions)
				barString := "bar"
				newResource := &gardencorev1beta1.ExtensionResourceState{
					Kind:    barString,
					Name:    &barString,
					Purpose: &barString,
					State:   &runtime.RawExtension{Raw: []byte("state")},
				}
				list.Upsert(newResource)
				Expect(list).To(HaveLen(2))
			})

			It("should update an extension resource state in the list if it already exists", func() {
				list := ExtensionResourceStateList(shootState.Spec.Extensions)
				newState := &runtime.RawExtension{Raw: []byte("new state")}
				updatedResource := &gardencorev1beta1.ExtensionResourceState{
					Kind:    extensionKind,
					Name:    extensionName,
					Purpose: extensionPurpose,
					State:   newState,
				}
				list.Upsert(updatedResource)
				Expect(list).To(HaveLen(1))
				Expect(list[0].State).To(Equal(newState))
			})
		})
	})

	Describe("GardenerResourceDataList", func() {
		var (
			shootState           *gardencorev1beta1.ShootState
			dataName             = "foo"
			dataType             = "foo"
			gardenerResourceData = runtime.RawExtension{Raw: []byte("data")}
		)

		BeforeEach(func() {
			shootState = &gardencorev1beta1.ShootState{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "foo",
				},
				Spec: gardencorev1beta1.ShootStateSpec{
					Gardener: []gardencorev1beta1.GardenerResourceData{
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
			It("should return the correct Gardener resource data", func() {
				list := GardenerResourceDataList(shootState.Spec.Gardener)
				resource := list.Get(dataName)
				Expect(resource.Name).To(Equal(dataName))
				Expect(resource.Type).To(Equal(dataType))
				Expect(resource.Data).To(Equal(gardenerResourceData))
			})

			It("should return nil if Gardener resource data cannot be found", func() {
				list := GardenerResourceDataList(shootState.Spec.Gardener)
				resource := list.Get("bar")
				Expect(resource).To(BeNil())
			})
		})

		Context("#Select", func() {
			var (
				selector = labels.NewSelector().Add(
					utils.MustNewRequirement("foo", selection.Equals, "bar"),
				)

				data1 = gardencorev1beta1.GardenerResourceData{Name: "data1", Labels: map[string]string{"bar": "foo"}}
				data2 = gardencorev1beta1.GardenerResourceData{Name: "data2", Labels: map[string]string{"foo": "bar"}}
				data3 = gardencorev1beta1.GardenerResourceData{Name: "data3", Labels: map[string]string{"foo": "baz"}}
				data4 = gardencorev1beta1.GardenerResourceData{Name: "data4", Labels: map[string]string{"foo": "bar"}}
			)

			It("should return an empty list because nothing matches", func() {
				list := GardenerResourceDataList(shootState.Spec.Gardener)

				Expect(list.Select(selector)).To(BeEmpty())
			})

			It("should return one result", func() {
				shootState.Spec.Gardener = append(shootState.Spec.Gardener, data1, data2, data3)
				list := GardenerResourceDataList(shootState.Spec.Gardener)

				Expect(list.Select(selector)).To(ConsistOf(&data2))
			})

			It("should return two results", func() {
				shootState.Spec.Gardener = append(shootState.Spec.Gardener, data1, data2, data3, data4)
				list := GardenerResourceDataList(shootState.Spec.Gardener)

				Expect(list.Select(selector)).To(ConsistOf(&data2, &data4))
			})
		})

		Context("#Delete", func() {
			It("should delete the Gardener resource data when it can be found", func() {
				list := GardenerResourceDataList(shootState.Spec.Gardener)
				list.Delete(dataName)
				Expect(list).To(BeEmpty())
			})

			It("should do nothing if Gardener resource data cannot be found", func() {
				list := GardenerResourceDataList(shootState.Spec.Gardener)
				list.Delete("bar")
				Expect(list).To(HaveLen(1))
			})
		})

		Context("#Upsert", func() {
			It("should append new Gardener resource data to the list", func() {
				list := GardenerResourceDataList(shootState.Spec.Gardener)
				newResource := &gardencorev1beta1.GardenerResourceData{
					Name: "bar",
					Type: "bar",
					Data: runtime.RawExtension{Raw: []byte("data")},
				}
				list.Upsert(newResource)
				Expect(list).To(HaveLen(2))
			})

			It("should update a Gardener resource data in the list if it already exists", func() {
				list := GardenerResourceDataList(shootState.Spec.Gardener)

				newType := "foobar"
				newData := runtime.RawExtension{Raw: []byte("new data")}
				newLabels := map[string]string{"foo": "bar"}

				updatedResource := &gardencorev1beta1.GardenerResourceData{
					Name:   dataName,
					Type:   newType,
					Data:   newData,
					Labels: newLabels,
				}
				list.Upsert(updatedResource)
				Expect(list).To(HaveLen(1))
				Expect(list[0].Data).To(Equal(newData))
				Expect(list[0].Type).To(Equal(newType))
				Expect(list[0].Labels).To(Equal(newLabels))
			})
		})

		Context("#DeepCopy", func() {
			It("should reuse the slice of shootState", func() {
				list := GardenerResourceDataList(shootState.Spec.Gardener)
				shootStateResourceName := shootState.Spec.Gardener[0].Name

				newResource := &gardencorev1beta1.GardenerResourceData{
					Name: shootStateResourceName + "bar",
					Type: "bar",
					Data: runtime.RawExtension{Raw: []byte("data")},
				}

				list.Delete(shootStateResourceName)
				Expect(list).To(BeEmpty())
				Expect(shootState.Spec.Gardener[0].Name).To(Equal(shootStateResourceName))

				list.Upsert(newResource)
				Expect(list).To(HaveLen(1))
				Expect(shootState.Spec.Gardener[0].Name).ToNot(Equal(shootStateResourceName))
				Expect(shootState.Spec.Gardener[0].Name).To(Equal(shootStateResourceName + "bar"))

			})

			It("should not reuse the slice of shootState", func() {
				list := GardenerResourceDataList(shootState.Spec.Gardener).DeepCopy()
				shootStateResourceName := shootState.Spec.Gardener[0].Name

				newResource := &gardencorev1beta1.GardenerResourceData{
					Name: shootStateResourceName + "bar",
					Type: "bar",
					Data: runtime.RawExtension{Raw: []byte("data")},
				}

				list.Delete(shootStateResourceName)
				Expect(list).To(BeEmpty())
				Expect(shootState.Spec.Gardener[0].Name).To(Equal(shootStateResourceName))

				list.Upsert(newResource)
				Expect(list).To(HaveLen(1))
				Expect(shootState.Spec.Gardener[0].Name).To(Equal(shootStateResourceName))
				Expect(shootState.Spec.Gardener[0].Name).ToNot(Equal(shootStateResourceName + "bar"))

			})
		})
	})

	Describe("ResourceDataList", func() {
		var (
			shootState *gardencorev1beta1.ShootState
			ref        = autoscalingv1.CrossVersionObjectReference{
				Kind:       "Secret",
				Name:       "test-secret",
				APIVersion: "v1",
			}
			data = runtime.RawExtension{Raw: []byte("data")}
		)

		BeforeEach(func() {
			shootState = &gardencorev1beta1.ShootState{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "foo",
				},
				Spec: gardencorev1beta1.ShootStateSpec{
					Resources: []gardencorev1beta1.ResourceData{
						{
							CrossVersionObjectReference: ref,
							Data:                        data,
						},
					},
				},
			}
		})

		Context("#Get", func() {
			It("should return the correct resource data", func() {
				list := ResourceDataList(shootState.Spec.Resources)
				resource := list.Get(&ref)
				Expect(resource.CrossVersionObjectReference).To(Equal(ref))
				Expect(resource.Data).To(Equal(data))
			})

			It("should return nil if resource data cannot be found", func() {
				list := ResourceDataList(shootState.Spec.Resources)
				resource := list.Get(&autoscalingv1.CrossVersionObjectReference{})
				Expect(resource).To(BeNil())
			})
		})

		Context("#Delete", func() {
			It("should delete the resource data when it can be found", func() {
				list := ResourceDataList(shootState.Spec.Resources)
				list.Delete(&ref)
				Expect(list).To(BeEmpty())
			})

			It("should do nothing if resource data cannot be found", func() {
				list := ResourceDataList(shootState.Spec.Resources)
				list.Delete(&autoscalingv1.CrossVersionObjectReference{})
				Expect(list).To(HaveLen(1))
			})
		})

		Context("#Upsert", func() {
			It("should append new resource data to the list", func() {
				list := ResourceDataList(shootState.Spec.Resources)
				newResource := &gardencorev1beta1.ResourceData{
					CrossVersionObjectReference: autoscalingv1.CrossVersionObjectReference{
						Kind:       "Secret",
						Name:       "test-secret2",
						APIVersion: "v1",
					},
					Data: data,
				}
				list.Upsert(newResource)
				Expect(list).To(HaveLen(2))
			})

			It("should update a resource data in the list if it already exists", func() {
				list := ResourceDataList(shootState.Spec.Resources)
				newData := runtime.RawExtension{Raw: []byte("new data")}
				updatedResource := &gardencorev1beta1.ResourceData{
					CrossVersionObjectReference: ref,
					Data:                        newData,
				}
				list.Upsert(updatedResource)
				Expect(list).To(HaveLen(1))
				Expect(list[0].Data).To(Equal(newData))
			})
		})
	})
})
