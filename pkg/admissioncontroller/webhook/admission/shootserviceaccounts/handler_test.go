// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package shootserviceaccounts_test

import (
	"context"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionv1 "k8s.io/api/admission/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	logzap "sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/gardener/gardener/pkg/admissioncontroller/webhook/admission/shootserviceaccounts"
	"github.com/gardener/gardener/pkg/logger"
)

var _ = Describe("handler", func() {
	var (
		ctx     = context.TODO()
		handler admission.Handler
		request admission.Request

		responseAllowed = admission.Response{
			AdmissionResponse: admissionv1.AdmissionResponse{
				Allowed: true,
				Result:  &metav1.Status{Code: int32(http.StatusOK)},
			},
		}

		gardenletUser = authenticationv1.UserInfo{
			Username: "gardener.cloud:system:shoot:garden-myproject:myshoot",
			Groups:   []string{"gardener.cloud:system:shoots"},
		}

		extensionUser = authenticationv1.UserInfo{
			Username: "system:serviceaccount:garden-myproject:extension-shoot--myshoot--foo",
			Groups:   []string{"system:serviceaccounts", "system:serviceaccounts:garden-myproject"},
		}

		projectMemberUser = authenticationv1.UserInfo{
			Username: "alice",
			Groups:   []string{"system:authenticated"},
		}
	)

	BeforeEach(func() {
		log := logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, logzap.WriteTo(GinkgoWriter))
		handler = &shootserviceaccounts.Handler{Logger: log}
		request = admission.Request{
			AdmissionRequest: admissionv1.AdmissionRequest{
				Operation: admissionv1.Create,
				Namespace: "garden-myproject",
				Resource:  metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "serviceaccounts"},
			},
		}
	})

	Describe("#Handle", func() {
		It("should return a bad request error when the resource is not serviceaccounts", func() {
			request.Resource = metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
			request.Name = "extension-shoot--myshoot--controller"
			request.UserInfo = gardenletUser

			response := handler.Handle(ctx, request)

			Expect(response.Allowed).To(BeFalse())
			Expect(response.Result.Code).To(Equal(int32(http.StatusBadRequest)))
		})

		When("the ServiceAccount name carries the reserved prefix", func() {
			BeforeEach(func() {
				request.Name = "extension-shoot--myshoot--controller"
			})

			It("should allow when the caller is the gardenlet for this shoot", func() {
				request.UserInfo = gardenletUser

				Expect(handler.Handle(ctx, request)).To(Equal(responseAllowed))
			})

			It("should deny when the ServiceAccount name is malformed (no second --)", func() {
				request.Name = "extension-shoot--malformed"
				request.UserInfo = gardenletUser

				response := handler.Handle(ctx, request)

				Expect(response.Allowed).To(BeFalse())
				Expect(response.Result.Code).To(Equal(int32(http.StatusForbidden)))
			})

			It("should deny when the caller is a gardenlet for a different shoot", func() {
				request.UserInfo = authenticationv1.UserInfo{
					Username: "gardener.cloud:system:shoot:garden-myproject:other-shoot",
					Groups:   []string{"gardener.cloud:system:shoots"},
				}

				response := handler.Handle(ctx, request)

				Expect(response.Allowed).To(BeFalse())
				Expect(response.Result.Code).To(Equal(int32(http.StatusForbidden)))
			})

			It("should deny when the caller is a gardenlet in a different namespace", func() {
				request.UserInfo = authenticationv1.UserInfo{
					Username: "gardener.cloud:system:shoot:garden-other:myshoot",
					Groups:   []string{"gardener.cloud:system:shoots"},
				}

				response := handler.Handle(ctx, request)

				Expect(response.Allowed).To(BeFalse())
				Expect(response.Result.Code).To(Equal(int32(http.StatusForbidden)))
			})

			It("should deny when the caller is a project member", func() {
				request.UserInfo = projectMemberUser

				response := handler.Handle(ctx, request)

				Expect(response.Allowed).To(BeFalse())
				Expect(response.Result.Code).To(Equal(int32(http.StatusForbidden)))
				Expect(response.Result.Message).To(ContainSubstring("extension-shoot--"))
			})

			It("should deny when the caller is an extension service account", func() {
				request.UserInfo = extensionUser

				response := handler.Handle(ctx, request)

				Expect(response.Allowed).To(BeFalse())
				Expect(response.Result.Code).To(Equal(int32(http.StatusForbidden)))
			})
		})

		When("the token subresource is requested for a reserved-prefix ServiceAccount", func() {
			BeforeEach(func() {
				request.Name = "extension-shoot--myshoot--controller"
				request.SubResource = "token"
			})

			It("should allow when the caller is the gardenlet for this shoot", func() {
				request.UserInfo = gardenletUser

				Expect(handler.Handle(ctx, request)).To(Equal(responseAllowed))
			})

			It("should deny when the caller is a gardenlet for a different shoot", func() {
				request.UserInfo = authenticationv1.UserInfo{
					Username: "gardener.cloud:system:shoot:garden-myproject:other-shoot",
					Groups:   []string{"gardener.cloud:system:shoots"},
				}

				response := handler.Handle(ctx, request)

				Expect(response.Allowed).To(BeFalse())
				Expect(response.Result.Code).To(Equal(int32(http.StatusForbidden)))
			})

			It("should deny when the caller is a project member", func() {
				request.UserInfo = projectMemberUser

				response := handler.Handle(ctx, request)

				Expect(response.Allowed).To(BeFalse())
				Expect(response.Result.Code).To(Equal(int32(http.StatusForbidden)))
			})

			It("should deny when the caller is an extension service account", func() {
				request.UserInfo = extensionUser

				response := handler.Handle(ctx, request)

				Expect(response.Allowed).To(BeFalse())
				Expect(response.Result.Code).To(Equal(int32(http.StatusForbidden)))
			})
		})
	})
})
