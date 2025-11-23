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
	admissionv1 "k8s.io/api/admission/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	certificatesv1 "k8s.io/api/certificates/v1"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	logzap "sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	. "github.com/gardener/gardener/pkg/admissioncontroller/webhook/admission/shootrestriction"
	"github.com/gardener/gardener/pkg/api/indexer"
	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/logger"
)

var _ = Describe("handler", func() {
	var (
		ctx = context.TODO()
		err error

		fakeClient client.Client
		decoder    admission.Decoder

		log     logr.Logger
		handler admission.Handler
		request admission.Request
		encoder runtime.Encoder

		shootNamespace string
		shootName      string
		gardenletUser  authenticationv1.UserInfo
		gardenadmUser  authenticationv1.UserInfo

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
		fakeClient = fakeclient.
			NewClientBuilder().
			WithScheme(kubernetes.GardenScheme).
			WithIndex(&gardencorev1beta1.Shoot{}, core.ShootStatusUID, indexer.ShootStatusUIDIndexerFunc).
			Build()
		decoder = admission.NewDecoder(kubernetes.GardenScheme)
		Expect(err).NotTo(HaveOccurred())

		log = logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, logzap.WriteTo(GinkgoWriter))
		request = admission.Request{}
		encoder = &json.Serializer{}

		handler = &Handler{Logger: log, Client: fakeClient, Decoder: decoder}

		shootNamespace = "shoot-namespace"
		shootName = "shoot-name"
		gardenletUser = authenticationv1.UserInfo{
			Username: "gardener.cloud:system:shoot:" + shootNamespace + ":" + shootName,
			Groups:   []string{"gardener.cloud:system:shoots"},
		}
		gardenadmUser = authenticationv1.UserInfo{
			Username: "gardener.cloud:gardenadm:shoot:" + shootNamespace + ":" + shootName,
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

				Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
					AdmissionResponse: admissionv1.AdmissionResponse{
						Allowed: false,
						Result: &metav1.Status{
							Code:    int32(http.StatusBadRequest),
							Message: `unexpected resource: ""`,
						},
					},
				}))
			})

			It("should have no opinion because resource is irrelevant", func() {
				request.UserInfo = gardenletUser
				request.Resource = metav1.GroupVersionResource{}

				Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
					AdmissionResponse: admissionv1.AdmissionResponse{
						Allowed: false,
						Result: &metav1.Status{
							Code:    int32(http.StatusBadRequest),
							Message: `unexpected resource: ""`,
						},
					},
				}))
			})
		})

		Context("gardenlet client", func() {
			Context("when requested for CertificateSigningRequests", func() {
				var (
					name   string
					rawCSR = []byte(`-----BEGIN CERTIFICATE REQUEST-----
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
-----END CERTIFICATE REQUEST-----`)
				)

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
								Request: rawCSR,
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
								Request: rawCSR,
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
								Request: rawCSR,
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

			Context("when requested for Gardenlets", func() {
				var name string

				BeforeEach(func() {
					name = "self-hosted-shoot-foo"

					request.Name = name
					request.UserInfo = gardenletUser
					request.Resource = metav1.GroupVersionResource{
						Group:    seedmanagementv1alpha1.SchemeGroupVersion.Group,
						Version:  seedmanagementv1alpha1.SchemeGroupVersion.Version,
						Resource: "gardenlets",
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

					It("should return an error because resource name is not prefixed", func() {
						request.Name = "foo"

						Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
							AdmissionResponse: admissionv1.AdmissionResponse{
								Allowed: false,
								Result: &metav1.Status{
									Code:    int32(http.StatusBadRequest),
									Message: `the resource for self-hosted shoots must be prefixed with "self-hosted-shoot-"`,
								},
							},
						}))
					})

					It("should return an error because the requestor is not responsible for the resource", func() {
						Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
							AdmissionResponse: admissionv1.AdmissionResponse{
								Allowed: false,
								Result: &metav1.Status{
									Code:    int32(http.StatusForbidden),
									Message: "object does not belong to shoot " + shootNamespace + "/" + shootName,
								},
							},
						}))
					})

					It("should return success because the requestor is responsible for the resource", func() {
						request.Name = "self-hosted-shoot-" + shootName
						request.Namespace = shootNamespace

						Expect(handler.Handle(ctx, request)).To(Equal(responseAllowed))
					})
				})
			})

			Context("when requested for Leases", func() {
				var name string

				BeforeEach(func() {
					name = "self-hosted-shoot-foo"

					request.Name = name
					request.UserInfo = gardenletUser
					request.Resource = metav1.GroupVersionResource{
						Group:    coordinationv1.SchemeGroupVersion.Group,
						Version:  coordinationv1.SchemeGroupVersion.Version,
						Resource: "leases",
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

					It("should return an error because resource name is not prefixed", func() {
						request.Name = "foo"

						Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
							AdmissionResponse: admissionv1.AdmissionResponse{
								Allowed: false,
								Result: &metav1.Status{
									Code:    int32(http.StatusBadRequest),
									Message: `the resource for self-hosted shoots must be prefixed with "self-hosted-shoot-"`,
								},
							},
						}))
					})

					It("should return an error because the requestor is not responsible for the resource", func() {
						Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
							AdmissionResponse: admissionv1.AdmissionResponse{
								Allowed: false,
								Result: &metav1.Status{
									Code:    int32(http.StatusForbidden),
									Message: "object does not belong to shoot " + shootNamespace + "/" + shootName,
								},
							},
						}))
					})

					It("should return success because the requestor is responsible for the resource", func() {
						request.Name = "self-hosted-shoot-" + shootName
						request.Namespace = shootNamespace

						Expect(handler.Handle(ctx, request)).To(Equal(responseAllowed))
					})
				})
			})

			Context("when requested for Secrets", func() {
				var name, namespace string

				BeforeEach(func() {
					name, namespace = "foo", "bar"

					request.Name = name
					request.Namespace = namespace
					request.UserInfo = gardenletUser
					request.Resource = metav1.GroupVersionResource{
						Group:    corev1.SchemeGroupVersion.Group,
						Resource: "secrets",
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

					Context("BackupBucket secret", func() {
						BeforeEach(func() {
							request.Name = "generated-bucket-" + name
						})

						It("should return an error because the related BackupBucket was not found", func() {
							Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
								AdmissionResponse: admissionv1.AdmissionResponse{
									Allowed: false,
									Result: &metav1.Status{
										Code:    int32(http.StatusForbidden),
										Message: fmt.Sprintf("backupbuckets.core.gardener.cloud %q not found", name),
									},
								},
							}))
						})

						It("should return an error because the related Shoot could not be found", func() {
							backupBucket := &gardencorev1beta1.BackupBucket{ObjectMeta: metav1.ObjectMeta{Name: name}}
							Expect(fakeClient.Create(ctx, backupBucket)).To(Succeed())

							Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
								AdmissionResponse: admissionv1.AdmissionResponse{
									Allowed: false,
									Result: &metav1.Status{
										Code:    int32(http.StatusForbidden),
										Message: fmt.Sprintf("expected exactly one Shoot with .status.uid=%s but got 0", name),
									},
								},
							}))
						})

						It("should forbid because the related Shoot does not belong to gardenlet's shoot", func() {
							backupBucket := &gardencorev1beta1.BackupBucket{ObjectMeta: metav1.ObjectMeta{Name: name}}
							Expect(fakeClient.Create(ctx, backupBucket)).To(Succeed())

							shoot := &gardencorev1beta1.Shoot{
								ObjectMeta: metav1.ObjectMeta{Name: "some-other", Namespace: "shoot"},
								Status:     gardencorev1beta1.ShootStatus{UID: types.UID(name)},
							}
							Expect(fakeClient.Create(ctx, shoot)).To(Succeed())

							Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
								AdmissionResponse: admissionv1.AdmissionResponse{
									Allowed: false,
									Result: &metav1.Status{
										Code:    int32(http.StatusForbidden),
										Message: fmt.Sprintf("object does not belong to shoot %s/%s", shootNamespace, shootName),
									},
								},
							}))
						})

						It("should allow because the related BackupBucket does belong to gardenlet's seed", func() {
							backupBucket := &gardencorev1beta1.BackupBucket{ObjectMeta: metav1.ObjectMeta{Name: name}}
							Expect(fakeClient.Create(ctx, backupBucket)).To(Succeed())

							shoot := &gardencorev1beta1.Shoot{
								ObjectMeta: metav1.ObjectMeta{Name: shootName, Namespace: shootNamespace},
								Status:     gardencorev1beta1.ShootStatus{UID: types.UID(name)},
							}
							Expect(fakeClient.Create(ctx, shoot)).To(Succeed())

							Expect(handler.Handle(ctx, request)).To(Equal(responseAllowed))
						})
					})
				})
			})

			Context("when requested for ShootStates", func() {
				var name string

				BeforeEach(func() {
					name = "foo"

					request.Name = name
					request.UserInfo = gardenletUser
					request.Resource = metav1.GroupVersionResource{
						Group:    gardencorev1beta1.SchemeGroupVersion.Group,
						Version:  gardencorev1beta1.SchemeGroupVersion.Version,
						Resource: "shootstates",
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

					It("should return an error because the requestor is not responsible for the resource", func() {
						Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
							AdmissionResponse: admissionv1.AdmissionResponse{
								Allowed: false,
								Result: &metav1.Status{
									Code:    int32(http.StatusForbidden),
									Message: "object does not belong to shoot " + shootNamespace + "/" + shootName,
								},
							},
						}))
					})

					It("should return success because the requestor is responsible for the resource", func() {
						request.Name = shootName
						request.Namespace = shootNamespace

						Expect(handler.Handle(ctx, request)).To(Equal(responseAllowed))
					})
				})
			})
		})

		Context("gardenadm client", func() {
			Context("when requested for BackupBuckets", func() {
				var (
					name string
				)

				BeforeEach(func() {
					name = "foo"

					request.Name = name
					request.UserInfo = gardenadmUser
					request.Resource = metav1.GroupVersionResource{
						Group:    gardencorev1beta1.SchemeGroupVersion.Group,
						Version:  gardencorev1beta1.SchemeGroupVersion.Version,
						Resource: "backupbuckets",
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

					It("should deny the request because reading related Shoot object fails", func() {
						objData, err := runtime.Encode(encoder, &gardencorev1beta1.BackupBucket{
							TypeMeta: metav1.TypeMeta{
								APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
								Kind:       "BackupBucket",
							},
						})
						Expect(err).NotTo(HaveOccurred())
						request.Object.Raw = objData

						Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
							AdmissionResponse: admissionv1.AdmissionResponse{
								Allowed: false,
								Result: &metav1.Status{
									Code:    int32(http.StatusInternalServerError),
									Message: fmt.Sprintf(`failed reading Shoot resource "%s/%s" for gardenlet: shoots.core.gardener.cloud "%[2]s" not found`, shootNamespace, shootName),
								},
							},
						}))
					})

					It("should return an error because decoding the object failed", func() {
						shoot := &gardencorev1beta1.Shoot{ObjectMeta: metav1.ObjectMeta{Name: shootName, Namespace: shootNamespace}}
						Expect(fakeClient.Create(ctx, shoot)).To(Succeed())

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

					It("should deny the request because BackupBucket name does not match shoot status UID", func() {
						shoot := &gardencorev1beta1.Shoot{
							ObjectMeta: metav1.ObjectMeta{Name: shootName, Namespace: shootNamespace},
							Status:     gardencorev1beta1.ShootStatus{UID: "status-uid"},
						}
						Expect(fakeClient.Create(ctx, shoot)).To(Succeed())

						objData, err := runtime.Encode(encoder, &gardencorev1beta1.BackupBucket{
							TypeMeta: metav1.TypeMeta{
								APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
								Kind:       "BackupBucket",
							},
							ObjectMeta: metav1.ObjectMeta{
								Name: "not-the-shoot-status-uid",
							},
						})
						Expect(err).NotTo(HaveOccurred())
						request.Object.Raw = objData

						Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
							AdmissionResponse: admissionv1.AdmissionResponse{
								Allowed: false,
								Result: &metav1.Status{
									Code:    int32(http.StatusForbidden),
									Message: fmt.Sprintf("object does not belong to shoot %s/%s", shootNamespace, shootName),
								},
							},
						}))
					})

					It("should allow the request because BackupBucket name matches shoot status UID", func() {
						shoot := &gardencorev1beta1.Shoot{
							ObjectMeta: metav1.ObjectMeta{Name: shootName, Namespace: shootNamespace},
							Status:     gardencorev1beta1.ShootStatus{UID: "status-uid"},
						}
						Expect(fakeClient.Create(ctx, shoot)).To(Succeed())

						objData, err := runtime.Encode(encoder, &gardencorev1beta1.BackupBucket{
							TypeMeta: metav1.TypeMeta{
								APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
								Kind:       "BackupBucket",
							},
							ObjectMeta: metav1.ObjectMeta{
								Name: string(shoot.Status.UID),
							},
						})
						Expect(err).NotTo(HaveOccurred())
						request.Object.Raw = objData

						Expect(handler.Handle(ctx, request)).To(Equal(responseAllowed))
					})
				})
			})

			Context("when requested for BackupEntries", func() {
				var (
					name string
				)

				BeforeEach(func() {
					name = "foo"

					request.Name = name
					request.UserInfo = gardenadmUser
					request.Resource = metav1.GroupVersionResource{
						Group:    gardencorev1beta1.SchemeGroupVersion.Group,
						Version:  gardencorev1beta1.SchemeGroupVersion.Version,
						Resource: "backupentries",
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

					It("should deny the request because reading related Shoot object fails", func() {
						objData, err := runtime.Encode(encoder, &gardencorev1beta1.BackupEntry{
							TypeMeta: metav1.TypeMeta{
								APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
								Kind:       "BackupEntry",
							},
						})
						Expect(err).NotTo(HaveOccurred())
						request.Object.Raw = objData

						Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
							AdmissionResponse: admissionv1.AdmissionResponse{
								Allowed: false,
								Result: &metav1.Status{
									Code:    int32(http.StatusInternalServerError),
									Message: fmt.Sprintf(`failed reading Shoot resource "%s/%s" for gardenlet: shoots.core.gardener.cloud "%[2]s" not found`, shootNamespace, shootName),
								},
							},
						}))
					})

					It("should return an error because computing the expected BackupEntry name failed", func() {
						shoot := &gardencorev1beta1.Shoot{ObjectMeta: metav1.ObjectMeta{Name: shootName, Namespace: shootNamespace}}
						Expect(fakeClient.Create(ctx, shoot)).To(Succeed())

						Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
							AdmissionResponse: admissionv1.AdmissionResponse{
								Allowed: false,
								Result: &metav1.Status{
									Code:    int32(http.StatusInternalServerError),
									Message: "failed computing expected BackupEntry name for shoot: can't generate backup entry name with an empty shoot UID",
								},
							},
						}))
					})

					It("should return an error because decoding the object failed", func() {
						shoot := &gardencorev1beta1.Shoot{ObjectMeta: metav1.ObjectMeta{Name: shootName, Namespace: shootNamespace, UID: types.UID("1234")}}
						Expect(fakeClient.Create(ctx, shoot)).To(Succeed())

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

					It("should deny the request because BackupEntry name does not match expected name", func() {
						shoot := &gardencorev1beta1.Shoot{
							ObjectMeta: metav1.ObjectMeta{Name: shootName, Namespace: shootNamespace},
							Status:     gardencorev1beta1.ShootStatus{UID: "status-uid"},
						}
						Expect(fakeClient.Create(ctx, shoot)).To(Succeed())

						objData, err := runtime.Encode(encoder, &gardencorev1beta1.BackupEntry{
							TypeMeta: metav1.TypeMeta{
								APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
								Kind:       "BackupEntry",
							},
							ObjectMeta: metav1.ObjectMeta{
								Name:      "kube-system--not-the-shoot-status-uid",
								Namespace: shootNamespace,
							},
						})
						Expect(err).NotTo(HaveOccurred())
						request.Object.Raw = objData

						Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
							AdmissionResponse: admissionv1.AdmissionResponse{
								Allowed: false,
								Result: &metav1.Status{
									Code:    int32(http.StatusForbidden),
									Message: fmt.Sprintf("object does not belong to shoot %s/%s", shootNamespace, shootName),
								},
							},
						}))
					})

					It("should deny the request because BackupEntry name does not match expected namespace", func() {
						shoot := &gardencorev1beta1.Shoot{
							ObjectMeta: metav1.ObjectMeta{Name: shootName, Namespace: shootNamespace},
							Status:     gardencorev1beta1.ShootStatus{UID: "status-uid"},
						}
						Expect(fakeClient.Create(ctx, shoot)).To(Succeed())

						objData, err := runtime.Encode(encoder, &gardencorev1beta1.BackupEntry{
							TypeMeta: metav1.TypeMeta{
								APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
								Kind:       "BackupEntry",
							},
							ObjectMeta: metav1.ObjectMeta{
								Name:      "kube-system--status-uid",
								Namespace: "other-namespace",
							},
						})
						Expect(err).NotTo(HaveOccurred())
						request.Object.Raw = objData

						Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
							AdmissionResponse: admissionv1.AdmissionResponse{
								Allowed: false,
								Result: &metav1.Status{
									Code:    int32(http.StatusForbidden),
									Message: fmt.Sprintf("object does not belong to shoot %s/%s", shootNamespace, shootName),
								},
							},
						}))
					})

					It("should allow the request because BackupEntry name matches shoot status UID", func() {
						shoot := &gardencorev1beta1.Shoot{
							ObjectMeta: metav1.ObjectMeta{Name: shootName, Namespace: shootNamespace},
							Status:     gardencorev1beta1.ShootStatus{UID: "status-uid"},
						}
						Expect(fakeClient.Create(ctx, shoot)).To(Succeed())

						objData, err := runtime.Encode(encoder, &gardencorev1beta1.BackupEntry{
							TypeMeta: metav1.TypeMeta{
								APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
								Kind:       "BackupEntry",
							},
							ObjectMeta: metav1.ObjectMeta{
								Name:      "kube-system--status-uid",
								Namespace: shootNamespace,
							},
						})
						Expect(err).NotTo(HaveOccurred())
						request.Object.Raw = objData

						Expect(handler.Handle(ctx, request)).To(Equal(responseAllowed))
					})
				})
			})

			Context("when requested for Projects", func() {
				var (
					name string
				)

				BeforeEach(func() {
					name = "foo"

					request.Name = name
					request.UserInfo = gardenadmUser
					request.Resource = metav1.GroupVersionResource{
						Group:    gardencorev1beta1.SchemeGroupVersion.Group,
						Version:  gardencorev1beta1.SchemeGroupVersion.Version,
						Resource: "projects",
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

					It("should deny the request because project namespace does not match shoot namespaces", func() {
						objData, err := runtime.Encode(encoder, &gardencorev1beta1.Project{
							TypeMeta: metav1.TypeMeta{
								APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
								Kind:       "Project",
							},
							Spec: gardencorev1beta1.ProjectSpec{
								Namespace: ptr.To("other-namespace"),
							},
						})
						Expect(err).NotTo(HaveOccurred())
						request.Object.Raw = objData

						Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
							AdmissionResponse: admissionv1.AdmissionResponse{
								Allowed: false,
								Result: &metav1.Status{
									Code:    int32(http.StatusForbidden),
									Message: fmt.Sprintf("object does not belong to shoot %s/%s", shootNamespace, shootName),
								},
							},
						}))
					})

					It("should allow the request because project namespace matches shoot namespaces", func() {
						objData, err := runtime.Encode(encoder, &gardencorev1beta1.Project{
							TypeMeta: metav1.TypeMeta{
								APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
								Kind:       "Project",
							},
							Spec: gardencorev1beta1.ProjectSpec{
								Namespace: &shootNamespace,
							},
						})
						Expect(err).NotTo(HaveOccurred())
						request.Object.Raw = objData

						Expect(handler.Handle(ctx, request)).To(Equal(responseAllowed))
					})
				})
			})

			Context("when requested for Shoots", func() {
				var (
					name string
				)

				BeforeEach(func() {
					name = "foo"

					request.Name = name
					request.UserInfo = gardenadmUser
					request.Resource = metav1.GroupVersionResource{
						Group:    gardencorev1beta1.SchemeGroupVersion.Group,
						Version:  gardencorev1beta1.SchemeGroupVersion.Version,
						Resource: "shoots",
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

					It("should deny the request because shoot does not match shoot info", func() {
						objData, err := runtime.Encode(encoder, &gardencorev1beta1.Shoot{
							TypeMeta: metav1.TypeMeta{
								APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
								Kind:       "Shoot",
							},
							ObjectMeta: metav1.ObjectMeta{
								Namespace: "other-namespace",
								Name:      "other-name",
							},
						})
						Expect(err).NotTo(HaveOccurred())
						request.Object.Raw = objData

						Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
							AdmissionResponse: admissionv1.AdmissionResponse{
								Allowed: false,
								Result: &metav1.Status{
									Code:    int32(http.StatusForbidden),
									Message: fmt.Sprintf("object does not belong to shoot %s/%s", shootNamespace, shootName),
								},
							},
						}))
					})

					It("should allow the request because shoot namespace matches shoot namespaces", func() {
						objData, err := runtime.Encode(encoder, &gardencorev1beta1.Shoot{
							TypeMeta: metav1.TypeMeta{
								APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
								Kind:       "Shoot",
							},
							ObjectMeta: metav1.ObjectMeta{
								Namespace: shootNamespace,
								Name:      shootName,
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
