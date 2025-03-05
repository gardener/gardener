// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package garden_test

import (
	"context"
	"fmt"
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
	. "github.com/gardener/gardener/pkg/operator/webhook/validation/garden"
)

var _ = Describe("Handler", func() {
	var (
		ctx = context.Background()
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
				RuntimeCluster: operatorv1alpha1.RuntimeCluster{
					Ingress: operatorv1alpha1.Ingress{
						Domains: []operatorv1alpha1.DNSDomain{{Name: "ingress.bar.com"}},
					},
					Networking: operatorv1alpha1.RuntimeNetworking{
						Pods:     []string{"10.1.0.0/16"},
						Services: []string{"10.2.0.0/16"},
					},
				},
				VirtualCluster: operatorv1alpha1.VirtualCluster{
					DNS: operatorv1alpha1.DNS{
						Domains: []operatorv1alpha1.DNSDomain{{Name: "virtual-garden.local.gardener.cloud"}},
					},
					Kubernetes: operatorv1alpha1.Kubernetes{
						Version: "1.27.3",
					},
					Networking: operatorv1alpha1.Networking{
						Services: []string{"100.64.0.0/13"},
					},
				},
			},
		}
	})

	Describe("#ValidateCreate", func() {
		It("should return success if there are no errors", func() {
			warning, err := handler.ValidateCreate(ctx, garden)
			Expect(warning).To(BeNil())
			Expect(err).To(Succeed())
		})

		It("should return an error if there are validation errors", func() {
			metav1.SetMetaDataAnnotation(&garden.ObjectMeta, "gardener.cloud/operation", "rotate-credentials-complete")

			warnings, err := handler.ValidateCreate(ctx, garden)
			Expect(warnings).To(BeNil())

			statusError, ok := err.(*apierrors.StatusError)
			Expect(ok).To(BeTrue())
			Expect(statusError.Status().Code).To(Equal(int32(http.StatusUnprocessableEntity)))
			Expect(statusError.Status().Reason).To(Equal(metav1.StatusReasonInvalid))
		})

		It("should return an error if there is already another Garden resource", func() {
			garden2 := garden.DeepCopy()
			garden2.SetName("garden2")
			Expect(fakeClient.Create(ctx, garden2)).To(Succeed())

			warnings, err := handler.ValidateCreate(ctx, garden)
			Expect(warnings).To(BeNil())

			statusError, ok := err.(*apierrors.StatusError)
			Expect(ok).To(BeTrue())
			Expect(statusError.Status().Code).To(Equal(int32(http.StatusBadRequest)))
			Expect(statusError.Status().Message).To(ContainSubstring("there can be only one operator.gardener.cloud/v1alpha1.Garden resource in the system at a time"))
		})

		Context("forbidden finalizers", func() {
			test := func(finalizer string) {
				It("should return error", func() {
					garden.Finalizers = append([]string{"some-finalizer", "random"}, finalizer)

					warnings, err := handler.ValidateCreate(ctx, garden)
					Expect(warnings).To(BeNil())

					statusError, ok := err.(*apierrors.StatusError)
					Expect(ok).To(BeTrue())
					Expect(statusError.Status().Code).To(Equal(int32(http.StatusBadRequest)))
					Expect(statusError.Status().Message).To(Equal(fmt.Sprintf(`finalizer "%s" cannot be added on creation`, finalizer)))
				})
			}

			Context("reference-protection", func() {
				test("gardener.cloud/reference-protection")
			})

			Context("operator", func() {
				test("gardener.cloud/operator")
			})
		})
	})

	Describe("#ValidateUpdate", func() {
		It("should return success if there are no errors", func() {
			warnings, err := handler.ValidateUpdate(ctx, garden, garden)
			Expect(warnings).To(BeNil())
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return an error if there are validation errors", func() {
			oldGarden := garden.DeepCopy()
			metav1.SetMetaDataAnnotation(&garden.ObjectMeta, "gardener.cloud/operation", "rotate-credentials-complete")

			warnings, err := handler.ValidateUpdate(ctx, oldGarden, garden)
			Expect(warnings).To(BeNil())

			statusError, ok := err.(*apierrors.StatusError)
			Expect(ok).To(BeTrue())
			Expect(statusError.Status().Code).To(Equal(int32(http.StatusUnprocessableEntity)))
			Expect(statusError.Status().Reason).To(Equal(metav1.StatusReasonInvalid))
		})

		It("should not be possible to remove the high availability setting once set", func() {
			oldGarden := garden.DeepCopy()
			oldGarden.Spec.VirtualCluster.ControlPlane = &operatorv1alpha1.ControlPlane{HighAvailability: &operatorv1alpha1.HighAvailability{}}

			warnings, err := handler.ValidateUpdate(ctx, oldGarden, garden)
			Expect(warnings).To(BeNil())

			statusError, ok := err.(*apierrors.StatusError)
			Expect(ok).To(BeTrue())
			Expect(statusError.Status().Code).To(Equal(int32(http.StatusUnprocessableEntity)))
			Expect(statusError.Status().Reason).To(Equal(metav1.StatusReasonInvalid))
		})
	})

	Describe("#ValidateDelete", func() {
		It("should prevent deletion if it was not confirmed", func() {
			warning, err := handler.ValidateDelete(ctx, garden)
			Expect(warning).To(BeNil())
			Expect(err).To(MatchError(ContainSubstring(`must have a "confirmation.gardener.cloud/deletion" annotation to delete`)))
		})

		It("should allow deletion if it was confirmed", func() {
			metav1.SetMetaDataAnnotation(&garden.ObjectMeta, "confirmation.gardener.cloud/deletion", "true")

			warning, err := handler.ValidateDelete(ctx, garden)
			Expect(warning).To(BeNil())
			Expect(err).To(Succeed())
		})
	})
})
