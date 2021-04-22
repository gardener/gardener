// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package seedrestriction_test

import (
	"context"
	"fmt"
	"net/http"

	. "github.com/gardener/gardener/pkg/admissioncontroller/webhooks/admission/seedrestriction"
	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	mockcache "github.com/gardener/gardener/pkg/mock/controller-runtime/cache"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	"github.com/go-logr/logr"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	admissionv1 "k8s.io/api/admission/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	coordinationv1 "k8s.io/api/coordination/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logzap "sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var _ = Describe("handler", func() {
	var (
		ctx     = context.TODO()
		fakeErr = fmt.Errorf("fake")
		err     error

		ctrl      *gomock.Controller
		mockCache *mockcache.MockCache
		decoder   *admission.Decoder

		logger  logr.Logger
		handler admission.Handler
		request admission.Request
		encoder runtime.Encoder

		seedName                string
		seedUser, ambiguousUser authenticationv1.UserInfo

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
		decoder, err = admission.NewDecoder(kubernetes.GardenScheme)
		Expect(err).NotTo(HaveOccurred())

		logger = logzap.New(logzap.WriteTo(GinkgoWriter))
		request = admission.Request{}
		encoder = &json.Serializer{}

		mockCache.EXPECT().GetInformer(ctx, gomock.AssignableToTypeOf(&gardencorev1beta1.BackupBucket{}))
		mockCache.EXPECT().GetInformer(ctx, gomock.AssignableToTypeOf(&gardencorev1beta1.Shoot{}))

		handler, err = New(ctx, logger, mockCache)
		Expect(err).NotTo(HaveOccurred())
		Expect(admission.InjectDecoderInto(decoder, handler)).To(BeTrue())

		seedName = "seed"
		seedUser = authenticationv1.UserInfo{
			Username: fmt.Sprintf("%s%s", v1beta1constants.SeedUserNamePrefix, seedName),
			Groups:   []string{v1beta1constants.SeedsGroup},
		}
		ambiguousUser = authenticationv1.UserInfo{
			Username: fmt.Sprintf("%s%s", v1beta1constants.SeedUserNamePrefix, v1beta1constants.SeedUserNameSuffixAmbiguous),
			Groups:   []string{v1beta1constants.SeedsGroup},
		}
	})

	Describe("#Handle", func() {
		Context("when resource is unhandled", func() {
			It("should have no opinion because no seed", func() {
				request.UserInfo = authenticationv1.UserInfo{Username: "foo"}

				Expect(handler.Handle(ctx, request)).To(Equal(responseAllowed))
			})

			It("should have no opinion because no resource request", func() {
				request.UserInfo = seedUser

				Expect(handler.Handle(ctx, request)).To(Equal(responseAllowed))
			})

			It("should have no opinion because resource is irrelevant", func() {
				request.UserInfo = seedUser
				request.Resource = metav1.GroupVersionResource{}

				Expect(handler.Handle(ctx, request)).To(Equal(responseAllowed))
			})
		})

		Context("when requested for ShootStates", func() {
			var name, namespace string

			BeforeEach(func() {
				name, namespace = "foo", "bar"

				request.Name = name
				request.Namespace = namespace
				request.UserInfo = seedUser
				request.Resource = metav1.GroupVersionResource{
					Group:    gardencorev1alpha1.SchemeGroupVersion.Group,
					Resource: "shootstates",
				}
			})

			DescribeTable("should forbid because no allowed verb",
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
				Entry("connect", admissionv1.Connect),
			)

			Context("when operation is create", func() {
				BeforeEach(func() {
					request.Operation = admissionv1.Create
				})

				It("should return an error because fetching the related shoot failed", func() {
					mockCache.EXPECT().Get(ctx, kutil.Key(namespace, name), gomock.AssignableToTypeOf(&gardencorev1beta1.Shoot{})).Return(fakeErr)

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

				DescribeTable("should forbid the request because the seed name of the related shoot does not match",
					func(seedNameInShoot *string) {
						mockCache.EXPECT().Get(ctx, kutil.Key(namespace, name), gomock.AssignableToTypeOf(&gardencorev1beta1.Shoot{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *gardencorev1beta1.Shoot) error {
							(&gardencorev1beta1.Shoot{Spec: gardencorev1beta1.ShootSpec{SeedName: seedNameInShoot}}).DeepCopyInto(obj)
							return nil
						})

						Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
							AdmissionResponse: admissionv1.AdmissionResponse{
								Allowed: false,
								Result: &metav1.Status{
									Code:    int32(http.StatusForbidden),
									Message: fmt.Sprintf("object does not belong to seed %q", seedName),
								},
							},
						}))
					},

					Entry("seed name is nil", nil),
					Entry("seed name is different", pointer.StringPtr("some-different-seed")),
				)

				It("should allow the request because seed name matches", func() {
					mockCache.EXPECT().Get(ctx, kutil.Key(namespace, name), gomock.AssignableToTypeOf(&gardencorev1beta1.Shoot{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *gardencorev1beta1.Shoot) error {
						(&gardencorev1beta1.Shoot{Spec: gardencorev1beta1.ShootSpec{SeedName: &seedName}}).DeepCopyInto(obj)
						return nil
					})

					Expect(handler.Handle(ctx, request)).To(Equal(responseAllowed))
				})

				It("should allow the request because seed name is ambiguous", func() {
					request.UserInfo = ambiguousUser

					mockCache.EXPECT().Get(ctx, kutil.Key(namespace, name), gomock.AssignableToTypeOf(&gardencorev1beta1.Shoot{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *gardencorev1beta1.Shoot) error {
						(&gardencorev1beta1.Shoot{Spec: gardencorev1beta1.ShootSpec{SeedName: pointer.StringPtr("some-different-seed")}}).DeepCopyInto(obj)
						return nil
					})

					Expect(handler.Handle(ctx, request)).To(Equal(responseAllowed))
				})
			})
		})

		Context("when requested for BackupBuckets", func() {
			var name string

			BeforeEach(func() {
				name = "foo"

				request.Name = name
				request.UserInfo = seedUser
				request.Resource = metav1.GroupVersionResource{
					Group:    gardencorev1beta1.SchemeGroupVersion.Group,
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
				Entry("connect", admissionv1.Connect),
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

				DescribeTable("should forbid the request because the seed name of the related bucket does not match",
					func(seedNameInBackupBucket *string) {
						objData, err := runtime.Encode(encoder, &gardencorev1beta1.BackupBucket{
							Spec: gardencorev1beta1.BackupBucketSpec{
								SeedName: seedNameInBackupBucket,
							},
						})
						Expect(err).NotTo(HaveOccurred())
						request.Object.Raw = objData

						Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
							AdmissionResponse: admissionv1.AdmissionResponse{
								Allowed: false,
								Result: &metav1.Status{
									Code:    int32(http.StatusForbidden),
									Message: fmt.Sprintf("object does not belong to seed %q", seedName),
								},
							},
						}))
					},

					Entry("seed name is nil", nil),
					Entry("seed name is different", pointer.StringPtr("some-different-seed")),
				)

				It("should allow the request because seed name matches", func() {
					objData, err := runtime.Encode(encoder, &gardencorev1beta1.BackupBucket{
						Spec: gardencorev1beta1.BackupBucketSpec{
							SeedName: &seedName,
						},
					})
					Expect(err).NotTo(HaveOccurred())
					request.Object.Raw = objData

					Expect(handler.Handle(ctx, request)).To(Equal(responseAllowed))
				})

				It("should allow the request because seed name is ambiguous", func() {
					request.UserInfo = ambiguousUser

					objData, err := runtime.Encode(encoder, &gardencorev1beta1.BackupBucket{
						Spec: gardencorev1beta1.BackupBucketSpec{
							SeedName: pointer.StringPtr("some-different-seed"),
						},
					})
					Expect(err).NotTo(HaveOccurred())
					request.Object.Raw = objData

					Expect(handler.Handle(ctx, request)).To(Equal(responseAllowed))
				})
			})
		})

		Context("when requested for BackupEntrys", func() {
			var name, namespace, bucketName string

			BeforeEach(func() {
				name = "foo"
				namespace = "bar"
				bucketName = "bucket"

				request.Name = name
				request.Namespace = namespace
				request.UserInfo = seedUser
				request.Resource = metav1.GroupVersionResource{
					Group:    gardencorev1beta1.SchemeGroupVersion.Group,
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
				Entry("connect", admissionv1.Connect),
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

				DescribeTable("should forbid the request because the seed name of the related entry does not match",
					func(seedNameInBackupEntry, seedNameInBackupBucket *string) {
						objData, err := runtime.Encode(encoder, &gardencorev1beta1.BackupEntry{
							Spec: gardencorev1beta1.BackupEntrySpec{
								BucketName: bucketName,
								SeedName:   seedNameInBackupEntry,
							},
						})
						Expect(err).NotTo(HaveOccurred())
						request.Object.Raw = objData

						if seedNameInBackupEntry != nil && *seedNameInBackupEntry == seedName {
							mockCache.EXPECT().Get(ctx, kutil.Key(bucketName), gomock.AssignableToTypeOf(&gardencorev1beta1.BackupBucket{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *gardencorev1beta1.BackupBucket) error {
								(&gardencorev1beta1.BackupBucket{Spec: gardencorev1beta1.BackupBucketSpec{SeedName: seedNameInBackupBucket}}).DeepCopyInto(obj)
								return nil
							})
						}

						Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
							AdmissionResponse: admissionv1.AdmissionResponse{
								Allowed: false,
								Result: &metav1.Status{
									Code:    int32(http.StatusForbidden),
									Message: fmt.Sprintf("object does not belong to seed %q", seedName),
								},
							},
						}))
					},

					Entry("seed name is nil", nil, nil),
					Entry("seed name is different", pointer.StringPtr("some-different-seed"), nil),
					Entry("seed name is equal but bucket's seed name is nil", &seedName, nil),
					Entry("seed name is equal but bucket's seed name is different", &seedName, pointer.StringPtr("some-different-seed")),
				)

				It("should allow the request because seed name matches for both entry and bucket", func() {
					objData, err := runtime.Encode(encoder, &gardencorev1beta1.BackupEntry{
						Spec: gardencorev1beta1.BackupEntrySpec{
							BucketName: bucketName,
							SeedName:   &seedName,
						},
					})
					Expect(err).NotTo(HaveOccurred())
					request.Object.Raw = objData

					mockCache.EXPECT().Get(ctx, kutil.Key(bucketName), gomock.AssignableToTypeOf(&gardencorev1beta1.BackupBucket{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *gardencorev1beta1.BackupBucket) error {
						(&gardencorev1beta1.BackupBucket{Spec: gardencorev1beta1.BackupBucketSpec{SeedName: &seedName}}).DeepCopyInto(obj)
						return nil
					})

					Expect(handler.Handle(ctx, request)).To(Equal(responseAllowed))
				})

				DescribeTable("should allow the request because seed name is ambiguous",
					func(seedNameInBackupEntry, seedNameInBackupBucket *string) {
						request.UserInfo = ambiguousUser

						objData, err := runtime.Encode(encoder, &gardencorev1beta1.BackupEntry{
							Spec: gardencorev1beta1.BackupEntrySpec{
								BucketName: bucketName,
								SeedName:   seedNameInBackupEntry,
							},
						})
						Expect(err).NotTo(HaveOccurred())
						request.Object.Raw = objData

						mockCache.EXPECT().Get(ctx, kutil.Key(bucketName), gomock.AssignableToTypeOf(&gardencorev1beta1.BackupBucket{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *gardencorev1beta1.BackupBucket) error {
							(&gardencorev1beta1.BackupBucket{Spec: gardencorev1beta1.BackupBucketSpec{SeedName: seedNameInBackupBucket}}).DeepCopyInto(obj)
							return nil
						})

						Expect(handler.Handle(ctx, request)).To(Equal(responseAllowed))
					},

					Entry("seed name nil", nil, nil),
					Entry("seed name is different", pointer.StringPtr("some-different-seed"), nil),
					Entry("seed name is equal but bucket's seed name is nil", &seedName, nil),
					Entry("seed name is equal but bucket's seed name is different", &seedName, pointer.StringPtr("some-different-seed")),
				)
			})
		})

		Context("when requested for Leases", func() {
			var name, namespace string

			BeforeEach(func() {
				name, namespace = "foo", "bar"

				request.Name = name
				request.Namespace = namespace
				request.UserInfo = seedUser
				request.Resource = metav1.GroupVersionResource{
					Group:    coordinationv1.SchemeGroupVersion.Group,
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
				Entry("connect", admissionv1.Connect),
			)

			Context("when operation is create", func() {
				BeforeEach(func() {
					request.Operation = admissionv1.Create
				})

				DescribeTable("should forbid the request because the seed name of the lease does not match",
					func(seedNameInLease string) {
						request.Name = seedNameInLease

						Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
							AdmissionResponse: admissionv1.AdmissionResponse{
								Allowed: false,
								Result: &metav1.Status{
									Code:    int32(http.StatusForbidden),
									Message: fmt.Sprintf("object does not belong to seed %q", seedName),
								},
							},
						}))
					},

					Entry("seed name is different", "some-different-seed"),
				)

				It("should allow the request because lease is used for leader-election", func() {
					request.Name = "gardenlet-leader-election"

					Expect(handler.Handle(ctx, request)).To(Equal(responseAllowed))
				})

				It("should allow the request because seed name matches", func() {
					request.Name = seedName

					Expect(handler.Handle(ctx, request)).To(Equal(responseAllowed))
				})

				It("should allow the request because seed name is ambiguous", func() {
					request.Name = "some-different-seed"
					request.UserInfo = ambiguousUser

					Expect(handler.Handle(ctx, request)).To(Equal(responseAllowed))
				})
			})
		})

		Context("when requested for Seeds", func() {
			var name string

			BeforeEach(func() {
				name = "foo"

				request.Name = name
				request.UserInfo = seedUser
				request.Resource = metav1.GroupVersionResource{
					Group:    gardencorev1beta1.SchemeGroupVersion.Group,
					Resource: "seeds",
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
				Entry("connect", admissionv1.Connect),
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

				DescribeTable("should forbid the request because the seed name of the object does not match",
					func(seedNameOfObject string) {
						objData, err := runtime.Encode(encoder, &gardencorev1beta1.Seed{
							ObjectMeta: metav1.ObjectMeta{
								Name: seedNameOfObject,
							},
						})
						Expect(err).NotTo(HaveOccurred())
						request.Object.Raw = objData

						Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
							AdmissionResponse: admissionv1.AdmissionResponse{
								Allowed: false,
								Result: &metav1.Status{
									Code:    int32(http.StatusForbidden),
									Message: fmt.Sprintf("object does not belong to seed %q", seedName),
								},
							},
						}))
					},

					Entry("seed name is different", "some-different-seed"),
				)

				It("should allow the request because seed name matches", func() {
					objData, err := runtime.Encode(encoder, &gardencorev1beta1.Seed{
						ObjectMeta: metav1.ObjectMeta{
							Name: seedName,
						},
					})
					Expect(err).NotTo(HaveOccurred())
					request.Object.Raw = objData

					Expect(handler.Handle(ctx, request)).To(Equal(responseAllowed))
				})

				It("should allow the request because seed name is ambiguous", func() {
					request.UserInfo = ambiguousUser

					objData, err := runtime.Encode(encoder, &gardencorev1beta1.Seed{
						ObjectMeta: metav1.ObjectMeta{
							Name: "some-different-seed",
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
