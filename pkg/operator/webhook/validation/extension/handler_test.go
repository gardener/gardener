// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package extension_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	. "github.com/gardener/gardener/pkg/operator/webhook/validation/extension"
)

var _ = Describe("Handler", func() {
	var (
		ctx       context.Context
		handler   *Handler
		extension *operatorv1alpha1.Extension
	)

	BeforeEach(func() {
		ctx = context.Background()
		handler = &Handler{}
		extension = &operatorv1alpha1.Extension{}
	})

	Describe("#ValidateUpdate", func() {
		var resources []gardencorev1beta1.ControllerResource

		BeforeEach(func() {
			resources = []gardencorev1beta1.ControllerResource{
				{Kind: "Worker", Type: "test"},
			}
			extension.Spec.Resources = resources
		})

		It("should return success if the extension resources are not updated", func() {
			newExtension := extension.DeepCopy()

			warning, err := handler.ValidateUpdate(ctx, extension, newExtension)
			Expect(warning).To(BeNil())
			Expect(err).To(Succeed())
		})

		It("should return success if the extension resources are added", func() {
			newExtension := extension.DeepCopy()
			newExtension.Spec.Resources = append(newExtension.Spec.Resources, gardencorev1beta1.ControllerResource{Kind: "NewResource", Type: "test", Primary: ptr.To(false)})

			warning, err := handler.ValidateUpdate(ctx, extension, newExtension)
			Expect(warning).To(BeNil())
			Expect(err).To(Succeed())
		})

		It("should prevent changing the primary field", func() {
			extension.Spec.Resources[0].Primary = ptr.To(true)
			newExtension := extension.DeepCopy()
			newExtension.Spec.Resources[0].Primary = ptr.To(false)

			warning, err := handler.ValidateUpdate(ctx, extension, newExtension)
			Expect(warning).To(BeNil())
			Expect(err).To(MatchError(ContainSubstring("field is immutable")))
		})
	})

	Describe("#ValidateDelete", func() {
		It("should return success if there are no conditions", func() {
			warning, err := handler.ValidateDelete(ctx, extension)
			Expect(warning).To(BeNil())
			Expect(err).To(Succeed())
		})

		It("should return success if there are no required conditions", func() {
			extension.Status.Conditions = []gardencorev1beta1.Condition{
				{Type: "foo", Status: gardencorev1beta1.ConditionTrue},
			}

			warning, err := handler.ValidateDelete(ctx, extension)
			Expect(warning).To(BeNil())
			Expect(err).To(Succeed())
		})

		It("should return success if there are no required conditions set to True", func() {
			extension.Status.Conditions = []gardencorev1beta1.Condition{
				{Type: "RequiredRuntime", Status: gardencorev1beta1.ConditionFalse},
				{Type: "RequiredVirtual", Status: gardencorev1beta1.ConditionFalse},
			}

			warning, err := handler.ValidateDelete(ctx, extension)
			Expect(warning).To(BeNil())
			Expect(err).To(Succeed())
		})

		It("should prevent deletion if required runtime is set to True", func() {
			extension.Status.Conditions = []gardencorev1beta1.Condition{
				{Type: "RequiredRuntime", Status: gardencorev1beta1.ConditionTrue, Message: "required for testing"},
				{Type: "RequiredVirtual", Status: gardencorev1beta1.ConditionFalse},
			}

			warning, err := handler.ValidateDelete(ctx, extension)
			Expect(warning).To(BeNil())
			Expect(err).To(MatchError(`extension is still being required: "required for testing"`))
		})

		It("should prevent deletion if required virtual is set to True", func() {
			extension.Status.Conditions = []gardencorev1beta1.Condition{
				{Type: "RequiredRuntime", Status: gardencorev1beta1.ConditionFalse},
				{Type: "RequiredVirtual", Status: gardencorev1beta1.ConditionTrue, Message: "required for testing"},
			}

			warning, err := handler.ValidateDelete(ctx, extension)
			Expect(warning).To(BeNil())
			Expect(err).To(MatchError(`extension is still being required: "required for testing"`))
		})
	})
})
