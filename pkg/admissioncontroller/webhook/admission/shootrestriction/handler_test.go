// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shootrestriction_test

import (
	"context"
	"fmt"
	"net/http"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	admissionv1 "k8s.io/api/admission/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	certificatesv1 "k8s.io/api/certificates/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	logzap "sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	. "github.com/gardener/gardener/pkg/admissioncontroller/webhook/admission/shootrestriction"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/logger"
	mockcache "github.com/gardener/gardener/third_party/mock/controller-runtime/cache"
)

var _ = Describe("handler", func() {
	var (
		ctx = context.TODO()
		err error

		ctrl      *gomock.Controller
		mockCache *mockcache.MockCache
		decoder   admission.Decoder

		log     logr.Logger
		handler admission.Handler
		request admission.Request
		encoder runtime.Encoder

		shootNamespace string
		shootName      string
		gardenletUser  authenticationv1.UserInfo

		responseAllowed = admission.Response{
			AdmissionResponse: admissionv1.AdmissionResponse{
				Allowed: true,
				Result: &metav1.Status{
					Code: int32(http.StatusOK),
				},
			},
		}
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockCache = mockcache.NewMockCache(ctrl)
		decoder = admission.NewDecoder(kubernetes.GardenScheme)
		Expect(err).NotTo(HaveOccurred())

		log = logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, logzap.WriteTo(GinkgoWriter))
		request = admission.Request{}
		encoder = &json.Serializer{}

		handler = &Handler{Logger: log, Client: mockCache, Decoder: decoder}

		shootNamespace = "shoot-namespace"
		shootName = "shoot-name"
		gardenletUser = authenticationv1.UserInfo{
			Username: "gardener.cloud:system:shoot:" + shootNamespace + ":" + shootName,
			Groups:   []string{"gardener.cloud:system:shoots"},
		}
	})

	Describe("#Handle", func() {
		Context("when resource is unhandled", func() {
			It("should have no opinion because no shoot", func() {
				request.UserInfo = authenticationv1.UserInfo{Username: "foo"}

				Expect(handler.Handle(ctx, request)).To(Equal(responseAllowed))
			})

			It("should have no opinion because no resource request", func() {
				request.UserInfo = gardenletUser

				Expect(handler.Handle(ctx, request)).To(Equal(responseAllowed))
			})

			It("should have no opinion because resource is irrelevant", func() {
				request.UserInfo = gardenletUser
				request.Resource = metav1.GroupVersionResource{}

				Expect(handler.Handle(ctx, request)).To(Equal(responseAllowed))
			})
		})

		Context("gardenlet client", func() {
			Context("when requested for CertificateSigningRequests", func() {
				var name string

				BeforeEach(func() {
					name = "foo"

					request.Name = name
					request.UserInfo = gardenletUser
					request.Resource = metav1.GroupVersionResource{
						Group:    certificatesv1.SchemeGroupVersion.Group,
						Version:  "v1",
						Resource: "certificatesigningrequests",
					}
				})

				DescribeTable("should not allow the request because no allowed verb",
					func(operation admissionv1.Operation) {
						request.Operation = operation

						Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
							AdmissionResponse: admissionv1.AdmissionResponse{
								Allowed: false,
								Result: &metav1.Status{
									Code:    int32(http.StatusBadRequest),
									Message: fmt.Sprintf("unexpected operation: %q", operation),
								},
							},
						}))
					},

					Entry("update", admissionv1.Update),
					Entry("delete", admissionv1.Delete),
				)

				Context("when operation is create", func() {
					BeforeEach(func() {
						request.Operation = admissionv1.Create
					})

					It("should return an error because decoding the object failed", func() {
						request.Object.Raw = []byte(`{]`)

						Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
							AdmissionResponse: admissionv1.AdmissionResponse{
								Allowed: false,
								Result: &metav1.Status{
									Code:    int32(http.StatusBadRequest),
									Message: "couldn't get version/kind; json parse error: invalid character ']' looking for beginning of object key string",
								},
							},
						}))
					})

					It("should forbid the request because the CSR is not a valid shoot-related CSR", func() {
						objData, err := runtime.Encode(encoder, &certificatesv1.CertificateSigningRequest{
							TypeMeta: metav1.TypeMeta{
								APIVersion: certificatesv1.SchemeGroupVersion.String(),
								Kind:       "CertificateSigningRequest",
							},
							Spec: certificatesv1.CertificateSigningRequestSpec{
								Request: []byte(`-----BEGIN CERTIFICATE REQUEST-----
MIICrTCCAZUCAQAwaDElMCMGA1UECgwcZ2FyZGVuZXIuY2xvdWQ6c3lzdGVtOnNo
b290czE/MD0GA1UEAww2Z2FyZGVuZXIuY2xvdWQ6c3lzdGVtOnNob290OnNob290
LW5hbWVzcGFjZTpzaG9vdC1uYW1lMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIB
CgKCAQEA4pgVu/dZ3SFK8myE1ywscgaAuA4WRPDCegjIyrCK6ZCXc/srdzkFkcck
pAkebs5q4XfO8/ELQfpsUU0kIrZG+AgzuBKLq2DwIK/0Xb8xtyExb+supVum0ugA
1h2yJVK0QdzgSoEIBTezvnIqy1p3zNgOOaPlIUBWzCiGoQIQOb2PWDkrv/IQL4I4
Pt1pwVolNqNH7iExpCLCAqHYQYnNYjHdX3lw+cS72Vx8YwE2ex7v89o0O8yoSk6/
w/t/GNRtfdXlCipI5XP+iH3kGVQa3485eu/MP7Zj1goYJQclHNBvDcWk5BcIIA7B
dZQgw3VRmapOlsuHjQHTa+MIccdRQQIDAQABoAAwDQYJKoZIhvcNAQELBQADggEB
AL8QqH9x4D3Hi8EkQ+bL7U81o766T1oKWksnMeJk7jyilrWKRotBLJzijzRTe6Br
wst2faOXTCqsSgHu31z2MU3bCS0pYA8SrFLCp2uEP3oQgDFmVv6Gm9MViK6cHIe/
zNvBwqnrpCkOtjQnjDga4MxZZo2d/Ada11/arIR9Two0/EFJr0pYI0RnQ+SBdTEQ
PxC38H4SLeAx0x4CV/lKVT/7a2siOIcW1LTtjRVaCFbplTeUqFYm9uA4quYObb4d
Foj/rmOanFj5g6QF3GRDrqaNc1GNEXDU6fW7JsTx6+Anj1M/aDNxOXYqIqUN0s3d
2MyLm9v3qQ4mbHB8XgV2Nrg=
-----END CERTIFICATE REQUEST-----`),
							},
						})
						Expect(err).NotTo(HaveOccurred())
						request.Object.Raw = objData

						Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
							AdmissionResponse: admissionv1.AdmissionResponse{
								Allowed: false,
								Result: &metav1.Status{
									Code:    int32(http.StatusForbidden),
									Message: "can only create CSRs for shoot clusters: key usages are not set to [key encipherment digital signature client auth]",
								},
							},
						}))
					})

					It("should forbid the request because the shoot info of the csr does not match", func() {
						objData, err := runtime.Encode(encoder, &certificatesv1.CertificateSigningRequest{
							TypeMeta: metav1.TypeMeta{
								APIVersion: certificatesv1.SchemeGroupVersion.String(),
								Kind:       "CertificateSigningRequest",
							},
							Spec: certificatesv1.CertificateSigningRequestSpec{
								Request: []byte(`-----BEGIN CERTIFICATE REQUEST-----
MIICrTCCAZUCAQAwaDElMCMGA1UECgwcZ2FyZGVuZXIuY2xvdWQ6c3lzdGVtOnNo
b290czE/MD0GA1UEAww2Z2FyZGVuZXIuY2xvdWQ6c3lzdGVtOnNob290OnNob290
LW5hbWVzcGFjZTpzaG9vdC1uYW1lMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIB
CgKCAQEA4pgVu/dZ3SFK8myE1ywscgaAuA4WRPDCegjIyrCK6ZCXc/srdzkFkcck
pAkebs5q4XfO8/ELQfpsUU0kIrZG+AgzuBKLq2DwIK/0Xb8xtyExb+supVum0ugA
1h2yJVK0QdzgSoEIBTezvnIqy1p3zNgOOaPlIUBWzCiGoQIQOb2PWDkrv/IQL4I4
Pt1pwVolNqNH7iExpCLCAqHYQYnNYjHdX3lw+cS72Vx8YwE2ex7v89o0O8yoSk6/
w/t/GNRtfdXlCipI5XP+iH3kGVQa3485eu/MP7Zj1goYJQclHNBvDcWk5BcIIA7B
dZQgw3VRmapOlsuHjQHTa+MIccdRQQIDAQABoAAwDQYJKoZIhvcNAQELBQADggEB
AL8QqH9x4D3Hi8EkQ+bL7U81o766T1oKWksnMeJk7jyilrWKRotBLJzijzRTe6Br
wst2faOXTCqsSgHu31z2MU3bCS0pYA8SrFLCp2uEP3oQgDFmVv6Gm9MViK6cHIe/
zNvBwqnrpCkOtjQnjDga4MxZZo2d/Ada11/arIR9Two0/EFJr0pYI0RnQ+SBdTEQ
PxC38H4SLeAx0x4CV/lKVT/7a2siOIcW1LTtjRVaCFbplTeUqFYm9uA4quYObb4d
Foj/rmOanFj5g6QF3GRDrqaNc1GNEXDU6fW7JsTx6+Anj1M/aDNxOXYqIqUN0s3d
2MyLm9v3qQ4mbHB8XgV2Nrg=
-----END CERTIFICATE REQUEST-----`),
								Usages: []certificatesv1.KeyUsage{
									certificatesv1.UsageKeyEncipherment,
									certificatesv1.UsageDigitalSignature,
									certificatesv1.UsageClientAuth,
								},
							},
						})
						Expect(err).NotTo(HaveOccurred())
						request.Object.Raw = objData

						request.UserInfo = authenticationv1.UserInfo{
							Username: "gardener.cloud:system:shoot:foo:bar",
							Groups:   []string{"gardener.cloud:system:shoots"},
						}

						Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
							AdmissionResponse: admissionv1.AdmissionResponse{
								Allowed: false,
								Result: &metav1.Status{
									Code:    int32(http.StatusForbidden),
									Message: "object does not belong to shoot foo/bar",
								},
							},
						}))
					})

					It("should allow the request because shoot info matches", func() {
						objData, err := runtime.Encode(encoder, &certificatesv1.CertificateSigningRequest{
							TypeMeta: metav1.TypeMeta{
								APIVersion: certificatesv1.SchemeGroupVersion.String(),
								Kind:       "CertificateSigningRequest",
							},
							Spec: certificatesv1.CertificateSigningRequestSpec{
								Request: []byte(`-----BEGIN CERTIFICATE REQUEST-----
MIICrTCCAZUCAQAwaDElMCMGA1UECgwcZ2FyZGVuZXIuY2xvdWQ6c3lzdGVtOnNo
b290czE/MD0GA1UEAww2Z2FyZGVuZXIuY2xvdWQ6c3lzdGVtOnNob290OnNob290
LW5hbWVzcGFjZTpzaG9vdC1uYW1lMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIB
CgKCAQEA4pgVu/dZ3SFK8myE1ywscgaAuA4WRPDCegjIyrCK6ZCXc/srdzkFkcck
pAkebs5q4XfO8/ELQfpsUU0kIrZG+AgzuBKLq2DwIK/0Xb8xtyExb+supVum0ugA
1h2yJVK0QdzgSoEIBTezvnIqy1p3zNgOOaPlIUBWzCiGoQIQOb2PWDkrv/IQL4I4
Pt1pwVolNqNH7iExpCLCAqHYQYnNYjHdX3lw+cS72Vx8YwE2ex7v89o0O8yoSk6/
w/t/GNRtfdXlCipI5XP+iH3kGVQa3485eu/MP7Zj1goYJQclHNBvDcWk5BcIIA7B
dZQgw3VRmapOlsuHjQHTa+MIccdRQQIDAQABoAAwDQYJKoZIhvcNAQELBQADggEB
AL8QqH9x4D3Hi8EkQ+bL7U81o766T1oKWksnMeJk7jyilrWKRotBLJzijzRTe6Br
wst2faOXTCqsSgHu31z2MU3bCS0pYA8SrFLCp2uEP3oQgDFmVv6Gm9MViK6cHIe/
zNvBwqnrpCkOtjQnjDga4MxZZo2d/Ada11/arIR9Two0/EFJr0pYI0RnQ+SBdTEQ
PxC38H4SLeAx0x4CV/lKVT/7a2siOIcW1LTtjRVaCFbplTeUqFYm9uA4quYObb4d
Foj/rmOanFj5g6QF3GRDrqaNc1GNEXDU6fW7JsTx6+Anj1M/aDNxOXYqIqUN0s3d
2MyLm9v3qQ4mbHB8XgV2Nrg=
-----END CERTIFICATE REQUEST-----`),
								Usages: []certificatesv1.KeyUsage{
									certificatesv1.UsageKeyEncipherment,
									certificatesv1.UsageDigitalSignature,
									certificatesv1.UsageClientAuth,
								},
							},
						})
						Expect(err).NotTo(HaveOccurred())
						request.Object.Raw = objData

						Expect(handler.Handle(ctx, request)).To(Equal(responseAllowed))
					})
				})
			})
		})
	})
})
