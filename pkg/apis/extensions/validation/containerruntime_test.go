// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	. "github.com/gardener/gardener/pkg/apis/extensions/validation"
)

var _ = Describe("ContainerRuntime validation tests", func() {
	var cr *extensionsv1alpha1.ContainerRuntime

	BeforeEach(func() {
		cr = &extensionsv1alpha1.ContainerRuntime{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-cr",
				Namespace: "shoot-namespace-seed",
			},
			Spec: extensionsv1alpha1.ContainerRuntimeSpec{
				DefaultSpec: extensionsv1alpha1.DefaultSpec{
					Type: "provider",
				},
				BinaryPath: "/test/path",
				WorkerPool: extensionsv1alpha1.ContainerRuntimeWorkerPool{
					Name: "test-workerPool",
				},
			},
		}
	})

	Describe("#ValidContainerRuntime", func() {
		It("should forbid empty ContainerRuntime resources", func() {
			errorList := ValidateContainerRuntime(&extensionsv1alpha1.ContainerRuntime{})

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("metadata.name"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("metadata.namespace"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("spec.type"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("spec.binaryPath"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("spec.workerPool.name"),
			}))))
		})

		It("should allow valid ContainerRuntime resources", func() {
			errorList := ValidateContainerRuntime(cr)

			Expect(errorList).To(BeEmpty())
		})
	})

	Describe("#ValidContainerRuntimeUpdate", func() {
		It("should prevent updating anything if deletion time stamp is set", func() {
			now := metav1.Now()
			cr.DeletionTimestamp = &now
			newContainerRuntime := prepareContainerRuntimeForUpdate(cr)
			newContainerRuntime.Spec.BinaryPath = "changed-binaryPath"

			errorList := ValidateContainerRuntimeUpdate(newContainerRuntime, cr)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeForbidden),
				"Field":  Equal("spec"),
				"Detail": Equal("cannot update container runtime spec if deletion timestamp is set. Requested changes: BinaryPath: changed-binaryPath != /test/path"),
			}))))
		})

		It("should prevent updating the type and workerPool ", func() {
			newContainerRuntime := prepareContainerRuntimeForUpdate(cr)
			newContainerRuntime.Spec.Type = "changed-type"
			newContainerRuntime.Spec.WorkerPool.Name = "changed-workerpool-name"

			errorList := ValidateContainerRuntimeUpdate(newContainerRuntime, cr)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.type"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.workerPool.name"),
			}))))
		})

		It("should allow updating the binaryPath", func() {
			newContainerRuntime := prepareContainerRuntimeForUpdate(cr)
			newContainerRuntime.Spec.BinaryPath = "changed-binary-path"

			errorList := ValidateContainerRuntimeUpdate(newContainerRuntime, cr)
			Expect(errorList).To(BeEmpty())
		})
	})
})

func prepareContainerRuntimeForUpdate(obj *extensionsv1alpha1.ContainerRuntime) *extensionsv1alpha1.ContainerRuntime {
	newObj := obj.DeepCopy()
	newObj.ResourceVersion = "1"
	return newObj
}
