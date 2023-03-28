// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	operatorclient "github.com/gardener/gardener/pkg/operator/client"
	. "github.com/gardener/gardener/pkg/operator/webhook/validation"
)

var _ = Describe("Handler", func() {
	var (
		ctx = context.TODO()
		log = logr.Discard()

		fakeClient client.Client
		handler    *Handler
		garden     *operatorv1alpha1.Garden
	)

	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().WithScheme(operatorclient.RuntimeScheme).Build()
		handler = &Handler{Logger: log, RuntimeClient: fakeClient}
		garden = &operatorv1alpha1.Garden{
			Spec: operatorv1alpha1.GardenSpec{
				VirtualCluster: operatorv1alpha1.VirtualCluster{
					DNS: operatorv1alpha1.DNS{
						Domain: "virtual-garden.local.gardener.cloud",
					},
					Kubernetes: operatorv1alpha1.Kubernetes{
						Version: "1.26.3",
					},
					Networking: operatorv1alpha1.Networking{
						Services: "100.64.0.0/13",
					},
				},
			},
		}
	})

	Describe("#ValidateCreate", func() {
		It("should return success if there are no errors", func() {
			Expect(handler.ValidateCreate(ctx, garden)).To(Succeed())
		})

		It("should return an error if there are validation errors", func() {
			metav1.SetMetaDataAnnotation(&garden.ObjectMeta, "gardener.cloud/operation", "rotate-credentials-complete")

			err := handler.ValidateCreate(ctx, garden)
			statusError, ok := err.(*apierrors.StatusError)
			Expect(ok).To(BeTrue())
			Expect(statusError.Status().Code).To(Equal(int32(http.StatusUnprocessableEntity)))
			Expect(statusError.Status().Reason).To(Equal(metav1.StatusReasonInvalid))
		})

		It("should return an error if there is already another Garden resource", func() {
			garden2 := garden.DeepCopy()
			garden2.SetName("garden2")
			Expect(fakeClient.Create(ctx, garden2)).To(Succeed())

			err := handler.ValidateCreate(ctx, garden)
			statusError, ok := err.(*apierrors.StatusError)
			Expect(ok).To(BeTrue())
			Expect(statusError.Status().Code).To(Equal(int32(http.StatusBadRequest)))
			Expect(statusError.Status().Message).To(ContainSubstring("there can be only one operator.gardener.cloud/v1alpha1.Garden resource in the system at a time"))
		})
	})

	Describe("#ValidateUpdate", func() {
		It("should return success if there are no errors", func() {
			Expect(handler.ValidateUpdate(ctx, garden, garden)).To(Succeed())
		})

		It("should return an error if there are validation errors", func() {
			oldGarden := garden.DeepCopy()
			metav1.SetMetaDataAnnotation(&garden.ObjectMeta, "gardener.cloud/operation", "rotate-credentials-complete")

			err := handler.ValidateUpdate(ctx, oldGarden, garden)
			statusError, ok := err.(*apierrors.StatusError)
			Expect(ok).To(BeTrue())
			Expect(statusError.Status().Code).To(Equal(int32(http.StatusUnprocessableEntity)))
			Expect(statusError.Status().Reason).To(Equal(metav1.StatusReasonInvalid))
		})

		It("should not be possible to remove the high availability setting once set", func() {
			oldGarden := garden.DeepCopy()
			oldGarden.Spec.VirtualCluster.ControlPlane = &operatorv1alpha1.ControlPlane{HighAvailability: &operatorv1alpha1.HighAvailability{}}

			err := handler.ValidateUpdate(ctx, oldGarden, garden)
			statusError, ok := err.(*apierrors.StatusError)
			Expect(ok).To(BeTrue())
			Expect(statusError.Status().Code).To(Equal(int32(http.StatusUnprocessableEntity)))
			Expect(statusError.Status().Reason).To(Equal(metav1.StatusReasonInvalid))
		})
	})

	Describe("#ValidateDelete", func() {
		It("should prevent deletion if it was not confirmed", func() {
			Expect(handler.ValidateDelete(ctx, garden)).To(MatchError(ContainSubstring(`must have a "confirmation.gardener.cloud/deletion" annotation to delete`)))
		})

		It("should allow deletion if it was confirmed", func() {
			metav1.SetMetaDataAnnotation(&garden.ObjectMeta, "confirmation.gardener.cloud/deletion", "true")

			Expect(handler.ValidateDelete(ctx, garden)).To(Succeed())
		})
	})
})
