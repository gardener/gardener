// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package shootrestriction_test

import (
	"context"
	stdjson "encoding/json"
	"errors"
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
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	logzap "sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	. "github.com/gardener/gardener/pkg/admissioncontroller/webhook/admission/shootrestriction"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	operationsv1alpha1 "github.com/gardener/gardener/pkg/apis/operations/v1alpha1"
	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
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

		shootNamespace          string
		shootName               string
		extensionShootNamespace string
		gardenletUser           authenticationv1.UserInfo
		gardenadmUser           authenticationv1.UserInfo
		extensionUser           authenticationv1.UserInfo

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
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).Build()
		decoder = admission.NewDecoder(kubernetes.GardenScheme)
		Expect(err).NotTo(HaveOccurred())

		log = logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, logzap.WriteTo(GinkgoWriter))
		request = admission.Request{}
		encoder = &json.Serializer{}

		handler = &Handler{Logger: log, Client: fakeClient, Decoder: decoder}

		shootNamespace = "shoot-namespace"
		shootName = "shoot-name"
		extensionShootNamespace = "garden-project"
		gardenletUser = authenticationv1.UserInfo{
			Username: "gardener.cloud:system:shoot:" + shootNamespace + ":" + shootName,
			Groups:   []string{"gardener.cloud:system:shoots"},
		}
		gardenadmUser = authenticationv1.UserInfo{
			Username: "gardener.cloud:gardenadm:shoot:" + shootNamespace + ":" + shootName,
			Groups:   []string{"gardener.cloud:system:shoots"},
		}
		extensionUser = authenticationv1.UserInfo{
			Username: "system:serviceaccount:" + extensionShootNamespace + ":extension-shoot--" + shootName + "--foo",
			Groups:   []string{"system:serviceaccounts"},
		}
	})

	Describe("#Handle", func() {
		When("resource is unhandled", func() {
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
			When("requested for BackupBuckets", func() {
				BeforeEach(func() {
					request.Name = "foo"
					request.UserInfo = gardenletUser
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

					Entry("delete", admissionv1.Delete),
				)

				When("operation is update", func() {
					BeforeEach(func() {
						request.Operation = admissionv1.Update
					})

					It("should allow when spec is unchanged", func() {
						oldBB := &gardencorev1beta1.BackupBucket{
							ObjectMeta: metav1.ObjectMeta{Name: "foo"},
							Spec:       gardencorev1beta1.BackupBucketSpec{Provider: gardencorev1beta1.BackupBucketProvider{Type: "gcp", Region: "eu-west-1"}},
						}
						newBB := oldBB.DeepCopy()
						newBB.Annotations = map[string]string{"foo": "bar"}

						oldRaw, err := stdjson.Marshal(oldBB)
						Expect(err).NotTo(HaveOccurred())
						newRaw, err := stdjson.Marshal(newBB)
						Expect(err).NotTo(HaveOccurred())
						request.OldObject = runtime.RawExtension{Raw: oldRaw}
						request.Object = runtime.RawExtension{Raw: newRaw}

						Expect(handler.Handle(ctx, request)).To(Equal(responseAllowed))
					})

					It("should deny when spec is changed", func() {
						oldBB := &gardencorev1beta1.BackupBucket{
							ObjectMeta: metav1.ObjectMeta{Name: "foo"},
							Spec:       gardencorev1beta1.BackupBucketSpec{Provider: gardencorev1beta1.BackupBucketProvider{Type: "gcp", Region: "eu-west-1"}},
						}
						newBB := oldBB.DeepCopy()
						newBB.Spec.Provider.Region = "us-east-1"

						oldRaw, err := stdjson.Marshal(oldBB)
						Expect(err).NotTo(HaveOccurred())
						newRaw, err := stdjson.Marshal(newBB)
						Expect(err).NotTo(HaveOccurred())
						request.OldObject = runtime.RawExtension{Raw: oldRaw}
						request.Object = runtime.RawExtension{Raw: newRaw}

						response := handler.Handle(ctx, request)
						Expect(response.Allowed).To(BeFalse())
						Expect(response.Result.Message).To(ContainSubstring("must not modify .spec of BackupBucket"))
					})
				})

				When("operation is create", func() {
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

					It("should deny the request because BackupBucket has no ShootRef", func() {
						objData, err := runtime.Encode(encoder, &gardencorev1beta1.BackupBucket{
							TypeMeta: metav1.TypeMeta{
								APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
								Kind:       "BackupBucket",
							},
							ObjectMeta: metav1.ObjectMeta{Name: "some-name"},
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

					It("should deny the request because BackupBucket does not reference the Shoot", func() {
						objData, err := runtime.Encode(encoder, &gardencorev1beta1.BackupBucket{
							TypeMeta: metav1.TypeMeta{
								APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
								Kind:       "BackupBucket",
							},
							ObjectMeta: metav1.ObjectMeta{Name: "some-name"},
							Spec: gardencorev1beta1.BackupBucketSpec{
								ShootRef: &corev1.ObjectReference{Name: "other-shoot", Namespace: shootNamespace},
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

					Context("after ShootRef is validated", func() {
						var (
							providerType   = "foo-provider"
							providerRegion = "foo-region"
							credentialsRef = corev1.ObjectReference{
								APIVersion: "v1",
								Kind:       "Secret",
								Namespace:  "kube-system",
								Name:       "foo-credentials",
							}

							shootWithBackup = func(region *string) *gardencorev1beta1.Shoot {
								return &gardencorev1beta1.Shoot{
									ObjectMeta: metav1.ObjectMeta{Name: shootName, Namespace: shootNamespace},
									Spec: gardencorev1beta1.ShootSpec{
										Region: providerRegion,
										Provider: gardencorev1beta1.Provider{
											Workers: []gardencorev1beta1.Worker{{
												Name: "cp-pool",
												ControlPlane: &gardencorev1beta1.WorkerControlPlane{
													Backup: &gardencorev1beta1.Backup{
														Provider:       providerType,
														Region:         region,
														CredentialsRef: &credentialsRef,
													},
												},
											}},
										},
									},
								}
							}

							validBucket = func(region string) *gardencorev1beta1.BackupBucket {
								return &gardencorev1beta1.BackupBucket{
									TypeMeta: metav1.TypeMeta{
										APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
										Kind:       "BackupBucket",
									},
									ObjectMeta: metav1.ObjectMeta{Name: "some-name"},
									Spec: gardencorev1beta1.BackupBucketSpec{
										ShootRef: &corev1.ObjectReference{Name: shootName, Namespace: shootNamespace},
										Provider: gardencorev1beta1.BackupBucketProvider{
											Type:   providerType,
											Region: region,
										},
										CredentialsRef: &credentialsRef,
									},
								}
							}

							encodeBucket = func(bb *gardencorev1beta1.BackupBucket) []byte {
								GinkgoHelper()
								objData, err := runtime.Encode(encoder, bb)
								Expect(err).NotTo(HaveOccurred())
								return objData
							}
						)

						It("should return an error because reading the Shoot failed", func() {
							fakeErr := errors.New("fake")
							fakeClient := fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).WithInterceptorFuncs(interceptor.Funcs{
								Get: func(_ context.Context, _ client.WithWatch, _ client.ObjectKey, _ client.Object, _ ...client.GetOption) error {
									return fakeErr
								},
							}).Build()
							handler = &Handler{Logger: log, Client: fakeClient, Decoder: decoder}

							request.Object.Raw = encodeBucket(validBucket(providerRegion))

							Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
								AdmissionResponse: admissionv1.AdmissionResponse{
									Allowed: false,
									Result: &metav1.Status{
										Code:    int32(http.StatusInternalServerError),
										Message: fakeErr.Error(),
									},
								},
							}))
						})

						It("should forbid the request because the shoot has no control-plane worker pool", func() {
							shoot := &gardencorev1beta1.Shoot{
								ObjectMeta: metav1.ObjectMeta{Name: shootName, Namespace: shootNamespace},
								Spec:       gardencorev1beta1.ShootSpec{Region: providerRegion},
							}
							Expect(fakeClient.Create(ctx, shoot)).To(Succeed())

							request.Object.Raw = encodeBucket(validBucket(providerRegion))

							Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
								AdmissionResponse: admissionv1.AdmissionResponse{
									Allowed: false,
									Result: &metav1.Status{
										Code:    int32(http.StatusForbidden),
										Message: "shoot has no control-plane worker pool",
									},
								},
							}))
						})

						It("should forbid the request because the control-plane worker pool has no backup configuration", func() {
							shoot := &gardencorev1beta1.Shoot{
								ObjectMeta: metav1.ObjectMeta{Name: shootName, Namespace: shootNamespace},
								Spec: gardencorev1beta1.ShootSpec{
									Region: providerRegion,
									Provider: gardencorev1beta1.Provider{
										Workers: []gardencorev1beta1.Worker{{
											Name:         "cp-pool",
											ControlPlane: &gardencorev1beta1.WorkerControlPlane{},
										}},
									},
								},
							}
							Expect(fakeClient.Create(ctx, shoot)).To(Succeed())

							request.Object.Raw = encodeBucket(validBucket(providerRegion))

							Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
								AdmissionResponse: admissionv1.AdmissionResponse{
									Allowed: false,
									Result: &metav1.Status{
										Code:    int32(http.StatusForbidden),
										Message: "shoot's control-plane worker pool has no backup configuration",
									},
								},
							}))
						})

						DescribeTable("should forbid the request because the BackupBucket spec does not match the shoot's backup configuration",
							func(mutateBucket func(*gardencorev1beta1.BackupBucket)) {
								Expect(fakeClient.Create(ctx, shootWithBackup(&providerRegion))).To(Succeed())

								bucket := validBucket(providerRegion)
								mutateBucket(bucket)
								request.Object.Raw = encodeBucket(bucket)

								Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
									AdmissionResponse: admissionv1.AdmissionResponse{
										Allowed: false,
										Result: &metav1.Status{
											Code:    int32(http.StatusForbidden),
											Message: "BackupBucket spec does not match the backup configuration of the shoot's control-plane worker pool",
										},
									},
								}))
							},

							Entry("provider type mismatch", func(b *gardencorev1beta1.BackupBucket) {
								b.Spec.Provider.Type = "other-provider"
							}),
							Entry("provider region mismatch", func(b *gardencorev1beta1.BackupBucket) {
								b.Spec.Provider.Region = "other-region"
							}),
							Entry("credentialsRef mismatch", func(b *gardencorev1beta1.BackupBucket) {
								b.Spec.CredentialsRef = &corev1.ObjectReference{Name: "other-credentials"}
							}),
						)

						It("should allow the request when spec matches and backup region is set explicitly", func() {
							Expect(fakeClient.Create(ctx, shootWithBackup(&providerRegion))).To(Succeed())

							request.Object.Raw = encodeBucket(validBucket(providerRegion))

							Expect(handler.Handle(ctx, request)).To(Equal(responseAllowed))
						})

						It("should allow the request when spec matches and backup region falls back to shoot region", func() {
							Expect(fakeClient.Create(ctx, shootWithBackup(nil))).To(Succeed())

							request.Object.Raw = encodeBucket(validBucket(providerRegion))

							Expect(handler.Handle(ctx, request)).To(Equal(responseAllowed))
						})
					})
				})
			})

			When("requested for BackupEntries", func() {
				BeforeEach(func() {
					request.Name = "foo"
					request.UserInfo = gardenletUser
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

					Entry("delete", admissionv1.Delete),
				)

				When("operation is update", func() {
					BeforeEach(func() {
						request.Operation = admissionv1.Update
					})

					It("should allow when spec is unchanged", func() {
						oldBE := &gardencorev1beta1.BackupEntry{
							ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: shootNamespace},
							Spec:       gardencorev1beta1.BackupEntrySpec{BucketName: "my-bucket"},
						}
						newBE := oldBE.DeepCopy()
						newBE.Annotations = map[string]string{"foo": "bar"}

						oldRaw, err := stdjson.Marshal(oldBE)
						Expect(err).NotTo(HaveOccurred())
						newRaw, err := stdjson.Marshal(newBE)
						Expect(err).NotTo(HaveOccurred())
						request.OldObject = runtime.RawExtension{Raw: oldRaw}
						request.Object = runtime.RawExtension{Raw: newRaw}

						Expect(handler.Handle(ctx, request)).To(Equal(responseAllowed))
					})

					It("should deny when shootRef is changed", func() {
						oldBE := &gardencorev1beta1.BackupEntry{
							ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: shootNamespace},
							Spec:       gardencorev1beta1.BackupEntrySpec{BucketName: "my-bucket"},
						}
						newBE := oldBE.DeepCopy()
						newBE.Spec.ShootRef = &corev1.ObjectReference{Name: "some-shoot", Namespace: "some-ns"}

						oldRaw, err := stdjson.Marshal(oldBE)
						Expect(err).NotTo(HaveOccurred())
						newRaw, err := stdjson.Marshal(newBE)
						Expect(err).NotTo(HaveOccurred())
						request.OldObject = runtime.RawExtension{Raw: oldRaw}
						request.Object = runtime.RawExtension{Raw: newRaw}

						response := handler.Handle(ctx, request)
						Expect(response.Allowed).To(BeFalse())
						Expect(response.Result.Message).To(ContainSubstring("must not modify .spec.shootRef of BackupEntry"))
					})

					It("should deny when bucketName is changed to a bucket not belonging to the shoot", func() {
						Expect(fakeClient.Create(ctx, &gardencorev1beta1.BackupBucket{
							ObjectMeta: metav1.ObjectMeta{Name: "other-bucket"},
							Spec: gardencorev1beta1.BackupBucketSpec{
								ShootRef: &corev1.ObjectReference{Name: "other-shoot", Namespace: shootNamespace},
							},
						})).To(Succeed())

						oldBE := &gardencorev1beta1.BackupEntry{
							ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: shootNamespace},
							Spec:       gardencorev1beta1.BackupEntrySpec{BucketName: "my-bucket"},
						}
						newBE := oldBE.DeepCopy()
						newBE.Spec.BucketName = "other-bucket"

						oldRaw, err := stdjson.Marshal(oldBE)
						Expect(err).NotTo(HaveOccurred())
						newRaw, err := stdjson.Marshal(newBE)
						Expect(err).NotTo(HaveOccurred())
						request.OldObject = runtime.RawExtension{Raw: oldRaw}
						request.Object = runtime.RawExtension{Raw: newRaw}

						response := handler.Handle(ctx, request)
						Expect(response.Allowed).To(BeFalse())
					})

					It("should allow when bucketName is changed to a bucket belonging to the shoot (migration)", func() {
						Expect(fakeClient.Create(ctx, &gardencorev1beta1.BackupBucket{
							ObjectMeta: metav1.ObjectMeta{Name: "new-bucket"},
							Spec: gardencorev1beta1.BackupBucketSpec{
								ShootRef: &corev1.ObjectReference{Name: shootName, Namespace: shootNamespace},
							},
						})).To(Succeed())

						oldBE := &gardencorev1beta1.BackupEntry{
							ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: shootNamespace},
							Spec:       gardencorev1beta1.BackupEntrySpec{BucketName: "my-bucket"},
						}
						newBE := oldBE.DeepCopy()
						newBE.Spec.BucketName = "new-bucket"

						oldRaw, err := stdjson.Marshal(oldBE)
						Expect(err).NotTo(HaveOccurred())
						newRaw, err := stdjson.Marshal(newBE)
						Expect(err).NotTo(HaveOccurred())
						request.OldObject = runtime.RawExtension{Raw: oldRaw}
						request.Object = runtime.RawExtension{Raw: newRaw}

						Expect(handler.Handle(ctx, request)).To(Equal(responseAllowed))
					})
				})

				When("operation is create", func() {
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

					It("should deny the request because BackupEntry has no ShootRef", func() {
						objData, err := runtime.Encode(encoder, &gardencorev1beta1.BackupEntry{
							TypeMeta: metav1.TypeMeta{
								APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
								Kind:       "BackupEntry",
							},
							ObjectMeta: metav1.ObjectMeta{Name: "some-name", Namespace: shootNamespace},
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

					It("should deny the request because BackupEntry does not reference the Shoot", func() {
						objData, err := runtime.Encode(encoder, &gardencorev1beta1.BackupEntry{
							TypeMeta: metav1.TypeMeta{
								APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
								Kind:       "BackupEntry",
							},
							ObjectMeta: metav1.ObjectMeta{Name: "some-name", Namespace: shootNamespace},
							Spec: gardencorev1beta1.BackupEntrySpec{
								ShootRef: &corev1.ObjectReference{Name: "other-shoot", Namespace: shootNamespace},
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

					It("should deny the request because the referenced BackupBucket was not found", func() {
						objData, err := runtime.Encode(encoder, &gardencorev1beta1.BackupEntry{
							TypeMeta: metav1.TypeMeta{
								APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
								Kind:       "BackupEntry",
							},
							ObjectMeta: metav1.ObjectMeta{Name: "some-name", Namespace: shootNamespace},
							Spec: gardencorev1beta1.BackupEntrySpec{
								BucketName: "missing-bucket",
								ShootRef:   &corev1.ObjectReference{Name: shootName, Namespace: shootNamespace},
							},
						})
						Expect(err).NotTo(HaveOccurred())
						request.Object.Raw = objData

						Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
							AdmissionResponse: admissionv1.AdmissionResponse{
								Allowed: false,
								Result: &metav1.Status{
									Code:    int32(http.StatusForbidden),
									Message: `backupbuckets.core.gardener.cloud "missing-bucket" not found`,
								},
							},
						}))
					})

					It("should deny the request because the referenced BackupBucket has no ShootRef", func() {
						backupBucket := &gardencorev1beta1.BackupBucket{
							ObjectMeta: metav1.ObjectMeta{Name: "the-bucket"},
						}
						Expect(fakeClient.Create(ctx, backupBucket)).To(Succeed())

						objData, err := runtime.Encode(encoder, &gardencorev1beta1.BackupEntry{
							TypeMeta: metav1.TypeMeta{
								APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
								Kind:       "BackupEntry",
							},
							ObjectMeta: metav1.ObjectMeta{Name: "some-name", Namespace: shootNamespace},
							Spec: gardencorev1beta1.BackupEntrySpec{
								BucketName: "the-bucket",
								ShootRef:   &corev1.ObjectReference{Name: shootName, Namespace: shootNamespace},
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

					It("should deny the request because the referenced BackupBucket does not reference the Shoot", func() {
						backupBucket := &gardencorev1beta1.BackupBucket{
							ObjectMeta: metav1.ObjectMeta{Name: "the-bucket"},
							Spec: gardencorev1beta1.BackupBucketSpec{
								ShootRef: &corev1.ObjectReference{Name: "other-shoot", Namespace: shootNamespace},
							},
						}
						Expect(fakeClient.Create(ctx, backupBucket)).To(Succeed())

						objData, err := runtime.Encode(encoder, &gardencorev1beta1.BackupEntry{
							TypeMeta: metav1.TypeMeta{
								APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
								Kind:       "BackupEntry",
							},
							ObjectMeta: metav1.ObjectMeta{Name: "some-name", Namespace: shootNamespace},
							Spec: gardencorev1beta1.BackupEntrySpec{
								BucketName: "the-bucket",
								ShootRef:   &corev1.ObjectReference{Name: shootName, Namespace: shootNamespace},
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

					It("should allow the request because both BackupEntry and referenced BackupBucket reference the Shoot", func() {
						backupBucket := &gardencorev1beta1.BackupBucket{
							ObjectMeta: metav1.ObjectMeta{Name: "the-bucket"},
							Spec: gardencorev1beta1.BackupBucketSpec{
								ShootRef: &corev1.ObjectReference{Name: shootName, Namespace: shootNamespace},
							},
						}
						Expect(fakeClient.Create(ctx, backupBucket)).To(Succeed())

						objData, err := runtime.Encode(encoder, &gardencorev1beta1.BackupEntry{
							TypeMeta: metav1.TypeMeta{
								APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
								Kind:       "BackupEntry",
							},
							ObjectMeta: metav1.ObjectMeta{Name: "some-name", Namespace: shootNamespace},
							Spec: gardencorev1beta1.BackupEntrySpec{
								BucketName: "the-bucket",
								ShootRef:   &corev1.ObjectReference{Name: shootName, Namespace: shootNamespace},
							},
						})
						Expect(err).NotTo(HaveOccurred())
						request.Object.Raw = objData

						Expect(handler.Handle(ctx, request)).To(Equal(responseAllowed))
					})
				})
			})

			When("requested for Bastions", func() {
				BeforeEach(func() {
					request.Name = "foo"
					request.Namespace = shootNamespace
					request.UserInfo = gardenletUser
					request.Resource = metav1.GroupVersionResource{
						Group:    operationsv1alpha1.SchemeGroupVersion.Group,
						Version:  operationsv1alpha1.SchemeGroupVersion.Version,
						Resource: "bastions",
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

					Entry("create", admissionv1.Create),
					Entry("delete", admissionv1.Delete),
				)

				When("operation is update", func() {
					BeforeEach(func() {
						request.Operation = admissionv1.Update
					})

					It("should allow when spec is unchanged", func() {
						oldB := &operationsv1alpha1.Bastion{
							ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: shootNamespace},
							Spec:       operationsv1alpha1.BastionSpec{SSHPublicKey: "ssh-rsa AAAA"},
						}
						newB := oldB.DeepCopy()
						newB.Annotations = map[string]string{"foo": "bar"}

						oldRaw, err := stdjson.Marshal(oldB)
						Expect(err).NotTo(HaveOccurred())
						newRaw, err := stdjson.Marshal(newB)
						Expect(err).NotTo(HaveOccurred())
						request.OldObject = runtime.RawExtension{Raw: oldRaw}
						request.Object = runtime.RawExtension{Raw: newRaw}

						Expect(handler.Handle(ctx, request)).To(Equal(responseAllowed))
					})

					It("should deny when spec is changed", func() {
						oldB := &operationsv1alpha1.Bastion{
							ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: shootNamespace},
							Spec:       operationsv1alpha1.BastionSpec{SSHPublicKey: "ssh-rsa AAAA"},
						}
						newB := oldB.DeepCopy()
						newB.Spec.SSHPublicKey = "ssh-rsa BBBB"

						oldRaw, err := stdjson.Marshal(oldB)
						Expect(err).NotTo(HaveOccurred())
						newRaw, err := stdjson.Marshal(newB)
						Expect(err).NotTo(HaveOccurred())
						request.OldObject = runtime.RawExtension{Raw: oldRaw}
						request.Object = runtime.RawExtension{Raw: newRaw}

						response := handler.Handle(ctx, request)
						Expect(response.Allowed).To(BeFalse())
						Expect(response.Result.Message).To(ContainSubstring("must not modify .spec of Bastion"))
					})
				})
			})

			When("requested for ControllerInstallations", func() {
				BeforeEach(func() {
					request.Name = "foo"
					request.UserInfo = gardenletUser
					request.Resource = metav1.GroupVersionResource{
						Group:    gardencorev1beta1.SchemeGroupVersion.Group,
						Version:  gardencorev1beta1.SchemeGroupVersion.Version,
						Resource: "controllerinstallations",
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

					Entry("create", admissionv1.Create),
					Entry("delete", admissionv1.Delete),
				)

				When("operation is update", func() {
					BeforeEach(func() {
						request.Operation = admissionv1.Update
					})

					It("should allow when spec is unchanged", func() {
						oldCI := &gardencorev1beta1.ControllerInstallation{
							ObjectMeta: metav1.ObjectMeta{Name: "foo"},
							Spec: gardencorev1beta1.ControllerInstallationSpec{
								RegistrationRef: corev1.ObjectReference{Name: "my-reg"},
							},
						}
						newCI := oldCI.DeepCopy()
						newCI.Annotations = map[string]string{"foo": "bar"}

						oldRaw, err := stdjson.Marshal(oldCI)
						Expect(err).NotTo(HaveOccurred())
						newRaw, err := stdjson.Marshal(newCI)
						Expect(err).NotTo(HaveOccurred())
						request.OldObject = runtime.RawExtension{Raw: oldRaw}
						request.Object = runtime.RawExtension{Raw: newRaw}

						Expect(handler.Handle(ctx, request)).To(Equal(responseAllowed))
					})

					It("should deny when spec is changed", func() {
						oldCI := &gardencorev1beta1.ControllerInstallation{
							ObjectMeta: metav1.ObjectMeta{Name: "foo"},
							Spec: gardencorev1beta1.ControllerInstallationSpec{
								RegistrationRef: corev1.ObjectReference{Name: "my-reg"},
							},
						}
						newCI := oldCI.DeepCopy()
						newCI.Spec.RegistrationRef.Name = "other-reg"

						oldRaw, err := stdjson.Marshal(oldCI)
						Expect(err).NotTo(HaveOccurred())
						newRaw, err := stdjson.Marshal(newCI)
						Expect(err).NotTo(HaveOccurred())
						request.OldObject = runtime.RawExtension{Raw: oldRaw}
						request.Object = runtime.RawExtension{Raw: newRaw}

						response := handler.Handle(ctx, request)
						Expect(response.Allowed).To(BeFalse())
						Expect(response.Result.Message).To(ContainSubstring("must not modify .spec of ControllerInstallation"))
					})
				})
			})

			When("requested for CertificateSigningRequests", func() {
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

				When("operation is create", func() {
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

			When("requested for ConfigMaps", func() {
				BeforeEach(func() {
					request.UserInfo = gardenletUser
					request.Resource = metav1.GroupVersionResource{
						Group:    corev1.SchemeGroupVersion.Group,
						Version:  corev1.SchemeGroupVersion.Version,
						Resource: "configmaps",
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

				When("operation is create", func() {
					BeforeEach(func() {
						request.Operation = admissionv1.Create
						request.Namespace = shootNamespace
					})

					It("should return an error because the config map name is not a shoot project config map", func() {
						request.Name = "foo"

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

					It("should return an error because the requestor is not responsible for the resource", func() {
						request.Name = "other-shoot.ca-cluster"

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

					DescribeTable("should return success because the requestor is responsible for the resource",
						func(suffix string) {
							request.Name = shootName + "." + suffix

							Expect(handler.Handle(ctx, request)).To(Equal(responseAllowed))
						},

						Entry("ca-cluster", "ca-cluster"),
						Entry("ca-kubelet", "ca-kubelet"),
					)
				})
			})

			When("requested for Gardenlets", func() {
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

					Entry("delete", admissionv1.Delete),
				)

				When("operation is update", func() {
					BeforeEach(func() {
						request.Operation = admissionv1.Update
						request.Name = shootName
						request.Namespace = shootNamespace
					})

					It("should allow when spec is unchanged", func() {
						oldGardenlet := &seedmanagementv1alpha1.Gardenlet{
							ObjectMeta: metav1.ObjectMeta{Name: shootName, Namespace: shootNamespace},
						}
						newGardenlet := oldGardenlet.DeepCopy()
						newGardenlet.Annotations = map[string]string{"foo": "bar"}

						oldRaw, err := stdjson.Marshal(oldGardenlet)
						Expect(err).NotTo(HaveOccurred())
						newRaw, err := stdjson.Marshal(newGardenlet)
						Expect(err).NotTo(HaveOccurred())
						request.OldObject = runtime.RawExtension{Raw: oldRaw}
						request.Object = runtime.RawExtension{Raw: newRaw}

						Expect(handler.Handle(ctx, request)).To(Equal(responseAllowed))
					})

					It("should deny when spec is changed", func() {
						oldGardenlet := &seedmanagementv1alpha1.Gardenlet{
							ObjectMeta: metav1.ObjectMeta{Name: shootName, Namespace: shootNamespace},
						}
						newGardenlet := oldGardenlet.DeepCopy()
						replicaCount := int32(3)
						newGardenlet.Spec.Deployment.ReplicaCount = &replicaCount

						oldRaw, err := stdjson.Marshal(oldGardenlet)
						Expect(err).NotTo(HaveOccurred())
						newRaw, err := stdjson.Marshal(newGardenlet)
						Expect(err).NotTo(HaveOccurred())
						request.OldObject = runtime.RawExtension{Raw: oldRaw}
						request.Object = runtime.RawExtension{Raw: newRaw}

						response := handler.Handle(ctx, request)
						Expect(response.Allowed).To(BeFalse())
						Expect(string(response.Result.Message)).To(ContainSubstring("must not modify .spec of Gardenlet"))
					})
				})

				When("operation is create", func() {
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

				When("operation is update", func() {
					BeforeEach(func() {
						request.Operation = admissionv1.Update
						request.Name = shootName
						request.Namespace = shootNamespace
					})

					It("should allow when spec is unchanged", func() {
						oldGardenlet := &seedmanagementv1alpha1.Gardenlet{
							ObjectMeta: metav1.ObjectMeta{Name: shootName, Namespace: shootNamespace},
						}
						newGardenlet := oldGardenlet.DeepCopy()
						newGardenlet.Annotations = map[string]string{"foo": "bar"}

						oldRaw, err := stdjson.Marshal(oldGardenlet)
						Expect(err).NotTo(HaveOccurred())
						newRaw, err := stdjson.Marshal(newGardenlet)
						Expect(err).NotTo(HaveOccurred())
						request.OldObject = runtime.RawExtension{Raw: oldRaw}
						request.Object = runtime.RawExtension{Raw: newRaw}

						Expect(handler.Handle(ctx, request)).To(Equal(responseAllowed))
					})

					It("should deny when spec is changed", func() {
						oldGardenlet := &seedmanagementv1alpha1.Gardenlet{
							ObjectMeta: metav1.ObjectMeta{Name: shootName, Namespace: shootNamespace},
						}
						newGardenlet := oldGardenlet.DeepCopy()
						replicaCount := int32(3)
						newGardenlet.Spec.Deployment.ReplicaCount = &replicaCount

						oldRaw, err := stdjson.Marshal(oldGardenlet)
						Expect(err).NotTo(HaveOccurred())
						newRaw, err := stdjson.Marshal(newGardenlet)
						Expect(err).NotTo(HaveOccurred())
						request.OldObject = runtime.RawExtension{Raw: oldRaw}
						request.Object = runtime.RawExtension{Raw: newRaw}

						response := handler.Handle(ctx, request)
						Expect(response.Allowed).To(BeFalse())
						Expect(response.Result.Message).To(ContainSubstring("must not modify .spec of Gardenlet"))
					})
				})
			})

			When("requested for InternalSecrets", func() {
				BeforeEach(func() {
					request.UserInfo = gardenletUser
					request.Resource = metav1.GroupVersionResource{
						Group:    gardencorev1beta1.SchemeGroupVersion.Group,
						Version:  gardencorev1beta1.SchemeGroupVersion.Version,
						Resource: "internalsecrets",
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

				When("operation is create", func() {
					BeforeEach(func() {
						request.Operation = admissionv1.Create
						request.Namespace = shootNamespace
					})

					It("should return an error because the internal secret name is not a shoot project internal secret", func() {
						request.Name = "foo"

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

					It("should return an error because the requestor is not responsible for the resource", func() {
						request.Name = "other-shoot.ca-client"

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

					It("should return success because the requestor is responsible for the resource", func() {
						request.Name = shootName + ".ca-client"

						Expect(handler.Handle(ctx, request)).To(Equal(responseAllowed))
					})
				})
			})

			When("requested for ManagedSeeds", func() {
				BeforeEach(func() {
					request.Name = "foo"
					request.Namespace = shootNamespace
					request.UserInfo = gardenletUser
					request.Resource = metav1.GroupVersionResource{
						Group:    seedmanagementv1alpha1.SchemeGroupVersion.Group,
						Version:  seedmanagementv1alpha1.SchemeGroupVersion.Version,
						Resource: "managedseeds",
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

					Entry("create", admissionv1.Create),
					Entry("delete", admissionv1.Delete),
				)

				When("operation is update", func() {
					BeforeEach(func() {
						request.Operation = admissionv1.Update
					})

					It("should allow when spec is unchanged and ManagedSeed belongs to gardenlet's shoot", func() {
						oldMS := &seedmanagementv1alpha1.ManagedSeed{
							ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: shootNamespace},
							Spec:       seedmanagementv1alpha1.ManagedSeedSpec{Shoot: &seedmanagementv1alpha1.Shoot{Name: shootName}},
						}
						newMS := oldMS.DeepCopy()
						newMS.Annotations = map[string]string{"foo": "bar"}

						oldRaw, err := stdjson.Marshal(oldMS)
						Expect(err).NotTo(HaveOccurred())
						newRaw, err := stdjson.Marshal(newMS)
						Expect(err).NotTo(HaveOccurred())
						request.OldObject = runtime.RawExtension{Raw: oldRaw}
						request.Object = runtime.RawExtension{Raw: newRaw}

						Expect(handler.Handle(ctx, request)).To(Equal(responseAllowed))
					})

					It("should deny when spec is changed", func() {
						oldMS := &seedmanagementv1alpha1.ManagedSeed{
							ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: shootNamespace},
							Spec:       seedmanagementv1alpha1.ManagedSeedSpec{Shoot: &seedmanagementv1alpha1.Shoot{Name: shootName}},
						}
						newMS := oldMS.DeepCopy()
						newMS.Spec.Shoot.Name = "other-shoot"

						oldRaw, err := stdjson.Marshal(oldMS)
						Expect(err).NotTo(HaveOccurred())
						newRaw, err := stdjson.Marshal(newMS)
						Expect(err).NotTo(HaveOccurred())
						request.OldObject = runtime.RawExtension{Raw: oldRaw}
						request.Object = runtime.RawExtension{Raw: newRaw}

						response := handler.Handle(ctx, request)
						Expect(response.Allowed).To(BeFalse())
						Expect(response.Result.Message).To(ContainSubstring("must not modify .spec of ManagedSeed"))
					})

					It("should forbid because the ManagedSeed does not belong to gardenlet's shoot", func() {
						oldMS := &seedmanagementv1alpha1.ManagedSeed{
							ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: shootNamespace},
							Spec:       seedmanagementv1alpha1.ManagedSeedSpec{Shoot: &seedmanagementv1alpha1.Shoot{Name: "other-shoot"}},
						}
						newMS := oldMS.DeepCopy()
						newMS.Annotations = map[string]string{"foo": "bar"}

						oldRaw, err := stdjson.Marshal(oldMS)
						Expect(err).NotTo(HaveOccurred())
						newRaw, err := stdjson.Marshal(newMS)
						Expect(err).NotTo(HaveOccurred())
						request.OldObject = runtime.RawExtension{Raw: oldRaw}
						request.Object = runtime.RawExtension{Raw: newRaw}

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
				})
			})

			When("requested for Leases", func() {
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

				When("operation is create", func() {
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

				Context("extension client", func() {
					BeforeEach(func() {
						request.UserInfo = extensionUser
						request.Operation = admissionv1.Create
						request.Namespace = extensionShootNamespace
						request.Name = shootName + "--provider-aws-leader-election"
					})

					It("should allow lease creation in shoot namespace with correct name prefix", func() {
						Expect(handler.Handle(ctx, request)).To(Equal(responseAllowed))
					})

					It("should forbid lease creation outside shoot namespace", func() {
						request.Namespace = "other-namespace"

						Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
							AdmissionResponse: admissionv1.AdmissionResponse{
								Allowed: false,
								Result: &metav1.Status{
									Code:    int32(http.StatusForbidden),
									Message: fmt.Sprintf("extension client can only create leases in the namespace for shoot \"%s/%s\"", extensionShootNamespace, shootName),
								},
							},
						}))
					})

					It("should forbid lease creation when name does not have the shoot name as prefix", func() {
						request.Name = "other-shoot--provider-aws-leader-election"

						Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
							AdmissionResponse: admissionv1.AdmissionResponse{
								Allowed: false,
								Result: &metav1.Status{
									Code:    int32(http.StatusForbidden),
									Message: fmt.Sprintf("extension client can only create leases with the shoot name %q as prefix", shootName),
								},
							},
						}))
					})
				})
			})

			When("requested for Secrets", func() {
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

				When("operation is create", func() {
					BeforeEach(func() {
						request.Operation = admissionv1.Create
					})

					Context("shoot-project secret", func() {
						BeforeEach(func() {
							request.Namespace = shootNamespace
						})

						It("should forbid because the shoot-project secret does not belong to gardenlet's shoot", func() {
							request.Name = "other-shoot.ca-cluster"

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

						DescribeTable("should allow because the shoot-project secret belongs to gardenlet's shoot",
							func(suffix string) {
								request.Name = shootName + "." + suffix

								Expect(handler.Handle(ctx, request)).To(Equal(responseAllowed))
							},

							Entry("ca-cluster", "ca-cluster"),
							Entry("ssh-keypair", "ssh-keypair"),
							Entry("ssh-keypair.old", "ssh-keypair.old"),
							Entry("monitoring", "monitoring"),
						)
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

						It("should forbid because the related Shoot does not belong to gardenlet's shoot", func() {
							backupBucket := &gardencorev1beta1.BackupBucket{
								ObjectMeta: metav1.ObjectMeta{Name: name},
								Spec:       gardencorev1beta1.BackupBucketSpec{ShootRef: &corev1.ObjectReference{Name: "other-shoot", Namespace: "other-namespace"}},
							}
							Expect(fakeClient.Create(ctx, backupBucket)).To(Succeed())

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
							backupBucket := &gardencorev1beta1.BackupBucket{
								ObjectMeta: metav1.ObjectMeta{Name: name},
								Spec:       gardencorev1beta1.BackupBucketSpec{ShootRef: &corev1.ObjectReference{Name: shootName, Namespace: shootNamespace}},
							}
							Expect(fakeClient.Create(ctx, backupBucket)).To(Succeed())

							shoot := &gardencorev1beta1.Shoot{
								ObjectMeta: metav1.ObjectMeta{Name: shootName, Namespace: shootNamespace},
								Status:     gardencorev1beta1.ShootStatus{UID: types.UID(name)},
							}
							Expect(fakeClient.Create(ctx, shoot)).To(Succeed())

							Expect(handler.Handle(ctx, request)).To(Equal(responseAllowed))
						})
					})

					Context("ManagedSeed bootstrap token secret", func() {
						BeforeEach(func() {
							request.Name = "bootstrap-token-abcdef"
							request.Namespace = metav1.NamespaceSystem
						})

						It("should allow because the ManagedSeed belongs to gardenlet's shoot", func() {
							managedSeed := &seedmanagementv1alpha1.ManagedSeed{
								ObjectMeta: metav1.ObjectMeta{Name: "my-managed-seed", Namespace: shootNamespace},
								Spec: seedmanagementv1alpha1.ManagedSeedSpec{
									Shoot: &seedmanagementv1alpha1.Shoot{Name: shootName},
								},
							}
							Expect(fakeClient.Create(ctx, managedSeed)).To(Succeed())

							secret := &corev1.Secret{
								ObjectMeta: metav1.ObjectMeta{Name: "bootstrap-token-abcdef", Namespace: metav1.NamespaceSystem},
								Type:       corev1.SecretTypeBootstrapToken,
								Data: map[string][]byte{
									"description": []byte("A bootstrap token for the Gardenlet for seedmanagement.gardener.cloud/v1alpha1.ManagedSeed resource " + shootNamespace + "/my-managed-seed."),
								},
							}
							objData, err := runtime.Encode(encoder, secret)
							Expect(err).NotTo(HaveOccurred())
							request.Object.Raw = objData

							Expect(handler.Handle(ctx, request)).To(Equal(responseAllowed))
						})

						It("should forbid because the ManagedSeed does not belong to gardenlet's shoot", func() {
							managedSeed := &seedmanagementv1alpha1.ManagedSeed{
								ObjectMeta: metav1.ObjectMeta{Name: "my-managed-seed", Namespace: "other-namespace"},
								Spec: seedmanagementv1alpha1.ManagedSeedSpec{
									Shoot: &seedmanagementv1alpha1.Shoot{Name: "other-shoot"},
								},
							}
							Expect(fakeClient.Create(ctx, managedSeed)).To(Succeed())

							secret := &corev1.Secret{
								ObjectMeta: metav1.ObjectMeta{Name: "bootstrap-token-abcdef", Namespace: metav1.NamespaceSystem},
								Type:       corev1.SecretTypeBootstrapToken,
								Data: map[string][]byte{
									"description": []byte("A bootstrap token for the Gardenlet for seedmanagement.gardener.cloud/v1alpha1.ManagedSeed resource other-namespace/my-managed-seed."),
								},
							}
							objData, err := runtime.Encode(encoder, secret)
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
					})

					Context("Gardenlet bootstrap token secret", func() {
						BeforeEach(func() {
							request.Name = "bootstrap-token-abcdef"
							request.Namespace = metav1.NamespaceSystem
						})

						It("should allow because the Gardenlet belongs to gardenlet's shoot", func() {
							secret := &corev1.Secret{
								ObjectMeta: metav1.ObjectMeta{Name: "bootstrap-token-abcdef", Namespace: metav1.NamespaceSystem},
								Type:       corev1.SecretTypeBootstrapToken,
								Data: map[string][]byte{
									"description": []byte("A bootstrap token for the Gardenlet for seedmanagement.gardener.cloud/v1alpha1.Gardenlet resource " + shootNamespace + "/self-hosted-shoot-" + shootName + "."),
								},
							}
							objData, err := runtime.Encode(encoder, secret)
							Expect(err).NotTo(HaveOccurred())
							request.Object.Raw = objData

							Expect(handler.Handle(ctx, request)).To(Equal(responseAllowed))
						})

						It("should forbid because the Gardenlet does not belong to gardenlet's shoot", func() {
							secret := &corev1.Secret{
								ObjectMeta: metav1.ObjectMeta{Name: "bootstrap-token-abcdef", Namespace: "other-namespace"},
								Type:       corev1.SecretTypeBootstrapToken,
								Data: map[string][]byte{
									"description": []byte("A bootstrap token for the Gardenlet for seedmanagement.gardener.cloud/v1alpha1.Gardenlet resource other-namespace/self-hosted-shoot-other-shoot."),
								},
							}
							objData, err := runtime.Encode(encoder, secret)
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
					})
				})
			})

			When("requested for ServiceAccounts", func() {
				BeforeEach(func() {
					request.Name = "extension-shoot--" + shootName + "--provider-aws"
					request.Namespace = shootNamespace
					request.UserInfo = gardenletUser
					request.Resource = metav1.GroupVersionResource{
						Group:    corev1.SchemeGroupVersion.Group,
						Resource: "serviceaccounts",
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

				When("operation is create", func() {
					BeforeEach(func() {
						request.Operation = admissionv1.Create
					})

					Context("gardenlet client", func() {
						It("should allow when service account is in the shoot's project namespace with the correct name prefix", func() {
							Expect(handler.Handle(ctx, request)).To(Equal(responseAllowed))
						})

						It("should forbid when service account name does not have the required prefix", func() {
							request.Name = "not-prefixed-sa"

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

						It("should forbid when service account is in a different namespace", func() {
							request.Namespace = "other-namespace"

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
					})

					Context("extension client", func() {
						BeforeEach(func() {
							request.UserInfo = extensionUser
						})

						It("should forbid service account creation", func() {
							Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
								AdmissionResponse: admissionv1.AdmissionResponse{
									Allowed: false,
									Result: &metav1.Status{
										Code:    int32(http.StatusForbidden),
										Message: "extension client may not create ServiceAccounts",
									},
								},
							}))
						})
					})
				})
			})

			When("requested for ShootStates", func() {
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

				When("operation is create", func() {
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

			When("resource is shoot", func() {
				BeforeEach(func() {
					request.Resource = metav1.GroupVersionResource{
						Group:    gardencorev1beta1.SchemeGroupVersion.Group,
						Version:  gardencorev1beta1.SchemeGroupVersion.Version,
						Resource: "shoots",
					}
					request.UserInfo = gardenletUser
				})

				When("operation is update", func() {
					BeforeEach(func() {
						request.Operation = admissionv1.Update
						request.Name = shootName
						request.Namespace = shootNamespace
					})

					It("should allow when spec is unchanged", func() {
						oldShoot := &gardencorev1beta1.Shoot{
							ObjectMeta: metav1.ObjectMeta{Name: shootName, Namespace: shootNamespace},
						}
						newShoot := oldShoot.DeepCopy()
						newShoot.Labels = map[string]string{"foo": "bar"}

						oldRaw, err := stdjson.Marshal(oldShoot)
						Expect(err).NotTo(HaveOccurred())
						newRaw, err := stdjson.Marshal(newShoot)
						Expect(err).NotTo(HaveOccurred())
						request.OldObject = runtime.RawExtension{Raw: oldRaw}
						request.Object = runtime.RawExtension{Raw: newRaw}

						Expect(handler.Handle(ctx, request)).To(Equal(responseAllowed))
					})

					It("should allow when only .spec.networking.nodes is set from nil", func() {
						nodes := "10.0.0.0/24"
						oldShoot := &gardencorev1beta1.Shoot{
							ObjectMeta: metav1.ObjectMeta{Name: shootName, Namespace: shootNamespace},
						}
						newShoot := oldShoot.DeepCopy()
						newShoot.Spec.Networking = &gardencorev1beta1.Networking{Nodes: &nodes}

						oldRaw, err := stdjson.Marshal(oldShoot)
						Expect(err).NotTo(HaveOccurred())
						newRaw, err := stdjson.Marshal(newShoot)
						Expect(err).NotTo(HaveOccurred())
						request.OldObject = runtime.RawExtension{Raw: oldRaw}
						request.Object = runtime.RawExtension{Raw: newRaw}

						Expect(handler.Handle(ctx, request)).To(Equal(responseAllowed))
					})

					It("should deny when .spec.networking.nodes is changed from non-nil", func() {
						nodes := "10.0.0.0/24"
						newNodes := "192.168.0.0/24"
						oldShoot := &gardencorev1beta1.Shoot{
							ObjectMeta: metav1.ObjectMeta{Name: shootName, Namespace: shootNamespace},
							Spec:       gardencorev1beta1.ShootSpec{Networking: &gardencorev1beta1.Networking{Nodes: &nodes}},
						}
						newShoot := oldShoot.DeepCopy()
						newShoot.Spec.Networking.Nodes = &newNodes

						oldRaw, err := stdjson.Marshal(oldShoot)
						Expect(err).NotTo(HaveOccurred())
						newRaw, err := stdjson.Marshal(newShoot)
						Expect(err).NotTo(HaveOccurred())
						request.OldObject = runtime.RawExtension{Raw: oldRaw}
						request.Object = runtime.RawExtension{Raw: newRaw}

						response := handler.Handle(ctx, request)
						Expect(response.Allowed).To(BeFalse())
						Expect(response.Result.Message).To(ContainSubstring("must not modify .spec of Shoot"))
					})

					It("should deny when other spec fields are changed alongside .spec.networking.nodes", func() {
						nodes := "10.0.0.0/24"
						oldShoot := &gardencorev1beta1.Shoot{
							ObjectMeta: metav1.ObjectMeta{Name: shootName, Namespace: shootNamespace},
						}
						newShoot := oldShoot.DeepCopy()
						newShoot.Spec.Networking = &gardencorev1beta1.Networking{Nodes: &nodes}
						newShoot.Spec.Region = "eu-west-1"

						oldRaw, err := stdjson.Marshal(oldShoot)
						Expect(err).NotTo(HaveOccurred())
						newRaw, err := stdjson.Marshal(newShoot)
						Expect(err).NotTo(HaveOccurred())
						request.OldObject = runtime.RawExtension{Raw: oldRaw}
						request.Object = runtime.RawExtension{Raw: newRaw}

						response := handler.Handle(ctx, request)
						Expect(response.Allowed).To(BeFalse())
						Expect(response.Result.Message).To(ContainSubstring("must not modify .spec of Shoot"))
					})
				})
			})
		})

		Context("gardenadm client", func() {
			When("requested for ConfigMaps", func() {
				var (
					name, namespace string
				)

				BeforeEach(func() {
					name, namespace = "foo", "bar"

					request.Name = name
					request.Namespace = namespace
					request.UserInfo = gardenadmUser
					request.Resource = metav1.GroupVersionResource{
						Group:    corev1.SchemeGroupVersion.Group,
						Version:  corev1.SchemeGroupVersion.Version,
						Resource: "configmaps",
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

				When("operation is create", func() {
					BeforeEach(func() {
						request.Operation = admissionv1.Create
					})

					It("should deny the request because object namespace does not match shoot namespaces", func() {
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

					It("should allow the request because object namespace matches shoot namespaces", func() {
						request.Namespace = shootNamespace

						Expect(handler.Handle(ctx, request)).To(Equal(responseAllowed))
					})
				})
			})

			When("requested for WorkloadIdentities", func() {
				var (
					name, namespace string
				)

				BeforeEach(func() {
					name, namespace = "foo", "bar"

					request.Name = name
					request.Namespace = namespace
					request.UserInfo = gardenadmUser
					request.Resource = metav1.GroupVersionResource{
						Group:    securityv1alpha1.SchemeGroupVersion.Group,
						Version:  securityv1alpha1.SchemeGroupVersion.Version,
						Resource: "workloadidentities",
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

				When("operation is create", func() {
					BeforeEach(func() {
						request.Operation = admissionv1.Create
					})

					It("should deny the request because object namespace does not match shoot namespaces", func() {
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

					It("should allow the request because object namespace matches shoot namespaces", func() {
						request.Namespace = shootNamespace

						Expect(handler.Handle(ctx, request)).To(Equal(responseAllowed))
					})
				})
			})

			When("requested for Projects", func() {
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

				When("operation is create", func() {
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
								Namespace: new("other-namespace"),
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

			When("requested for Secrets", func() {
				var (
					name, namespace string
				)

				BeforeEach(func() {
					name, namespace = "foo", "bar"

					request.Name = name
					request.Namespace = namespace
					request.UserInfo = gardenadmUser
					request.Resource = metav1.GroupVersionResource{
						Group:    corev1.SchemeGroupVersion.Group,
						Version:  corev1.SchemeGroupVersion.Version,
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

				When("operation is create", func() {
					BeforeEach(func() {
						request.Operation = admissionv1.Create
					})

					It("should deny the request because object namespace does not match shoot namespaces", func() {
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

					It("should allow the request because object namespace matches shoot namespaces", func() {
						request.Namespace = shootNamespace

						Expect(handler.Handle(ctx, request)).To(Equal(responseAllowed))
					})
				})
			})

			When("requested for Shoots", func() {
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

				When("operation is create", func() {
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
