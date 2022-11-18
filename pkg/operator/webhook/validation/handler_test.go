// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package validation_test

import (
	"context"
	"net/http"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	. "github.com/gardener/gardener/pkg/operator/webhook/validation"
)

var _ = Describe("Handler", func() {
	var (
		ctx = context.TODO()
		log = logr.Discard()

		handler *Handler
		garden  *operatorv1alpha1.Garden
	)

	BeforeEach(func() {
		handler = &Handler{Logger: log}
		garden = &operatorv1alpha1.Garden{}
	})

	Describe("#ValidateCreate", func() {
		It("should return success if there are no errors", func() {
			Expect(handler.ValidateCreate(ctx, garden)).To(Succeed())
		})

		It("should return an error if there are no errors", func() {
			metav1.SetMetaDataAnnotation(&garden.ObjectMeta, "gardener.cloud/operation", "rotate-credentials-complete")

			err := handler.ValidateCreate(ctx, garden)
			statusError, ok := err.(*apierrors.StatusError)
			Expect(ok).To(BeTrue())
			Expect(statusError.Status().Code).To(Equal(int32(http.StatusUnprocessableEntity)))
			Expect(statusError.Status().Reason).To(Equal(metav1.StatusReasonInvalid))
		})
	})

	Describe("#ValidateUpdate", func() {
		It("should return success if there are no errors", func() {
			Expect(handler.ValidateUpdate(ctx, nil, garden)).To(Succeed())
		})

		It("should return an error if there are no errors", func() {
			metav1.SetMetaDataAnnotation(&garden.ObjectMeta, "gardener.cloud/operation", "rotate-credentials-complete")

			err := handler.ValidateUpdate(ctx, nil, garden)
			statusError, ok := err.(*apierrors.StatusError)
			Expect(ok).To(BeTrue())
			Expect(statusError.Status().Code).To(Equal(int32(http.StatusUnprocessableEntity)))
			Expect(statusError.Status().Reason).To(Equal(metav1.StatusReasonInvalid))
		})
	})

	Describe("#ValidateDelete", func() {
		It("should return nil", func() {
			Expect(handler.ValidateDelete(ctx, garden)).To(BeNil())
		})
	})
})
