// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package seedrestriction_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionv1 "k8s.io/api/admission/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	certificatesv1 "k8s.io/api/certificates/v1"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/apiserver/pkg/authentication/serviceaccount"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	logzap "sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	. "github.com/gardener/gardener/pkg/admissioncontroller/webhook/admission/seedrestriction"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/apis/config/gardenlet/v1alpha1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	operationsv1alpha1 "github.com/gardener/gardener/pkg/apis/operations/v1alpha1"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/logger"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

var _ = Describe("handler", func() {
	var (
		ctx     = context.TODO()
		fakeErr = errors.New("fake")
		err     error

		fakeClient client.Client
		decoder    admission.Decoder

		log     logr.Logger
		handler admission.Handler
		request admission.Request
		encoder runtime.Encoder

		seedName      string
		seedUser      authenticationv1.UserInfo
		gardenletUser authenticationv1.UserInfo
		extensionUser authenticationv1.UserInfo

		responseAllowed admission.Response
	)

	BeforeEach(func() {
		decoder = admission.NewDecoder(kubernetes.GardenScheme)
		Expect(err).NotTo(HaveOccurred())
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).Build()

		log = logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, logzap.WriteTo(GinkgoWriter))
		request = admission.Request{}
		encoder = &json.Serializer{}

		handler = &Handler{Logger: log, Client: fakeClient, Decoder: decoder}

		seedName = "seed"
		gardenletUser = authenticationv1.UserInfo{
			Username: fmt.Sprintf("%s%s", v1beta1constants.SeedUserNamePrefix, seedName),
			Groups:   []string{v1beta1constants.SeedsGroup},
		}
		extensionUserInfo := (&serviceaccount.ServiceAccountInfo{
			Name:      v1beta1constants.ExtensionGardenServiceAccountPrefix + "provider-local",
			Namespace: gardenerutils.SeedNamespaceNamePrefix + seedName,
		}).UserInfo()
		extensionUser = authenticationv1.UserInfo{
			Username: extensionUserInfo.GetName(),
			Groups:   extensionUserInfo.GetGroups(),
		}

		responseAllowed = admission.Response{
			AdmissionResponse: admissionv1.AdmissionResponse{
				Allowed: true,
				Result: &metav1.Status{
					Code: int32(http.StatusOK),
				},
			},
		}
	})

	Describe("#Handle", func() {
		Context("when resource is unhandled", func() {
			It("should have no opinion because no seed", func() {
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

		Context("when resource already exists and operation is CREATE", func() {
			It("should allow the request", func() {
				request.UserInfo = gardenletUser
				request.Operation = admissionv1.Create
				request.Kind = metav1.GroupVersionKind{
					Group:   gardencorev1beta1.SchemeGroupVersion.Group,
					Version: gardencorev1beta1.SchemeGroupVersion.Version,
					Kind:    "BackupEntry",
				}
				request.Resource = metav1.GroupVersionResource{
					Group:    gardencorev1beta1.SchemeGroupVersion.Group,
					Resource: "backupentries",
				}
				request.Name = "foo"
				request.Namespace = "bar"

				backupEntry := &gardencorev1beta1.BackupEntry{
					ObjectMeta: metav1.ObjectMeta{
						Name:      request.Name,
						Namespace: request.Namespace,
					},
				}
				Expect(fakeClient.Create(ctx, backupEntry)).To(Succeed())

				responseAllowed.Result.Message = "object already exists"
				Expect(handler.Handle(ctx, request)).To(Equal(responseAllowed))
			})
		})

		testCommonAccess := func() {
			Context("when requested for ShootStates", func() {
				var name, namespace string

				BeforeEach(func() {
					name, namespace = "foo", "bar"

					request.Name = name
					request.Namespace = namespace
					request.UserInfo = seedUser
					request.Resource = metav1.GroupVersionResource{
						Group:    gardencorev1beta1.SchemeGroupVersion.Group,
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
				)

				Context("when operation is create", func() {
					BeforeEach(func() {
						request.Operation = admissionv1.Create
					})

					It("should return an error because fetching the related shoot failed", func() {
						Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
							AdmissionResponse: admissionv1.AdmissionResponse{
								Allowed: false,
								Result: &metav1.Status{
									Code:    int32(http.StatusInternalServerError),
									Message: `shoots.core.gardener.cloud "foo" not found`,
								},
							},
						}))
					})

					DescribeTable("should forbid the request because the seed name of the related shoot does not match",
						func(seedNameInShoot *string) {
							shoot := &gardencorev1beta1.Shoot{
								ObjectMeta: metav1.ObjectMeta{
									Name:      name,
									Namespace: namespace,
								},
								Spec: gardencorev1beta1.ShootSpec{
									SeedName: seedNameInShoot,
								},
							}

							Expect(fakeClient.Create(ctx, shoot)).To(Succeed())

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
						Entry("seed name is different", ptr.To("some-different-seed")),
					)

					It("should allow the request because seed name in spec matches", func() {
						shoot := &gardencorev1beta1.Shoot{
							ObjectMeta: metav1.ObjectMeta{
								Name:      name,
								Namespace: namespace,
							},
							Spec: gardencorev1beta1.ShootSpec{
								SeedName: &seedName,
							},
						}

						Expect(fakeClient.Create(ctx, shoot)).To(Succeed())

						Expect(handler.Handle(ctx, request)).To(Equal(responseAllowed))
					})

					It("should allow the request because seed name in status matches", func() {
						shoot := &gardencorev1beta1.Shoot{
							ObjectMeta: metav1.ObjectMeta{
								Name:      name,
								Namespace: namespace,
							},
							Status: gardencorev1beta1.ShootStatus{
								SeedName: &seedName,
							},
						}

						Expect(fakeClient.Create(ctx, shoot)).To(Succeed())

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
						Entry("seed name is different", ptr.To("some-different-seed")),
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
				})

				Context("when operation is delete", func() {
					BeforeEach(func() {
						request.Operation = admissionv1.Delete
					})

					It("should return an error because reading the Seed failed", func() {
						Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
							AdmissionResponse: admissionv1.AdmissionResponse{
								Allowed: false,
								Result: &metav1.Status{
									Code:    int32(http.StatusInternalServerError),
									Message: `seeds.core.gardener.cloud "seed" not found`,
								},
							},
						}))
					})

					It("should forbid the request because the seed UID and the bucket name does not match", func() {
						seed := &gardencorev1beta1.Seed{
							ObjectMeta: metav1.ObjectMeta{
								Name: seedName,
							},
						}
						Expect(fakeClient.Create(ctx, seed)).To(Succeed())

						Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
							AdmissionResponse: admissionv1.AdmissionResponse{
								Allowed: false,
								Result: &metav1.Status{
									Code:    int32(http.StatusForbidden),
									Message: "cannot delete unrelated BackupBucket",
								},
							},
						}))
					})

					It("should allow the request because the seed UID and the bucket name does match", func() {
						seed := &gardencorev1beta1.Seed{
							ObjectMeta: metav1.ObjectMeta{
								Name: seedName,
							},
						}
						Expect(fakeClient.Create(ctx, seed)).To(Succeed())

						request.Name = string(seed.UID)

						Expect(handler.Handle(ctx, request)).To(Equal(responseAllowed))
					})
				})
			})

			Context("when requested for BackupEntries", func() {
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
								backupBucket := &gardencorev1beta1.BackupBucket{
									ObjectMeta: metav1.ObjectMeta{
										Name: bucketName,
									},
									Spec: gardencorev1beta1.BackupBucketSpec{
										SeedName: seedNameInBackupBucket,
									},
								}

								Expect(fakeClient.Create(ctx, backupBucket)).To(Succeed())
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
						Entry("seed name is different", ptr.To("some-different-seed"), nil),
						Entry("seed name is equal but bucket's seed name is nil", &seedName, nil),
						Entry("seed name is equal but bucket's seed name is different", &seedName, ptr.To("some-different-seed")),
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

						backupBucket := &gardencorev1beta1.BackupBucket{
							ObjectMeta: metav1.ObjectMeta{
								Name: bucketName,
							},
							Spec: gardencorev1beta1.BackupBucketSpec{
								SeedName: &seedName,
							},
						}
						Expect(fakeClient.Create(ctx, backupBucket)).To(Succeed())

						Expect(handler.Handle(ctx, request)).To(Equal(responseAllowed))
					})

					Context("when creating a source BackupEntry", func() {
						const (
							shootBackupEntryName = "backupentry"
							shootName            = "foo"
						)

						var shoot *gardencorev1beta1.Shoot

						BeforeEach(func() {
							objData, err := runtime.Encode(encoder, &gardencorev1beta1.BackupEntry{
								ObjectMeta: metav1.ObjectMeta{
									Name:      fmt.Sprintf("%s-%s", v1beta1constants.BackupSourcePrefix, shootBackupEntryName),
									Namespace: namespace,
									OwnerReferences: []metav1.OwnerReference{
										{
											Name: shootName,
											Kind: "Shoot",
										},
									},
								},
								Spec: gardencorev1beta1.BackupEntrySpec{
									BucketName: bucketName,
									SeedName:   &seedName,
								},
							})
							Expect(err).NotTo(HaveOccurred())
							request.Object.Raw = objData

							shoot = &gardencorev1beta1.Shoot{
								ObjectMeta: metav1.ObjectMeta{
									Name:      shootName,
									Namespace: namespace,
								},
								Status: gardencorev1beta1.ShootStatus{
									LastOperation: &gardencorev1beta1.LastOperation{
										Type:  gardencorev1beta1.LastOperationTypeRestore,
										State: gardencorev1beta1.LastOperationStateProcessing,
									},
								},
							}
						})

						It("should forbid the request because the shoot owning the source BackupEntry could not be found", func() {
							Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
								AdmissionResponse: admissionv1.AdmissionResponse{
									Allowed: false,
									Result: &metav1.Status{
										Code:    int32(http.StatusInternalServerError),
										Message: apierrors.NewNotFound(schema.GroupResource{Group: gardencorev1beta1.SchemeGroupVersion.Group, Resource: "shoots"}, shootName).Error(),
									},
								},
							}))
						})

						DescribeTable("should forbid the request because a the shoot owning the source BackupEntry is not in restore phase",
							func(lastOperation *gardencorev1beta1.LastOperation) {
								shoot := &gardencorev1beta1.Shoot{
									ObjectMeta: metav1.ObjectMeta{
										Name:      shootName,
										Namespace: namespace,
									},
									Status: gardencorev1beta1.ShootStatus{
										LastOperation: lastOperation,
									},
								}

								Expect(fakeClient.Create(ctx, shoot)).To(Succeed())

								Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
									AdmissionResponse: admissionv1.AdmissionResponse{
										Allowed: false,
										Result: &metav1.Status{
											Code:    int32(http.StatusForbidden),
											Message: fmt.Sprintf("creation of source BackupEntry is only allowed during shoot Restore operation (shoot: %s)", shootName),
										},
									},
								}))
							},
							Entry("lastOperation is nil", nil),
							Entry("lastOperation is create", &gardencorev1beta1.LastOperation{Type: gardencorev1beta1.LastOperationTypeCreate}),
							Entry("lastOperation is reconcile", &gardencorev1beta1.LastOperation{Type: gardencorev1beta1.LastOperationTypeReconcile}),
							Entry("lastOperation is delete", &gardencorev1beta1.LastOperation{Type: gardencorev1beta1.LastOperationTypeDelete}),
							Entry("lastOperation is migrate", &gardencorev1beta1.LastOperation{Type: gardencorev1beta1.LastOperationTypeMigrate}),
						)

						It("should forbid the request because a BackupEntry for the shoot does not exist", func() {
							Expect(fakeClient.Create(ctx, shoot)).To(Succeed())

							Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
								AdmissionResponse: admissionv1.AdmissionResponse{
									Allowed: false,
									Result: &metav1.Status{
										Code:    int32(http.StatusForbidden),
										Message: fmt.Sprintf("could not find original BackupEntry %s: %v", shootBackupEntryName, apierrors.NewNotFound(schema.GroupResource{Group: gardencorev1beta1.SchemeGroupVersion.Group, Resource: "backupentries"}, shootBackupEntryName).Error()),
									},
								},
							}))
						})

						It("should forbid the request because the source BackupEntry does not match the BackupEntry for the shoot", func() {
							Expect(fakeClient.Create(ctx, shoot)).To(Succeed())

							backupEntry := &gardencorev1beta1.BackupEntry{
								ObjectMeta: metav1.ObjectMeta{
									Name:      shootBackupEntryName,
									Namespace: namespace,
								},
								Spec: gardencorev1beta1.BackupEntrySpec{
									BucketName: "some-different-bucket",
									SeedName:   ptr.To("some-different-seedname"),
								},
							}

							Expect(fakeClient.Create(ctx, backupEntry)).To(Succeed())

							Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
								AdmissionResponse: admissionv1.AdmissionResponse{
									Allowed: false,
									Result: &metav1.Status{
										Code:    int32(http.StatusForbidden),
										Message: "specification of source BackupEntry must equal specification of original BackupEntry " + shootBackupEntryName,
									},
								},
							}))
						})

						It("should allow creation of source BackupEntry if a matching BackupEntry exists and shoot is in restore phase", func() {
							Expect(fakeClient.Create(ctx, shoot)).To(Succeed())

							backupEntry := &gardencorev1beta1.BackupEntry{
								ObjectMeta: metav1.ObjectMeta{
									Name:      shootBackupEntryName,
									Namespace: namespace,
								},
								Spec: gardencorev1beta1.BackupEntrySpec{
									BucketName: bucketName,
									SeedName:   &seedName,
								},
							}

							Expect(fakeClient.Create(ctx, backupEntry)).To(Succeed())

							Expect(handler.Handle(ctx, request)).To(Equal(responseAllowed))
						})
					})
				})
			})

			Context("when requested for Bastions", func() {
				var name string

				BeforeEach(func() {
					name = "foo"

					request.Name = name
					request.UserInfo = seedUser
					request.Resource = metav1.GroupVersionResource{
						Group:    operationsv1alpha1.SchemeGroupVersion.Group,
						Resource: "bastions",
					}
				})

				DescribeTable("should have no opinion because no allowed verb",
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

					It("should allow the request because seed name matches", func() {
						objData, err := runtime.Encode(encoder, &operationsv1alpha1.Bastion{
							Spec: operationsv1alpha1.BastionSpec{
								SeedName: &seedName,
							},
						})
						Expect(err).NotTo(HaveOccurred())
						request.Object.Raw = objData

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

				generateTestsForOperation := func(operation admissionv1.Operation) func() {
					return func() {
						BeforeEach(func() {
							request.Operation = operation
						})

						It("should allow the request because seed name matches", func() {
							request.Name = seedName

							Expect(handler.Handle(ctx, request)).To(Equal(responseAllowed))
						})

						Context("requiring information from managedseed", func() {
							var (
								differentSeedName    = "some-different-seed"
								managedSeedNamespace = "garden"
								shootName            = "shoot"
							)

							BeforeEach(func() {
								request.Name = differentSeedName
							})

							It("should forbid the request because seed does not belong to a managedseed", func() {
								Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
									AdmissionResponse: admissionv1.AdmissionResponse{
										Allowed: false,
										Result: &metav1.Status{
											Code:    int32(http.StatusForbidden),
											Message: fmt.Sprintf("object does not belong to seed %q", seedName),
										},
									},
								}))
							})

							if operation == admissionv1.Delete {
								It("should forbid the request because an error occurred while fetching the managedseed", func() {
									fakeClient := fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).WithInterceptorFuncs(interceptor.Funcs{
										Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
											if _, ok := obj.(*seedmanagementv1alpha1.ManagedSeed); ok {
												return fakeErr
											}
											return c.Get(ctx, key, obj, opts...)
										},
									}).Build()

									handler = &Handler{Logger: log, Client: fakeClient, Decoder: decoder}

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

								It("should forbid the request because managedseed's `.metadata.deletionTimestamp` is nil", func() {
									managedSeed := &seedmanagementv1alpha1.ManagedSeed{
										ObjectMeta: metav1.ObjectMeta{
											Name:      differentSeedName,
											Namespace: managedSeedNamespace,
										},
									}

									Expect(fakeClient.Create(ctx, managedSeed)).To(Succeed())

									Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
										AdmissionResponse: admissionv1.AdmissionResponse{
											Allowed: false,
											Result: &metav1.Status{
												Code:    int32(http.StatusForbidden),
												Message: "object can only be deleted if corresponding ManagedSeed has a deletion timestamp",
											},
										},
									}))
								})
							}

							if operation == admissionv1.Delete {
								Context("requiring information from shoot", func() {
									It("should forbid the request because managedseed's `.spec.shoot` is nil", func() {
										managedSeed := &seedmanagementv1alpha1.ManagedSeed{
											ObjectMeta: metav1.ObjectMeta{
												Name:       differentSeedName,
												Namespace:  managedSeedNamespace,
												Finalizers: []string{"finalizer"},
											},
											Spec: seedmanagementv1alpha1.ManagedSeedSpec{},
										}

										Expect(fakeClient.Create(ctx, managedSeed)).To(Succeed())
										Expect(fakeClient.Delete(ctx, managedSeed)).To(Succeed())

										Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
											AdmissionResponse: admissionv1.AdmissionResponse{
												Allowed: false,
												Result: &metav1.Status{
													Code:    int32(http.StatusForbidden),
													Message: fmt.Sprintf("object does not belong to seed %q", seedName),
												},
											},
										}))
									})

									It("should forbid the request because reading the shoot referenced by the managedseed failed", func() {
										managedSeed := &seedmanagementv1alpha1.ManagedSeed{
											ObjectMeta: metav1.ObjectMeta{
												Name:       differentSeedName,
												Namespace:  managedSeedNamespace,
												Finalizers: []string{"finalizer"},
											},
											Spec: seedmanagementv1alpha1.ManagedSeedSpec{
												Shoot: &seedmanagementv1alpha1.Shoot{Name: shootName},
											},
										}

										Expect(fakeClient.Create(ctx, managedSeed)).To(Succeed())
										Expect(fakeClient.Delete(ctx, managedSeed)).To(Succeed())

										Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
											AdmissionResponse: admissionv1.AdmissionResponse{
												Allowed: false,
												Result: &metav1.Status{
													Code:    int32(http.StatusInternalServerError),
													Message: apierrors.NewNotFound(schema.GroupResource{Group: gardencorev1beta1.SchemeGroupVersion.Group, Resource: "shoots"}, shootName).Error(),
												},
											},
										}))
									})

									DescribeTable("should forbid the request because the seed name of the shoot referenced by the managedseed does not match",
										func(seedNameInShoot *string) {
											managedSeed := &seedmanagementv1alpha1.ManagedSeed{
												ObjectMeta: metav1.ObjectMeta{
													Name:       differentSeedName,
													Namespace:  managedSeedNamespace,
													Finalizers: []string{"finalizer"},
												},
												Spec: seedmanagementv1alpha1.ManagedSeedSpec{
													Shoot: &seedmanagementv1alpha1.Shoot{Name: shootName},
												},
											}

											Expect(fakeClient.Create(ctx, managedSeed)).To(Succeed())
											Expect(fakeClient.Delete(ctx, managedSeed)).To(Succeed())

											shoot := &gardencorev1beta1.Shoot{
												ObjectMeta: metav1.ObjectMeta{
													Name:      shootName,
													Namespace: managedSeedNamespace,
												},
												Spec: gardencorev1beta1.ShootSpec{
													SeedName: seedNameInShoot,
												},
											}

											Expect(fakeClient.Create(ctx, shoot)).To(Succeed())

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
										Entry("seed name is different", ptr.To("some-different-seed")),
									)

									It("should allow the request because the seed name of the shoot referenced by the managedseed matches", func() {
										managedSeed := &seedmanagementv1alpha1.ManagedSeed{
											ObjectMeta: metav1.ObjectMeta{
												Name:       differentSeedName,
												Namespace:  managedSeedNamespace,
												Finalizers: []string{"finalizer"},
											},
											Spec: seedmanagementv1alpha1.ManagedSeedSpec{
												Shoot: &seedmanagementv1alpha1.Shoot{Name: shootName},
											},
										}

										Expect(fakeClient.Create(ctx, managedSeed)).To(Succeed())
										Expect(fakeClient.Delete(ctx, managedSeed)).To(Succeed())

										shoot := &gardencorev1beta1.Shoot{
											ObjectMeta: metav1.ObjectMeta{
												Name:      shootName,
												Namespace: managedSeedNamespace,
											},
											Spec: gardencorev1beta1.ShootSpec{
												SeedName: &seedName,
											},
										}

										Expect(fakeClient.Create(ctx, shoot)).To(Succeed())

										Expect(handler.Handle(ctx, request)).To(Equal(responseAllowed))
									})
								})
							}
						})
					}
				}

				Context("when operation is create", generateTestsForOperation(admissionv1.Create))
				Context("when operation is update", generateTestsForOperation(admissionv1.Update))
				Context("when operation is delete", generateTestsForOperation(admissionv1.Delete))
			})

			Context("when requested for Secrets", func() {
				var name, namespace string

				BeforeEach(func() {
					name, namespace = "foo", "bar"

					request.Name = name
					request.Namespace = namespace
					request.UserInfo = seedUser
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

					It("should forbid the request because it's no expected secret", func() {
						Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
							AdmissionResponse: admissionv1.AdmissionResponse{
								Allowed: false,
								Result: &metav1.Status{
									Code:    int32(http.StatusForbidden),
									Message: fmt.Sprintf("object does not belong to seed %q", seedName),
								},
							},
						}))
					})

					Context("backupbucket secret", func() {
						BeforeEach(func() {
							request.Name = "generated-bucket-" + name
						})

						It("should return an error because the related backupbucket was not found", func() {
							Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
								AdmissionResponse: admissionv1.AdmissionResponse{
									Allowed: false,
									Result: &metav1.Status{
										Code:    int32(http.StatusForbidden),
										Message: apierrors.NewNotFound(schema.GroupResource{Group: gardencorev1beta1.SchemeGroupVersion.Group, Resource: "backupbuckets"}, name).Error(),
									},
								},
							}))
						})

						It("should return an error because the related backupbucket could not be read", func() {
							fakeClient := fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).WithInterceptorFuncs(interceptor.Funcs{
								Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
									if _, ok := obj.(*gardencorev1beta1.BackupBucket); ok {
										return fakeErr
									}
									return c.Get(ctx, key, obj, opts...)
								},
							}).Build()

							handler = &Handler{Logger: log, Client: fakeClient, Decoder: decoder}

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

						It("should forbid because the related backupbucket does not belong to gardenlet's seed", func() {
							backupBucket := &gardencorev1beta1.BackupBucket{
								ObjectMeta: metav1.ObjectMeta{
									Name: name,
								},
								Spec: gardencorev1beta1.BackupBucketSpec{
									SeedName: ptr.To("some-different-seed"),
								},
							}
							Expect(fakeClient.Create(ctx, backupBucket)).To(Succeed())

							Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
								AdmissionResponse: admissionv1.AdmissionResponse{
									Allowed: false,
									Result: &metav1.Status{
										Code:    int32(http.StatusForbidden),
										Message: fmt.Sprintf("object does not belong to seed %q", seedName),
									},
								},
							}))
						})

						It("should allow because the related backupbucket does belong to gardenlet's seed", func() {
							backupBucket := gardencorev1beta1.BackupBucket{
								ObjectMeta: metav1.ObjectMeta{
									Name: name,
								},
								Spec: gardencorev1beta1.BackupBucketSpec{
									SeedName: &seedName,
								},
							}

							Expect(fakeClient.Create(ctx, &backupBucket)).To(Succeed())

							Expect(handler.Handle(ctx, request)).To(Equal(responseAllowed))
						})
					})

					Context("shoot-related project secret", func() {
						testSuite := func(suffix string) {
							BeforeEach(func() {
								request.Name = name + suffix
							})

							It("should return an error because the related shoot was not found", func() {
								Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
									AdmissionResponse: admissionv1.AdmissionResponse{
										Allowed: false,
										Result: &metav1.Status{
											Code:    int32(http.StatusForbidden),
											Message: apierrors.NewNotFound(schema.GroupResource{Group: gardencorev1beta1.SchemeGroupVersion.Group, Resource: "shoots"}, name).Error(),
										},
									},
								}))
							})

							It("should return an error because the related shoot could not be read", func() {
								fakeClient := fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).WithInterceptorFuncs(interceptor.Funcs{
									Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
										if _, ok := obj.(*gardencorev1beta1.Shoot); ok {
											return fakeErr
										}
										return c.Get(ctx, key, obj, opts...)
									},
								}).Build()

								handler = &Handler{Logger: log, Client: fakeClient, Decoder: decoder}

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

							It("should forbid because the related shoot does not belong to gardenlet's seed", func() {
								shoot := &gardencorev1beta1.Shoot{
									ObjectMeta: metav1.ObjectMeta{
										Name:      name,
										Namespace: namespace,
									},
									Spec: gardencorev1beta1.ShootSpec{
										SeedName: ptr.To("some-different-seed"),
									},
								}

								Expect(fakeClient.Create(ctx, shoot)).To(Succeed())

								Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
									AdmissionResponse: admissionv1.AdmissionResponse{
										Allowed: false,
										Result: &metav1.Status{
											Code:    int32(http.StatusForbidden),
											Message: fmt.Sprintf("object does not belong to seed %q", seedName),
										},
									},
								}))
							})

							It("should allow because the related shoot does belong to gardenlet's seed", func() {
								shoot := &gardencorev1beta1.Shoot{
									ObjectMeta: metav1.ObjectMeta{
										Name:      name,
										Namespace: namespace,
									},
									Spec: gardencorev1beta1.ShootSpec{
										SeedName: &seedName,
									},
								}

								Expect(fakeClient.Create(ctx, shoot)).To(Succeed())

								Expect(handler.Handle(ctx, request)).To(Equal(responseAllowed))
							})
						}

						Describe("ca-cluster suffix", func() { testSuite(".ca-cluster") })
						Describe("ssh-keypair suffix", func() { testSuite(".ssh-keypair") })
						Describe("ssh-keypair.old suffix", func() { testSuite(".ssh-keypair.old") })
						Describe("monitoring suffix", func() { testSuite(".monitoring") })
					})

					Context("managed shoot service account issuer secrets", func() {
						BeforeEach(func() {
							request.Namespace = "gardener-system-shoot-issuer"
						})

						DescribeTable("secret is missing labels",
							func(secret *corev1.Secret, expectedResult *metav1.Status) {
								data, err := runtime.Encode(encoder, secret)
								Expect(err).NotTo(HaveOccurred())
								request.Object.Raw = data

								Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
									AdmissionResponse: admissionv1.AdmissionResponse{
										Allowed: false,
										Result:  expectedResult,
									},
								}))
							},

							Entry(
								"missing shoot.gardener.cloud/name label",
								&corev1.Secret{},
								&metav1.Status{
									Code:    int32(http.StatusUnprocessableEntity),
									Message: `label "shoot.gardener.cloud/name" is missing`,
								},
							),
							Entry(
								"missing shoot.gardener.cloud/namespace label",
								&corev1.Secret{
									ObjectMeta: metav1.ObjectMeta{
										Labels: map[string]string{
											"shoot.gardener.cloud/name": "foo",
										},
									},
								},
								&metav1.Status{
									Code:    int32(http.StatusUnprocessableEntity),
									Message: `label "shoot.gardener.cloud/namespace" is missing`,
								},
							),
							Entry(
								"missing authentication.gardener.cloud/public-keys label",
								&corev1.Secret{
									ObjectMeta: metav1.ObjectMeta{
										Labels: map[string]string{
											"shoot.gardener.cloud/name":      "foo",
											"shoot.gardener.cloud/namespace": "foo",
										},
									},
								},
								&metav1.Status{
									Code:    int32(http.StatusUnprocessableEntity),
									Message: `label "authentication.gardener.cloud/public-keys" is missing`,
								},
							),
							Entry(
								"label authentication.gardener.cloud/public-keys has wrong value",
								&corev1.Secret{
									ObjectMeta: metav1.ObjectMeta{
										Labels: map[string]string{
											"shoot.gardener.cloud/name":                 "foo",
											"shoot.gardener.cloud/namespace":            "foo",
											"authentication.gardener.cloud/public-keys": "foo",
										},
									},
								},
								&metav1.Status{
									Code:    int32(http.StatusUnprocessableEntity),
									Message: `label "authentication.gardener.cloud/public-keys" value must be set to "serviceaccount"`,
								},
							),
						)

						Context("secret is configured correctly", func() {
							BeforeEach(func() {
								secret := &corev1.Secret{
									ObjectMeta: metav1.ObjectMeta{
										Labels: map[string]string{
											"shoot.gardener.cloud/name":                 name,
											"shoot.gardener.cloud/namespace":            namespace,
											"authentication.gardener.cloud/public-keys": "serviceaccount",
										},
									},
								}
								data, err := runtime.Encode(encoder, secret)
								Expect(err).NotTo(HaveOccurred())
								request.Object.Raw = data
							})

							It("should return an error because the related shoot was not found", func() {
								Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
									AdmissionResponse: admissionv1.AdmissionResponse{
										Allowed: false,
										Result: &metav1.Status{
											Code:    int32(http.StatusForbidden),
											Message: apierrors.NewNotFound(schema.GroupResource{Group: gardencorev1beta1.SchemeGroupVersion.Group, Resource: "shoots"}, name).Error(),
										},
									},
								}))
							})

							It("should return an error because the related shoot could not be read", func() {
								fakeClient := fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).WithInterceptorFuncs(interceptor.Funcs{
									Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
										if _, ok := obj.(*gardencorev1beta1.Shoot); ok {
											return fakeErr
										}
										return c.Get(ctx, key, obj, opts...)
									},
								}).Build()

								handler = &Handler{Logger: log, Client: fakeClient, Decoder: decoder}

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

							It("should forbid because the related shoot does not belong to gardenlet's seed", func() {
								shoot := &gardencorev1beta1.Shoot{
									ObjectMeta: metav1.ObjectMeta{
										Name:      name,
										Namespace: namespace,
									},
									Spec: gardencorev1beta1.ShootSpec{
										SeedName: ptr.To("some-different-seed"),
									},
								}

								Expect(fakeClient.Create(ctx, shoot)).To(Succeed())

								Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
									AdmissionResponse: admissionv1.AdmissionResponse{
										Allowed: false,
										Result: &metav1.Status{
											Code:    int32(http.StatusForbidden),
											Message: fmt.Sprintf("object does not belong to seed %q", seedName),
										},
									},
								}))
							})

							It("should allow because the related shoot does belong to gardenlet's seed", func() {
								shoot := &gardencorev1beta1.Shoot{
									ObjectMeta: metav1.ObjectMeta{
										Name:      name,
										Namespace: namespace,
									},
									Spec: gardencorev1beta1.ShootSpec{
										SeedName: &seedName,
									},
								}

								Expect(fakeClient.Create(ctx, shoot)).To(Succeed())

								Expect(handler.Handle(ctx, request)).To(Equal(responseAllowed))
							})
						})
					})

					Context("bootstrap token secret for managed seed", func() {
						var (
							secret      *corev1.Secret
							managedSeed *seedmanagementv1alpha1.ManagedSeed
							shoot       *gardencorev1beta1.Shoot

							managedSeedNamespace = "ms1ns"
							managedSeedName      = "ms1name"
							shootName            = "ms1shoot"
						)

						BeforeEach(func() {
							secret = &corev1.Secret{
								Type: corev1.SecretTypeBootstrapToken,
								Data: map[string][]byte{
									"usage-bootstrap-authentication": []byte("true"),
									"usage-bootstrap-signing":        []byte("true"),
									"description":                    []byte("A bootstrap token for the Gardenlet for seedmanagement.gardener.cloud/v1alpha1.ManagedSeed resource " + managedSeedNamespace + "/" + managedSeedName + "."),
								},
							}
							managedSeed = &seedmanagementv1alpha1.ManagedSeed{
								ObjectMeta: metav1.ObjectMeta{
									Name:      managedSeedName,
									Namespace: managedSeedNamespace,
								},
								Spec: seedmanagementv1alpha1.ManagedSeedSpec{
									Shoot: &seedmanagementv1alpha1.Shoot{
										Name: shootName,
									},
								},
							}
							shoot = &gardencorev1beta1.Shoot{
								ObjectMeta: metav1.ObjectMeta{
									Name:      shootName,
									Namespace: managedSeedNamespace,
								},
								Spec: gardencorev1beta1.ShootSpec{
									SeedName: &seedName,
								},
							}

							request.Name = "bootstrap-token-123456"
							request.Namespace = "kube-system"

							objData, err := runtime.Encode(encoder, secret)
							Expect(err).NotTo(HaveOccurred())
							request.Object.Raw = objData
						})

						It("should return an error if decoding the secret fails", func() {
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

						It("should return an error if the secret type is unexpected", func() {
							secret.Type = corev1.SecretTypeOpaque
							objData, err := runtime.Encode(encoder, secret)
							Expect(err).NotTo(HaveOccurred())
							request.Object.Raw = objData

							Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
								AdmissionResponse: admissionv1.AdmissionResponse{
									Allowed: false,
									Result: &metav1.Status{
										Code:    int32(http.StatusUnprocessableEntity),
										Message: fmt.Sprintf("unexpected secret type: %q", secret.Type),
									},
								},
							}))
						})

						It("should return an error if the usage-bootstrap-authentication field is unexpected", func() {
							secret.Data["usage-bootstrap-authentication"] = []byte("false")
							objData, err := runtime.Encode(encoder, secret)
							Expect(err).NotTo(HaveOccurred())
							request.Object.Raw = objData

							Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
								AdmissionResponse: admissionv1.AdmissionResponse{
									Allowed: false,
									Result: &metav1.Status{
										Code:    int32(http.StatusUnprocessableEntity),
										Message: "\"usage-bootstrap-authentication\" must be set to 'true'",
									},
								},
							}))
						})

						It("should return an error if the usage-bootstrap-signing field is unexpected", func() {
							secret.Data["usage-bootstrap-signing"] = []byte("false")
							objData, err := runtime.Encode(encoder, secret)
							Expect(err).NotTo(HaveOccurred())
							request.Object.Raw = objData

							Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
								AdmissionResponse: admissionv1.AdmissionResponse{
									Allowed: false,
									Result: &metav1.Status{
										Code:    int32(http.StatusUnprocessableEntity),
										Message: "\"usage-bootstrap-signing\" must be set to 'true'",
									},
								},
							}))
						})

						It("should return an error if the auth-extra-groups field is unexpected", func() {
							secret.Data["auth-extra-groups"] = []byte("foo")
							objData, err := runtime.Encode(encoder, secret)
							Expect(err).NotTo(HaveOccurred())
							request.Object.Raw = objData

							Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
								AdmissionResponse: admissionv1.AdmissionResponse{
									Allowed: false,
									Result: &metav1.Status{
										Code:    int32(http.StatusUnprocessableEntity),
										Message: "\"auth-extra-groups\" must not be set",
									},
								},
							}))
						})

						It("should forbid if the managedseed does not exist", func() {
							Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
								AdmissionResponse: admissionv1.AdmissionResponse{
									Allowed: false,
									Result: &metav1.Status{
										Code:    int32(http.StatusForbidden),
										Message: apierrors.NewNotFound(schema.GroupResource{Group: seedmanagementv1alpha1.SchemeGroupVersion.Group, Resource: "managedseeds"}, managedSeedName).Error(),
									},
								},
							}))
						})

						It("should return an error if reading the managedseed fails", func() {
							fakeClient := fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).WithInterceptorFuncs(interceptor.Funcs{
								Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
									if _, ok := obj.(*seedmanagementv1alpha1.ManagedSeed); ok {
										return fakeErr
									}
									return c.Get(ctx, key, obj, opts...)
								},
							}).Build()

							handler = &Handler{Logger: log, Client: fakeClient, Decoder: decoder}

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

						It("should return an error if reading the shoot fails", func() {
							Expect(fakeClient.Create(ctx, managedSeed)).To(Succeed())

							Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
								AdmissionResponse: admissionv1.AdmissionResponse{
									Allowed: false,
									Result: &metav1.Status{
										Code:    int32(http.StatusInternalServerError),
										Message: apierrors.NewNotFound(schema.GroupResource{Group: gardencorev1beta1.SchemeGroupVersion.Group, Resource: "shoots"}, shootName).Error(),
									},
								},
							}))
						})

						It("should return an error if the shoot does not belong to the gardenlet's seed", func() {
							shoot.Spec.SeedName = ptr.To("some-other-seed")

							Expect(fakeClient.Create(ctx, managedSeed)).To(Succeed())
							Expect(fakeClient.Create(ctx, shoot)).To(Succeed())

							Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
								AdmissionResponse: admissionv1.AdmissionResponse{
									Allowed: false,
									Result: &metav1.Status{
										Code:    int32(http.StatusForbidden),
										Message: fmt.Sprintf("object does not belong to seed %q", seedName),
									},
								},
							}))
						})

						It("should return an error if reading the seed fails", func() {
							fakeClient := fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).WithInterceptorFuncs(interceptor.Funcs{
								Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
									if _, ok := obj.(*gardencorev1beta1.Seed); ok {
										return fakeErr
									}
									return c.Get(ctx, key, obj, opts...)
								},
							}).Build()

							handler = &Handler{Logger: log, Client: fakeClient, Decoder: decoder}

							Expect(fakeClient.Create(ctx, managedSeed)).To(Succeed())
							Expect(fakeClient.Create(ctx, shoot)).To(Succeed())

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

						It("should forbid if the seed does exist already", func() {
							Expect(fakeClient.Create(ctx, managedSeed)).To(Succeed())
							Expect(fakeClient.Create(ctx, shoot)).To(Succeed())

							Expect(fakeClient.Create(ctx, &gardencorev1beta1.Seed{
								ObjectMeta: metav1.ObjectMeta{
									Name: managedSeedName,
								},
							})).To(Succeed())

							Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
								AdmissionResponse: admissionv1.AdmissionResponse{
									Allowed: false,
									Result: &metav1.Status{
										Code:    int32(http.StatusBadRequest),
										Message: "managed seed " + managedSeedNamespace + "/" + managedSeedName + " is already bootstrapped",
									},
								},
							}))
						})

						It("should allow if the seed does not yet exist", func() {
							Expect(fakeClient.Create(ctx, managedSeed)).To(Succeed())
							Expect(fakeClient.Create(ctx, shoot)).To(Succeed())

							Expect(handler.Handle(ctx, request)).To(Equal(responseAllowed))
						})

						It("should allow if the seed does exist but client cert is expired", func() {
							Expect(fakeClient.Create(ctx, managedSeed)).To(Succeed())
							Expect(fakeClient.Create(ctx, shoot)).To(Succeed())

							seed := &gardencorev1beta1.Seed{
								ObjectMeta: metav1.ObjectMeta{
									Name: managedSeedName,
								},
								Status: gardencorev1beta1.SeedStatus{
									ClientCertificateExpirationTimestamp: &metav1.Time{Time: time.Now().Add(-time.Hour)},
								},
							}
							Expect(fakeClient.Create(ctx, seed)).To(Succeed())

							Expect(handler.Handle(ctx, request)).To(Equal(responseAllowed))
						})

						It("should allow if the seed does exist but the managedseed is annotated with the renew-kubeconfig annotation", func() {
							managedSeed.Annotations = map[string]string{v1beta1constants.GardenerOperation: v1beta1constants.GardenerOperationRenewKubeconfig}
							Expect(fakeClient.Create(ctx, managedSeed)).To(Succeed())
							Expect(fakeClient.Create(ctx, shoot)).To(Succeed())

							seed := &gardencorev1beta1.Seed{
								ObjectMeta: metav1.ObjectMeta{
									Name: managedSeedName,
								},
							}
							Expect(fakeClient.Create(ctx, seed)).To(Succeed())

							Expect(handler.Handle(ctx, request)).To(Equal(responseAllowed))
						})
					})

					Context("bootstrap token secret for gardenlets", func() {
						var (
							secret    *corev1.Secret
							gardenlet *seedmanagementv1alpha1.Gardenlet

							gardenletNamespace = "glet1ns"
							gardenletName      = "glet1name"
						)

						BeforeEach(func() {
							secret = &corev1.Secret{
								Type: corev1.SecretTypeBootstrapToken,
								Data: map[string][]byte{
									"usage-bootstrap-authentication": []byte("true"),
									"usage-bootstrap-signing":        []byte("true"),
									"description":                    []byte("A bootstrap token for the Gardenlet for seedmanagement.gardener.cloud/v1alpha1.Gardenlet resource " + gardenletNamespace + "/" + gardenletName + "."),
								},
							}
							gardenlet = &seedmanagementv1alpha1.Gardenlet{
								ObjectMeta: metav1.ObjectMeta{
									Name:      gardenletName,
									Namespace: gardenletNamespace,
								},
							}

							request.Name = "bootstrap-token-123456"
							request.Namespace = "kube-system"

							objData, err := runtime.Encode(encoder, secret)
							Expect(err).NotTo(HaveOccurred())
							request.Object.Raw = objData
						})

						It("should return an error if decoding the secret fails", func() {
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

						It("should return an error if the secret type is unexpected", func() {
							secret.Type = corev1.SecretTypeOpaque
							objData, err := runtime.Encode(encoder, secret)
							Expect(err).NotTo(HaveOccurred())
							request.Object.Raw = objData

							Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
								AdmissionResponse: admissionv1.AdmissionResponse{
									Allowed: false,
									Result: &metav1.Status{
										Code:    int32(http.StatusUnprocessableEntity),
										Message: fmt.Sprintf("unexpected secret type: %q", secret.Type),
									},
								},
							}))
						})

						It("should return an error if the usage-bootstrap-authentication field is unexpected", func() {
							secret.Data["usage-bootstrap-authentication"] = []byte("false")
							objData, err := runtime.Encode(encoder, secret)
							Expect(err).NotTo(HaveOccurred())
							request.Object.Raw = objData

							Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
								AdmissionResponse: admissionv1.AdmissionResponse{
									Allowed: false,
									Result: &metav1.Status{
										Code:    int32(http.StatusUnprocessableEntity),
										Message: "\"usage-bootstrap-authentication\" must be set to 'true'",
									},
								},
							}))
						})

						It("should return an error if the usage-bootstrap-signing field is unexpected", func() {
							secret.Data["usage-bootstrap-signing"] = []byte("false")
							objData, err := runtime.Encode(encoder, secret)
							Expect(err).NotTo(HaveOccurred())
							request.Object.Raw = objData

							Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
								AdmissionResponse: admissionv1.AdmissionResponse{
									Allowed: false,
									Result: &metav1.Status{
										Code:    int32(http.StatusUnprocessableEntity),
										Message: "\"usage-bootstrap-signing\" must be set to 'true'",
									},
								},
							}))
						})

						It("should return an error if the auth-extra-groups field is unexpected", func() {
							secret.Data["auth-extra-groups"] = []byte("foo")
							objData, err := runtime.Encode(encoder, secret)
							Expect(err).NotTo(HaveOccurred())
							request.Object.Raw = objData

							Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
								AdmissionResponse: admissionv1.AdmissionResponse{
									Allowed: false,
									Result: &metav1.Status{
										Code:    int32(http.StatusUnprocessableEntity),
										Message: "\"auth-extra-groups\" must not be set",
									},
								},
							}))
						})

						It("should forbid if the Gardenlet does not exist", func() {
							Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
								AdmissionResponse: admissionv1.AdmissionResponse{
									Allowed: false,
									Result: &metav1.Status{
										Code:    int32(http.StatusForbidden),
										Message: apierrors.NewNotFound(schema.GroupResource{Group: seedmanagementv1alpha1.SchemeGroupVersion.Group, Resource: "gardenlets"}, gardenletName).Error(),
									},
								},
							}))
						})

						It("should return an error if reading the Gardenlet fails", func() {
							fakeClient := fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).WithInterceptorFuncs(interceptor.Funcs{
								Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
									if _, ok := obj.(*seedmanagementv1alpha1.Gardenlet); ok {
										return fakeErr
									}
									return c.Get(ctx, key, obj, opts...)
								},
							}).Build()

							handler = &Handler{Logger: log, Client: fakeClient, Decoder: decoder}

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

						It("should return an error if reading the seed fails", func() {
							fakeClient := fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).WithInterceptorFuncs(interceptor.Funcs{
								Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
									if _, ok := obj.(*gardencorev1beta1.Seed); ok {
										return fakeErr
									}
									return c.Get(ctx, key, obj, opts...)
								},
							}).Build()

							handler = &Handler{Logger: log, Client: fakeClient, Decoder: decoder}

							Expect(fakeClient.Create(ctx, gardenlet)).To(Succeed())

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

						It("should forbid if the seed does exist already", func() {
							Expect(fakeClient.Create(ctx, gardenlet)).To(Succeed())
							Expect(fakeClient.Create(ctx, &gardencorev1beta1.Seed{
								ObjectMeta: metav1.ObjectMeta{
									Name: gardenletName,
								},
							})).To(Succeed())

							Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
								AdmissionResponse: admissionv1.AdmissionResponse{
									Allowed: false,
									Result: &metav1.Status{
										Code:    int32(http.StatusBadRequest),
										Message: "gardenlet " + gardenletNamespace + "/" + gardenletName + " is already bootstrapped",
									},
								},
							}))
						})

						It("should forbid if the seed does not yet exist", func() {
							Expect(fakeClient.Create(ctx, gardenlet)).To(Succeed())

							Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
								AdmissionResponse: admissionv1.AdmissionResponse{
									Allowed: false,
									Result: &metav1.Status{
										Code:    int32(http.StatusForbidden),
										Message: apierrors.NewNotFound(schema.GroupResource{Group: gardencorev1beta1.SchemeGroupVersion.Group, Resource: "seeds"}, gardenletName).Error(),
									},
								},
							}))
						})

						It("should allow if the seed does exist but client cert is expired", func() {
							Expect(fakeClient.Create(ctx, gardenlet)).To(Succeed())
							Expect(fakeClient.Create(ctx, &gardencorev1beta1.Seed{
								ObjectMeta: metav1.ObjectMeta{
									Name: gardenletName,
								},
								Status: gardencorev1beta1.SeedStatus{
									ClientCertificateExpirationTimestamp: &metav1.Time{Time: time.Now().Add(-time.Hour)},
								},
							})).To(Succeed())

							Expect(handler.Handle(ctx, request)).To(Equal(responseAllowed))
						})

						It("should allow if the seed does exist but the gardenlet is annotated with the renew-kubeconfig annotation", func() {
							gardenlet.Annotations = map[string]string{v1beta1constants.GardenerOperation: v1beta1constants.GardenerOperationRenewKubeconfig}

							Expect(fakeClient.Create(ctx, gardenlet)).To(Succeed())
							Expect(fakeClient.Create(ctx, &gardencorev1beta1.Seed{
								ObjectMeta: metav1.ObjectMeta{
									Name: gardenletName,
								},
							})).To(Succeed())

							Expect(handler.Handle(ctx, request)).To(Equal(responseAllowed))
						})
					})

					Context("managed seed secret", func() {
						var (
							managedSeed1Namespace    string
							shoot1, shoot2           *gardencorev1beta1.Shoot
							seedConfig1, seedConfig2 *gardenletconfigv1alpha1.SeedConfig
							managedSeeds             []seedmanagementv1alpha1.ManagedSeed
						)

						BeforeEach(func() {
							managedSeed1Namespace = "ns1"
							shoot1 = &gardencorev1beta1.Shoot{
								ObjectMeta: metav1.ObjectMeta{
									Namespace: managedSeed1Namespace,
									Name:      "shoot1",
								},
								Spec: gardencorev1beta1.ShootSpec{SeedName: ptr.To("some-other-seed-name")},
							}
							shoot2 = &gardencorev1beta1.Shoot{
								ObjectMeta: metav1.ObjectMeta{
									Namespace: managedSeed1Namespace,
									Name:      "shoot2",
								},
								Spec: gardencorev1beta1.ShootSpec{SeedName: &seedName},
							}
							seedConfig1 = &gardenletconfigv1alpha1.SeedConfig{
								SeedTemplate: gardencorev1beta1.SeedTemplate{},
							}
							seedConfig2 = &gardenletconfigv1alpha1.SeedConfig{
								SeedTemplate: gardencorev1beta1.SeedTemplate{},
							}
							managedSeeds = []seedmanagementv1alpha1.ManagedSeed{
								{
									ObjectMeta: metav1.ObjectMeta{Name: shoot1.Name, Namespace: managedSeed1Namespace},
									Spec: seedmanagementv1alpha1.ManagedSeedSpec{
										Shoot: &seedmanagementv1alpha1.Shoot{Name: shoot1.Name},
										Gardenlet: seedmanagementv1alpha1.GardenletConfig{
											Config: runtime.RawExtension{
												Object: &gardenletconfigv1alpha1.GardenletConfiguration{
													SeedConfig: seedConfig1,
												},
											},
										},
									},
								},
								{
									ObjectMeta: metav1.ObjectMeta{Name: shoot2.Name, Namespace: managedSeed1Namespace},
									Spec: seedmanagementv1alpha1.ManagedSeedSpec{
										Shoot: &seedmanagementv1alpha1.Shoot{Name: shoot2.Name},
										Gardenlet: seedmanagementv1alpha1.GardenletConfig{
											Config: runtime.RawExtension{
												Object: &gardenletconfigv1alpha1.GardenletConfiguration{
													SeedConfig: seedConfig2,
												},
											},
										},
									},
								},
							}
						})

						It("should return an error because listing managed seeds failed", func() {
							fakeClient := fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).WithInterceptorFuncs(interceptor.Funcs{
								List: func(_ context.Context, _ client.WithWatch, _ client.ObjectList, _ ...client.ListOption) error {
									return fakeErr
								},
							}).Build()

							handler = &Handler{Logger: log, Client: fakeClient, Decoder: decoder}

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

						It("should return an error because reading a shoot failed", func() {
							for _, ms := range managedSeeds {
								Expect(fakeClient.Create(ctx, &ms)).To(Succeed())
							}

							Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
								AdmissionResponse: admissionv1.AdmissionResponse{
									Allowed: false,
									Result: &metav1.Status{
										Code:    int32(http.StatusInternalServerError),
										Message: apierrors.NewNotFound(schema.GroupResource{Group: gardencorev1beta1.SchemeGroupVersion.Group, Resource: "shoots"}, shoot1.Name).Error(),
									},
								},
							}))
						})

						It("should return an error because extracting the seed template failed", func() {
							managedSeeds[1].Spec.Gardenlet = seedmanagementv1alpha1.GardenletConfig{
								Config: runtime.RawExtension{
									Object: &gardenletconfigv1alpha1.GardenletConfiguration{
										SeedConfig: nil,
									},
								},
							}

							for _, ms := range managedSeeds {
								Expect(fakeClient.Create(ctx, &ms)).To(Succeed())
							}

							Expect(fakeClient.Create(ctx, shoot1)).To(Succeed())
							Expect(fakeClient.Create(ctx, shoot2)).To(Succeed())

							Expect(handler.Handle(ctx, request)).To(Or(
								Equal(admission.Response{
									AdmissionResponse: admissionv1.AdmissionResponse{
										Allowed: false,
										Result: &metav1.Status{
											Code:    int32(http.StatusInternalServerError),
											Message: `seed template is nil for ManagedSeed "shoot1"`,
										},
									},
								}),
								Equal(admission.Response{
									AdmissionResponse: admissionv1.AdmissionResponse{
										Allowed: false,
										Result: &metav1.Status{
											Code:    int32(http.StatusInternalServerError),
											Message: `seed template is nil for ManagedSeed "shoot2"`,
										},
									},
								}),
							))
						})

						It("should forbid because the secret is referenced in a managedseed's gardenlet config but belongs to another seed", func() {
							var (
								secretName      = "secret-bar"
								secretNamespace = "secret-foo"
							)

							request.Namespace = secretNamespace
							request.Name = secretName
							seedConfig1.Spec.Backup = &gardencorev1beta1.Backup{
								CredentialsRef: &corev1.ObjectReference{
									APIVersion: "v1",
									Kind:       "Secret",
									Name:       secretName,
									Namespace:  secretNamespace,
								},
							}

							for _, ms := range managedSeeds {
								Expect(fakeClient.Create(ctx, &ms)).To(Succeed())
							}

							Expect(fakeClient.Create(ctx, shoot1)).To(Succeed())
							Expect(fakeClient.Create(ctx, shoot2)).To(Succeed())

							Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
								AdmissionResponse: admissionv1.AdmissionResponse{
									Allowed: false,
									Result: &metav1.Status{
										Code:    int32(http.StatusForbidden),
										Message: fmt.Sprintf("object does not belong to seed %q", seedName),
									},
								},
							}))
						})

						It("should allow because the secret is referenced in a managedseed's gardenlet config", func() {
							var (
								secretName      = "secret-bar"
								secretNamespace = "secret-foo"
							)

							request.Namespace = secretNamespace
							request.Name = secretName
							seedConfig2.Spec.Backup = &gardencorev1beta1.Backup{
								CredentialsRef: &corev1.ObjectReference{
									APIVersion: "v1",
									Kind:       "Secret",
									Name:       secretName,
									Namespace:  secretNamespace,
								},
							}

							for _, ms := range managedSeeds {
								Expect(fakeClient.Create(ctx, &ms)).To(Succeed())
							}

							Expect(fakeClient.Create(ctx, shoot1)).To(Succeed())
							Expect(fakeClient.Create(ctx, shoot2)).To(Succeed())

							Expect(handler.Handle(ctx, request)).To(Equal(responseAllowed))
						})
					})
				})
			})

			Context("when requested for InternalSecrets", func() {
				var name, namespace string

				BeforeEach(func() {
					name, namespace = "foo", "bar"

					request.Name = name
					request.Namespace = namespace
					request.UserInfo = seedUser
					request.Resource = metav1.GroupVersionResource{
						Group:    gardencorev1beta1.SchemeGroupVersion.Group,
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

				Context("when operation is create", func() {
					BeforeEach(func() {
						request.Operation = admissionv1.Create
					})

					It("should forbid the request because it's no expected internal secret", func() {
						Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
							AdmissionResponse: admissionv1.AdmissionResponse{
								Allowed: false,
								Result: &metav1.Status{
									Code:    int32(http.StatusForbidden),
									Message: fmt.Sprintf("object does not belong to seed %q", seedName),
								},
							},
						}))
					})

					Context("shoot-related project secret", func() {
						testSuite := func(suffix string) {
							BeforeEach(func() {
								request.Name = name + suffix
							})

							It("should return an error because the related shoot was not found", func() {
								Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
									AdmissionResponse: admissionv1.AdmissionResponse{
										Allowed: false,
										Result: &metav1.Status{
											Code:    int32(http.StatusForbidden),
											Message: apierrors.NewNotFound(schema.GroupResource{Group: gardencorev1beta1.SchemeGroupVersion.Group, Resource: "shoots"}, name).Error(),
										},
									},
								}))
							})

							It("should return an error because the related shoot could not be read", func() {
								fakeClient := fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).WithInterceptorFuncs(interceptor.Funcs{
									Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
										if _, ok := obj.(*gardencorev1beta1.Shoot); ok {
											return fakeErr
										}
										return c.Get(ctx, key, obj, opts...)
									},
								}).Build()

								handler = &Handler{Logger: log, Client: fakeClient, Decoder: decoder}

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

							It("should forbid because the related shoot does not belong to gardenlet's seed", func() {
								shoot := &gardencorev1beta1.Shoot{
									ObjectMeta: metav1.ObjectMeta{
										Name:      name,
										Namespace: namespace,
									},
									Spec: gardencorev1beta1.ShootSpec{SeedName: ptr.To("some-different-seed")},
								}

								Expect(fakeClient.Create(ctx, shoot)).To(Succeed())

								Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
									AdmissionResponse: admissionv1.AdmissionResponse{
										Allowed: false,
										Result: &metav1.Status{
											Code:    int32(http.StatusForbidden),
											Message: fmt.Sprintf("object does not belong to seed %q", seedName),
										},
									},
								}))
							})

							It("should allow because the related shoot does belong to gardenlet's seed", func() {
								shoot := &gardencorev1beta1.Shoot{
									ObjectMeta: metav1.ObjectMeta{
										Name:      name,
										Namespace: namespace,
									},
									Spec: gardencorev1beta1.ShootSpec{SeedName: &seedName},
								}

								Expect(fakeClient.Create(ctx, shoot)).To(Succeed())

								Expect(handler.Handle(ctx, request)).To(Equal(responseAllowed))
							})
						}

						Describe("ca-client suffix", func() { testSuite(".ca-client") })
					})
				})
			})

			Context("when requested for ConfigMaps", func() {
				var name, namespace string

				BeforeEach(func() {
					name, namespace = "foo", "bar"

					request.Name = name
					request.Namespace = namespace
					request.UserInfo = seedUser
					request.Resource = metav1.GroupVersionResource{
						Group:    corev1.SchemeGroupVersion.Group,
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

				Context("when operation is create", func() {
					BeforeEach(func() {
						request.Operation = admissionv1.Create
					})

					It("should forbid the request because it's no expected config map", func() {
						Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
							AdmissionResponse: admissionv1.AdmissionResponse{
								Allowed: false,
								Result: &metav1.Status{
									Code:    int32(http.StatusForbidden),
									Message: fmt.Sprintf("object does not belong to seed %q", seedName),
								},
							},
						}))
					})

					Context("shoot-related project config map", func() {
						testSuite := func(suffix string) {
							BeforeEach(func() {
								request.Name = name + suffix
							})

							It("should return an error because the related shoot was not found", func() {
								Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
									AdmissionResponse: admissionv1.AdmissionResponse{
										Allowed: false,
										Result: &metav1.Status{
											Code:    int32(http.StatusForbidden),
											Message: apierrors.NewNotFound(schema.GroupResource{Group: gardencorev1beta1.SchemeGroupVersion.Group, Resource: "shoots"}, name).Error(),
										},
									},
								}))
							})

							It("should return an error because the related shoot could not be read", func() {
								fakeClient := fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).WithInterceptorFuncs(interceptor.Funcs{
									Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
										if _, ok := obj.(*gardencorev1beta1.Shoot); ok {
											return fakeErr
										}
										return c.Get(ctx, key, obj, opts...)
									},
								}).Build()

								handler = &Handler{Logger: log, Client: fakeClient, Decoder: decoder}

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

							It("should forbid because the related shoot does not belong to gardenlet's seed", func() {
								shoot := &gardencorev1beta1.Shoot{
									ObjectMeta: metav1.ObjectMeta{
										Name:      name,
										Namespace: namespace,
									},
									Spec: gardencorev1beta1.ShootSpec{SeedName: ptr.To("some-different-seed")},
								}

								Expect(fakeClient.Create(ctx, shoot)).To(Succeed())

								Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
									AdmissionResponse: admissionv1.AdmissionResponse{
										Allowed: false,
										Result: &metav1.Status{
											Code:    int32(http.StatusForbidden),
											Message: fmt.Sprintf("object does not belong to seed %q", seedName),
										},
									},
								}))
							})

							It("should allow because the related shoot does belong to gardenlet's seed", func() {
								shoot := &gardencorev1beta1.Shoot{
									ObjectMeta: metav1.ObjectMeta{
										Name:      name,
										Namespace: namespace,
									},
									Spec: gardencorev1beta1.ShootSpec{SeedName: &seedName},
								}

								Expect(fakeClient.Create(ctx, shoot)).To(Succeed())

								Expect(handler.Handle(ctx, request)).To(Equal(responseAllowed))
							})
						}

						Describe("ca-cluster suffix", func() { testSuite(".ca-cluster") })
					})
				})
			})
		}

		Context("gardenlet client", func() {
			BeforeEach(func() {
				seedUser = gardenletUser
			})

			testCommonAccess()

			Context("when requested for CertificateSigningRequests", func() {
				var name string

				BeforeEach(func() {
					name = "foo"

					request.Name = name
					request.UserInfo = seedUser
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

					It("should forbid the request because the CSR is not a valid seed-related CSR", func() {
						objData, err := runtime.Encode(encoder, &certificatesv1.CertificateSigningRequest{
							TypeMeta: metav1.TypeMeta{
								APIVersion: certificatesv1.SchemeGroupVersion.String(),
								Kind:       "CertificateSigningRequest",
							},
							Spec: certificatesv1.CertificateSigningRequestSpec{
								Request: []byte(`-----BEGIN CERTIFICATE REQUEST-----
MIIClzCCAX8CAQAwUjEkMCIGA1UEChMbZ2FyZGVuZXIuY2xvdWQ6c3lzdGVtOnNl
ZWRzMSowKAYDVQQDEyFnYXJkZW5lci5jbG91ZDpzeXN0ZW06c2VlZDpteXNlZWQw
ggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQCzNgJWhogJrCSzAhKKmHkJ
FuooKAbxpWRGDOe5DiB8jPdgCoRCkZYnF7D9x9cDzliljA9IeBad3P3E9oegtSV/
sXFJYqb+lRuhJQ5oo2eBC6WRg+Oxglp+n7o7xt0bO7JHS977mqNrqsJ1d1FnJHTB
MPHPxqoqkgIbdW4t219ckSA20aWzC3PU7I7+Z9OD+YfuuYgzkWG541XyBBKVSD2w
Ix2yGu6zrslqZ1eVBZ4IoxpWrQNmLSMFQVnABThyEUi0U1eVtW0vPNwSnBf0mufX
Z0PpqAIPVjr64Z4s3HHml2GSu64iOxaG5wwb9qIPcdyFaQCep/sFh7kq1KjNI1Ql
AgMBAAGgADANBgkqhkiG9w0BAQsFAAOCAQEAb+meLvm7dgHpzhu0XQ39w41FgpTv
S7p78ABFwzDNcP1NwfrEUft0T/rUwPiMlN9zve2rRicaZX5Z7Bol/newejsu8H5z
OdotvtKjE7zBCMzwnXZwO/0pA0cuUFcAy50DPcr35gdGjGlzV9ogO+HPKPTieS3n
TRVg+MWlcLqCjALr9Y4N39DOzf4/SJts8AZJJ+lyyxnY3XIPXx7SdADwNWC8BX0U
OK8CwMwN3iiBQ4redVeMK7LU1unV899q/PWB+NXFcKVr+Grm/Kom5VxuhXSzcHEp
yO57qEcJqG1cB7iSchFuCSTuDBbZlN0fXgn4YjiWZyb4l3BDp3rm4iJImA==
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
									Message: "can only create CSRs for seed clusters: key usages are not set to [key encipherment digital signature client auth]",
								},
							},
						}))
					})

					It("should forbid the request because the seed name of the csr does not match", func() {
						objData, err := runtime.Encode(encoder, &certificatesv1.CertificateSigningRequest{
							TypeMeta: metav1.TypeMeta{
								APIVersion: certificatesv1.SchemeGroupVersion.String(),
								Kind:       "CertificateSigningRequest",
							},
							Spec: certificatesv1.CertificateSigningRequestSpec{
								Request: []byte(`-----BEGIN CERTIFICATE REQUEST-----
MIIClzCCAX8CAQAwUjEkMCIGA1UEChMbZ2FyZGVuZXIuY2xvdWQ6c3lzdGVtOnNl
ZWRzMSowKAYDVQQDEyFnYXJkZW5lci5jbG91ZDpzeXN0ZW06c2VlZDpteXNlZWQw
ggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQCzNgJWhogJrCSzAhKKmHkJ
FuooKAbxpWRGDOe5DiB8jPdgCoRCkZYnF7D9x9cDzliljA9IeBad3P3E9oegtSV/
sXFJYqb+lRuhJQ5oo2eBC6WRg+Oxglp+n7o7xt0bO7JHS977mqNrqsJ1d1FnJHTB
MPHPxqoqkgIbdW4t219ckSA20aWzC3PU7I7+Z9OD+YfuuYgzkWG541XyBBKVSD2w
Ix2yGu6zrslqZ1eVBZ4IoxpWrQNmLSMFQVnABThyEUi0U1eVtW0vPNwSnBf0mufX
Z0PpqAIPVjr64Z4s3HHml2GSu64iOxaG5wwb9qIPcdyFaQCep/sFh7kq1KjNI1Ql
AgMBAAGgADANBgkqhkiG9w0BAQsFAAOCAQEAb+meLvm7dgHpzhu0XQ39w41FgpTv
S7p78ABFwzDNcP1NwfrEUft0T/rUwPiMlN9zve2rRicaZX5Z7Bol/newejsu8H5z
OdotvtKjE7zBCMzwnXZwO/0pA0cuUFcAy50DPcr35gdGjGlzV9ogO+HPKPTieS3n
TRVg+MWlcLqCjALr9Y4N39DOzf4/SJts8AZJJ+lyyxnY3XIPXx7SdADwNWC8BX0U
OK8CwMwN3iiBQ4redVeMK7LU1unV899q/PWB+NXFcKVr+Grm/Kom5VxuhXSzcHEp
yO57qEcJqG1cB7iSchFuCSTuDBbZlN0fXgn4YjiWZyb4l3BDp3rm4iJImA==
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

						Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
							AdmissionResponse: admissionv1.AdmissionResponse{
								Allowed: false,
								Result: &metav1.Status{
									Code:    int32(http.StatusForbidden),
									Message: fmt.Sprintf("object does not belong to seed %q", seedName),
								},
							},
						}))
					})

					It("should allow the request because seed name matches", func() {
						objData, err := runtime.Encode(encoder, &certificatesv1.CertificateSigningRequest{
							TypeMeta: metav1.TypeMeta{
								APIVersion: certificatesv1.SchemeGroupVersion.String(),
								Kind:       "CertificateSigningRequest",
							},
							Spec: certificatesv1.CertificateSigningRequestSpec{
								Request: []byte(`-----BEGIN CERTIFICATE REQUEST-----
MIIClTCCAX0CAQAwUDEkMCIGA1UEChMbZ2FyZGVuZXIuY2xvdWQ6c3lzdGVtOnNl
ZWRzMSgwJgYDVQQDEx9nYXJkZW5lci5jbG91ZDpzeXN0ZW06c2VlZDpzZWVkMIIB
IjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAsDqibMtE5PXULTT12u0TYW1U
EI2f2MFImNPdEdmyTO8kjy61JzBQxUz6NLLmZWks7dnhZOrhfXqJjVzLWi7gAAIH
hkoxnu8spKTV53l6eY5RrivVsNFRuPF763bKd6JvsF1p9QD9y8uk6bY4NbLAjgMJ
MH64Sj398AnvLlIL+8XIFKtT/SjvOp99oGkKxWHBvokcz9MLUJc/2/JcOdsZ62ue
ZAsqimh0F085+BoG2YtLa4kLNAAiNsijgJ5QCXc7/F8uqkj4uy436LGgGmDfcQxC
9W2snEqriv1dsjF5R/kjh+UbTd+ZdHoAaNaiE7lfZcwe/ap6SNeZaszcDoR//wID
AQABoAAwDQYJKoZIhvcNAQELBQADggEBAKGWWWDHGHdUkOvE1L+tR/v3sDvLfmO7
jWtF/Sq7kRCrr6xEHLKmVA4wRovpzOML0ntrDCu3npKAWqN+U56L1ZeZSsxyOhvN
dXjk2wPg0+IXPscd33hq0wGZRtBc5MHNWwYLv3ERKnHNbPE2ifkYy6FQ/h/2Kx55
tHu5PlIwWS6CP+03s3/gjbHX7VL+V3RF5BIHDWcp9QfjN0zEx0R2WVXKIbhC8RTR
BkEao/FEz4eQuV5atSD0S78+aF4BriEtWKKjXECTCxMuqcA24vGOgHIrEbKd7zSC
2L4LgmHdCmMFOtPkykwLK6wV1YW7Ce8AxU3j+q4kgZQ+51HJDQDdB74=
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

			Context("when requested for ClusterRoleBindings", func() {
				var name string

				BeforeEach(func() {
					name = "foo"

					request.Name = name
					request.UserInfo = seedUser
					request.Resource = metav1.GroupVersionResource{
						Group:    rbacv1.SchemeGroupVersion.Group,
						Resource: "clusterrolebindings",
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

					It("should forbid the request because name pattern does not match", func() {
						Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
							AdmissionResponse: admissionv1.AdmissionResponse{
								Allowed: false,
								Result: &metav1.Status{
									Code:    int32(http.StatusForbidden),
									Message: fmt.Sprintf("object does not belong to seed %q", seedName),
								},
							},
						}))
					})

					Context("name pattern matches", func() {
						var (
							managedSeed *seedmanagementv1alpha1.ManagedSeed
							shoot       *gardencorev1beta1.Shoot

							managedSeedNamespace = "ms1ns"
							managedSeedName      = "ms1name"
							shootName            = "ms1shoot"
						)

						BeforeEach(func() {
							managedSeed = &seedmanagementv1alpha1.ManagedSeed{
								ObjectMeta: metav1.ObjectMeta{
									Name:      managedSeedName,
									Namespace: managedSeedNamespace,
								},
								Spec: seedmanagementv1alpha1.ManagedSeedSpec{
									Shoot: &seedmanagementv1alpha1.Shoot{
										Name: shootName,
									},
								},
							}
							shoot = &gardencorev1beta1.Shoot{
								ObjectMeta: metav1.ObjectMeta{
									Name:      shootName,
									Namespace: managedSeedNamespace,
								},
								Spec: gardencorev1beta1.ShootSpec{
									SeedName: &seedName,
								},
							}

							request.Name = "gardener.cloud:system:seed-bootstrapper:" + managedSeedNamespace + ":" + managedSeedName
						})

						It("should forbid if decoding the object fails", func() {
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

						It("should forbid if the role ref doesn't match expectations", func() {
							objData, err := runtime.Encode(encoder, &rbacv1.ClusterRoleBinding{
								RoleRef: rbacv1.RoleRef{
									Name: "cluster-admin",
								},
							})
							Expect(err).NotTo(HaveOccurred())
							request.Object.Raw = objData

							Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
								AdmissionResponse: admissionv1.AdmissionResponse{
									Allowed: false,
									Result: &metav1.Status{
										Code:    int32(http.StatusForbidden),
										Message: "can only bindings referring to the bootstrapper role",
									},
								},
							}))
						})

						Context("when role ref is expected", func() {
							BeforeEach(func() {
								objData, err := runtime.Encode(encoder, &rbacv1.ClusterRoleBinding{
									RoleRef: rbacv1.RoleRef{
										APIGroup: "rbac.authorization.k8s.io",
										Kind:     "ClusterRole",
										Name:     "gardener.cloud:system:seed-bootstrapper",
									},
								})
								Expect(err).NotTo(HaveOccurred())
								request.Object.Raw = objData
							})

							It("should forbid if the managedseed does not exist", func() {
								Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
									AdmissionResponse: admissionv1.AdmissionResponse{
										Allowed: false,
										Result: &metav1.Status{
											Code:    int32(http.StatusForbidden),
											Message: apierrors.NewNotFound(schema.GroupResource{Group: seedmanagementv1alpha1.SchemeGroupVersion.Group, Resource: "managedseeds"}, managedSeedName).Error(),
										},
									},
								}))
							})

							It("should return an error if reading the managedseed fails", func() {
								fakeClient := fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).WithInterceptorFuncs(interceptor.Funcs{
									Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
										if _, ok := obj.(*seedmanagementv1alpha1.ManagedSeed); ok {
											return fakeErr
										}
										return c.Get(ctx, key, obj, opts...)
									},
								}).Build()

								handler = &Handler{Logger: log, Client: fakeClient, Decoder: decoder}

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

							It("should return an error if reading the shoot fails", func() {
								fakeClient := fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).WithInterceptorFuncs(interceptor.Funcs{
									Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
										if _, ok := obj.(*gardencorev1beta1.Shoot); ok {
											return fakeErr
										}
										return c.Get(ctx, key, obj, opts...)
									},
								}).Build()

								handler = &Handler{Logger: log, Client: fakeClient, Decoder: decoder}

								Expect(fakeClient.Create(ctx, managedSeed)).To(Succeed())

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

							It("should return an error if the shoot does not belong to the gardenlet's seed", func() {
								shoot.Spec.SeedName = ptr.To("some-other-seed")

								Expect(fakeClient.Create(ctx, managedSeed)).To(Succeed())
								Expect(fakeClient.Create(ctx, shoot)).To(Succeed())

								Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
									AdmissionResponse: admissionv1.AdmissionResponse{
										Allowed: false,
										Result: &metav1.Status{
											Code:    int32(http.StatusForbidden),
											Message: fmt.Sprintf("object does not belong to seed %q", seedName),
										},
									},
								}))
							})

							It("should return an error if reading the seed fails", func() {
								fakeClient := fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).WithInterceptorFuncs(interceptor.Funcs{
									Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
										if _, ok := obj.(*gardencorev1beta1.Seed); ok {
											return fakeErr
										}
										return c.Get(ctx, key, obj, opts...)
									},
								}).Build()

								handler = &Handler{Logger: log, Client: fakeClient, Decoder: decoder}

								Expect(fakeClient.Create(ctx, managedSeed)).To(Succeed())
								Expect(fakeClient.Create(ctx, shoot)).To(Succeed())

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

							It("should forbid if the seed does exist already", func() {
								Expect(fakeClient.Create(ctx, managedSeed)).To(Succeed())
								Expect(fakeClient.Create(ctx, shoot)).To(Succeed())

								seed := &gardencorev1beta1.Seed{
									ObjectMeta: metav1.ObjectMeta{
										Name: managedSeedName,
									},
								}
								Expect(fakeClient.Create(ctx, seed)).To(Succeed())

								Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
									AdmissionResponse: admissionv1.AdmissionResponse{
										Allowed: false,
										Result: &metav1.Status{
											Code:    int32(http.StatusBadRequest),
											Message: "managed seed " + managedSeedNamespace + "/" + managedSeedName + " is already bootstrapped",
										},
									},
								}))
							})

							It("should allow if the seed does not yet exist", func() {
								Expect(fakeClient.Create(ctx, managedSeed)).To(Succeed())
								Expect(fakeClient.Create(ctx, shoot)).To(Succeed())

								Expect(handler.Handle(ctx, request)).To(Equal(responseAllowed))
							})

							It("should allow if the seed does exist but client cert is expired", func() {
								Expect(fakeClient.Create(ctx, managedSeed)).To(Succeed())
								Expect(fakeClient.Create(ctx, shoot)).To(Succeed())

								seed := &gardencorev1beta1.Seed{
									ObjectMeta: metav1.ObjectMeta{
										Name: managedSeedName,
									},
									Status: gardencorev1beta1.SeedStatus{
										ClientCertificateExpirationTimestamp: &metav1.Time{Time: time.Now().Add(-time.Hour)},
									},
								}
								Expect(fakeClient.Create(ctx, seed)).To(Succeed())

								Expect(handler.Handle(ctx, request)).To(Equal(responseAllowed))
							})
						})
					})
				})
			})

			Context("when requested for Gardenlets", func() {
				var name, namespace string

				BeforeEach(func() {
					name, namespace = seedName, "garden"

					request.Name = name
					request.Namespace = namespace
					request.UserInfo = seedUser
					request.Resource = metav1.GroupVersionResource{
						Group:    seedmanagementv1alpha1.SchemeGroupVersion.Group,
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

					It("should forbid the request because it does not belong to the seed", func() {
						request.Name = "some-other-name"
						Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
							AdmissionResponse: admissionv1.AdmissionResponse{
								Allowed: false,
								Result: &metav1.Status{
									Code:    int32(http.StatusForbidden),
									Message: fmt.Sprintf("object does not belong to seed %q", seedName),
								},
							},
						}))
					})

					It("should forbid the request because the namespace is not 'garden'", func() {
						request.Namespace = "bar"
						Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
							AdmissionResponse: admissionv1.AdmissionResponse{
								Allowed: false,
								Result: &metav1.Status{
									Code:    int32(http.StatusBadRequest),
									Message: `object must be in namespace: "garden"`,
								},
							},
						}))
					})

					It("should allow because it belongs to the seed", func() {
						Expect(handler.Handle(ctx, request)).To(Equal(responseAllowed))
					})
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
						request.Namespace = "gardener-system-seed-lease"

						Expect(handler.Handle(ctx, request)).To(Equal(responseAllowed))
					})
				})
			})

			Context("when requested for ServiceAccounts", func() {
				var name, namespace string

				BeforeEach(func() {
					name, namespace = "foo", "bar"

					request.Name = name
					request.Name = namespace
					request.UserInfo = seedUser
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

				Context("when operation is create", func() {
					BeforeEach(func() {
						request.Operation = admissionv1.Create
					})

					It("should allow the request because namespace is seed namespace", func() {
						request.Namespace = "seed-" + seedName
						Expect(handler.Handle(ctx, request)).To(Equal(responseAllowed))
					})

					It("should forbid the request because name pattern does not match", func() {
						Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
							AdmissionResponse: admissionv1.AdmissionResponse{
								Allowed: false,
								Result: &metav1.Status{
									Code:    int32(http.StatusForbidden),
									Message: fmt.Sprintf("object does not belong to seed %q", seedName),
								},
							},
						}))
					})

					Context("name pattern matches", func() {
						var (
							managedSeed *seedmanagementv1alpha1.ManagedSeed
							shoot       *gardencorev1beta1.Shoot

							managedSeedNamespace = "ms1ns"
							managedSeedName      = "ms1name"
							shootName            = "ms1shoot"
						)

						BeforeEach(func() {
							managedSeed = &seedmanagementv1alpha1.ManagedSeed{
								ObjectMeta: metav1.ObjectMeta{
									Name:      managedSeedName,
									Namespace: managedSeedNamespace,
								},
								Spec: seedmanagementv1alpha1.ManagedSeedSpec{
									Shoot: &seedmanagementv1alpha1.Shoot{
										Name: shootName,
									},
								},
							}
							shoot = &gardencorev1beta1.Shoot{
								ObjectMeta: metav1.ObjectMeta{
									Name:      shootName,
									Namespace: managedSeedNamespace,
								},
								Spec: gardencorev1beta1.ShootSpec{
									SeedName: &seedName,
								},
							}

							request.Name = "gardenlet-bootstrap-" + managedSeedName
							request.Namespace = managedSeedNamespace
						})

						It("should forbid if the managedseed does not exist", func() {
							Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
								AdmissionResponse: admissionv1.AdmissionResponse{
									Allowed: false,
									Result: &metav1.Status{
										Code:    int32(http.StatusForbidden),
										Message: apierrors.NewNotFound(schema.GroupResource{Group: seedmanagementv1alpha1.SchemeGroupVersion.Group, Resource: "managedseeds"}, managedSeedName).Error(),
									},
								},
							}))
						})

						It("should return an error if reading the managedseed fails", func() {
							fakeClient := fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).WithInterceptorFuncs(interceptor.Funcs{
								Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
									if _, ok := obj.(*seedmanagementv1alpha1.ManagedSeed); ok {
										return fakeErr
									}
									return c.Get(ctx, key, obj, opts...)
								},
							}).Build()

							handler = &Handler{Logger: log, Client: fakeClient, Decoder: decoder}

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

						It("should return an error if reading the shoot fails", func() {
							fakeClient := fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).WithInterceptorFuncs(interceptor.Funcs{
								Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
									if _, ok := obj.(*gardencorev1beta1.Shoot); ok {
										return fakeErr
									}
									return c.Get(ctx, key, obj, opts...)
								},
							}).Build()

							handler = &Handler{Logger: log, Client: fakeClient, Decoder: decoder}

							Expect(fakeClient.Create(ctx, managedSeed)).To(Succeed())

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

						It("should return an error if the shoot does not belong to the gardenlet's seed", func() {
							shoot.Spec.SeedName = ptr.To("some-other-seed")

							Expect(fakeClient.Create(ctx, managedSeed)).To(Succeed())
							Expect(fakeClient.Create(ctx, shoot)).To(Succeed())

							Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
								AdmissionResponse: admissionv1.AdmissionResponse{
									Allowed: false,
									Result: &metav1.Status{
										Code:    int32(http.StatusForbidden),
										Message: fmt.Sprintf("object does not belong to seed %q", seedName),
									},
								},
							}))
						})

						It("should return an error if reading the seed fails", func() {
							fakeClient := fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).WithInterceptorFuncs(interceptor.Funcs{
								Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
									if _, ok := obj.(*gardencorev1beta1.Seed); ok {
										return fakeErr
									}
									return c.Get(ctx, key, obj, opts...)
								},
							}).Build()

							handler = &Handler{Logger: log, Client: fakeClient, Decoder: decoder}

							Expect(fakeClient.Create(ctx, managedSeed)).To(Succeed())
							Expect(fakeClient.Create(ctx, shoot)).To(Succeed())

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

						It("should forbid if the seed does exist already", func() {
							Expect(fakeClient.Create(ctx, managedSeed)).To(Succeed())
							Expect(fakeClient.Create(ctx, shoot)).To(Succeed())

							seed := &gardencorev1beta1.Seed{
								ObjectMeta: metav1.ObjectMeta{
									Name: managedSeedName,
								},
							}

							Expect(fakeClient.Create(ctx, seed)).To(Succeed())

							Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
								AdmissionResponse: admissionv1.AdmissionResponse{
									Allowed: false,
									Result: &metav1.Status{
										Code:    int32(http.StatusBadRequest),
										Message: "managed seed " + managedSeedNamespace + "/" + managedSeedName + " is already bootstrapped",
									},
								},
							}))
						})

						It("should allow if the seed does not yet exist", func() {
							Expect(fakeClient.Create(ctx, managedSeed)).To(Succeed())
							Expect(fakeClient.Create(ctx, shoot)).To(Succeed())

							Expect(handler.Handle(ctx, request)).To(Equal(responseAllowed))
						})

						It("should allow if the seed does exist but client cert is expired", func() {
							Expect(fakeClient.Create(ctx, managedSeed)).To(Succeed())
							Expect(fakeClient.Create(ctx, shoot)).To(Succeed())

							seed := &gardencorev1beta1.Seed{
								ObjectMeta: metav1.ObjectMeta{
									Name: managedSeedName,
								},
								Status: gardencorev1beta1.SeedStatus{
									ClientCertificateExpirationTimestamp: &metav1.Time{Time: time.Now().Add(-time.Hour)},
								},
							}

							Expect(fakeClient.Create(ctx, seed)).To(Succeed())

							Expect(handler.Handle(ctx, request)).To(Equal(responseAllowed))
						})
					})
				})
			})
		})

		Context("extension client", func() {
			BeforeEach(func() {
				seedUser = extensionUser
			})

			testCommonAccess()

			Context("when requested for CertificateSigningRequests", func() {
				var name string

				BeforeEach(func() {
					name = "foo"

					request.Name = name
					request.UserInfo = seedUser
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

				It("should not allow create request", func() {
					request.Operation = admissionv1.Create
					Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
						AdmissionResponse: admissionv1.AdmissionResponse{
							Allowed: false,
							Result: &metav1.Status{
								Code:    int32(http.StatusForbidden),
								Message: "extension client may not create CertificateSigningRequests",
							},
						},
					}))
				})
			})

			Context("when requested for ClusterRoleBindings", func() {
				var name string

				BeforeEach(func() {
					name = "foo"

					request.Name = name
					request.UserInfo = seedUser
					request.Resource = metav1.GroupVersionResource{
						Group:    rbacv1.SchemeGroupVersion.Group,
						Resource: "clusterrolebindings",
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

				It("should not allow create request", func() {
					request.Operation = admissionv1.Create
					Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
						AdmissionResponse: admissionv1.AdmissionResponse{
							Allowed: false,
							Result: &metav1.Status{
								Code:    int32(http.StatusForbidden),
								Message: "extension client may not create ClusterRoleBindings",
							},
						},
					}))
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
				)

				Context("when operation is create", func() {
					BeforeEach(func() {
						request.Operation = admissionv1.Create
					})

					It("should forbid the request because lease is reserved for gardenlet leader-election", func() {
						request.Name = "gardenlet-leader-election"

						Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
							AdmissionResponse: admissionv1.AdmissionResponse{
								Allowed: false,
								Result: &metav1.Status{
									Code:    int32(http.StatusForbidden),
									Message: fmt.Sprintf("extension client can only create leases in the namespace for seed %q", seedName),
								},
							},
						}))
					})

					It("should forbid the request because lease is reserved for gardenlet seed lease", func() {
						request.Name = seedName
						request.Namespace = "gardener-system-seed-lease"

						Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
							AdmissionResponse: admissionv1.AdmissionResponse{
								Allowed: false,
								Result: &metav1.Status{
									Code:    int32(http.StatusForbidden),
									Message: fmt.Sprintf("extension client can only create leases in the namespace for seed %q", seedName),
								},
							},
						}))
					})

					It("should allow the request because lease is in seed namespace", func() {
						request.Name = seedName
						request.Namespace = "seed-" + seedName

						Expect(handler.Handle(ctx, request)).To(Equal(responseAllowed))
					})
				})
			})

			Context("when requested for ServiceAccounts", func() {
				var name, namespace string

				BeforeEach(func() {
					name, namespace = "foo", "bar"

					request.Name = name
					request.Name = namespace
					request.UserInfo = seedUser
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

				It("should not allow create request", func() {
					request.Operation = admissionv1.Create
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
})
