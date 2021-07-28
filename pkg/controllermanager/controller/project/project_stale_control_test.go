// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package project_test

import (
	"context"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/controllermanager/apis/config"
	. "github.com/gardener/gardener/pkg/controllermanager/controller/project"
	"github.com/gardener/gardener/pkg/logger"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/test"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("ProjectStaleControl", func() {
	Describe("projectStaleReconciler", func() {
		var (
			ctx = context.TODO()

			ctrl                   *gomock.Controller
			k8sGardenRuntimeClient *mockclient.MockClient

			oldTimenowFunc func() time.Time

			projectName       = "foo"
			namespaceName     = "garden-foo"
			secretName        = "secret"
			secretBindingName = "secretbinding"
			quotaName         = "quotaMeta"

			minimumLifetimeDays     = 5
			staleGracePeriodDays    = 10
			staleExpirationTimeDays = 15
			staleSyncPeriod         = metav1.Duration{Duration: time.Second}

			partialShootMetaList       = &metav1.PartialObjectMetadataList{TypeMeta: metav1.TypeMeta{APIVersion: "core.gardener.cloud/v1beta1", Kind: "ShootList"}}
			partialPlantMetaList       = &metav1.PartialObjectMetadataList{TypeMeta: metav1.TypeMeta{APIVersion: "core.gardener.cloud/v1beta1", Kind: "PlantList"}}
			partialBackupEntryMetaList = &metav1.PartialObjectMetadataList{TypeMeta: metav1.TypeMeta{APIVersion: "core.gardener.cloud/v1beta1", Kind: "BackupEntryList"}}
			partialQuotaMetaList       = &metav1.PartialObjectMetadataList{TypeMeta: metav1.TypeMeta{APIVersion: "core.gardener.cloud/v1beta1", Kind: "QuotaList"}}

			project               *gardencorev1beta1.Project
			namespace             *corev1.Namespace
			partialObjectMetadata *metav1.PartialObjectMetadata
			shoot                 *gardencorev1beta1.Shoot
			quotaMeta             *metav1.PartialObjectMetadata
			secret                *corev1.Secret
			secretBinding         *gardencorev1beta1.SecretBinding
			cfg                   *config.ProjectControllerConfiguration
			request               reconcile.Request

			reconciler reconcile.Reconciler
		)

		BeforeSuite(func() {
			oldTimenowFunc = gutil.TimeNow
		})

		AfterSuite(func() {
			gutil.TimeNow = oldTimenowFunc
		})

		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())
			k8sGardenRuntimeClient = mockclient.NewMockClient(ctrl)

			logger.Logger = logger.NewNopLogger()

			project = &gardencorev1beta1.Project{
				ObjectMeta: metav1.ObjectMeta{Name: projectName},
				Spec:       gardencorev1beta1.ProjectSpec{Namespace: &namespaceName},
			}
			namespace = &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{Name: namespaceName},
			}
			partialObjectMetadata = &metav1.PartialObjectMetadata{
				ObjectMeta: metav1.ObjectMeta{Namespace: namespaceName},
			}
			shoot = &gardencorev1beta1.Shoot{
				ObjectMeta: partialObjectMetadata.ObjectMeta,
				Spec:       gardencorev1beta1.ShootSpec{SecretBindingName: secretBindingName},
			}
			quotaMeta = &metav1.PartialObjectMetadata{
				ObjectMeta: metav1.ObjectMeta{Namespace: namespaceName, Name: quotaName},
			}
			secret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Namespace: namespaceName, Name: secretName},
				Type:       corev1.SecretTypeOpaque,
			}
			secretBinding = &gardencorev1beta1.SecretBinding{
				ObjectMeta: metav1.ObjectMeta{Namespace: namespaceName, Name: secretBindingName},
				SecretRef:  corev1.SecretReference{Namespace: namespaceName, Name: secretName},
				Quotas:     []corev1.ObjectReference{{Namespace: namespaceName, Name: quotaName}},
			}
			cfg = &config.ProjectControllerConfiguration{
				MinimumLifetimeDays:     &minimumLifetimeDays,
				StaleGracePeriodDays:    &staleGracePeriodDays,
				StaleExpirationTimeDays: &staleExpirationTimeDays,
				StaleSyncPeriod:         &staleSyncPeriod,
			}
			request = reconcile.Request{NamespacedName: types.NamespacedName{Name: project.Name}}

			reconciler = NewProjectStaleReconciler(logger.NewNopLogger(), cfg, k8sGardenRuntimeClient)

			k8sGardenRuntimeClient.EXPECT().Get(ctx, kutil.Key(project.Name), gomock.AssignableToTypeOf(&gardencorev1beta1.Project{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *gardencorev1beta1.Project) error {
				*obj = *project
				return nil
			})
		})

		Describe("projectStaleReconciler", func() {
			Context("early exit", func() {
				It("should do nothing because the project has no namespace", func() {
					project.Spec.Namespace = nil

					_, result := reconciler.Reconcile(ctx, request)
					Expect(result).To(Succeed())
				})
			})

			BeforeEach(func() {
				k8sGardenRuntimeClient.EXPECT().Get(ctx, kutil.Key(namespaceName), gomock.AssignableToTypeOf(&corev1.Namespace{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *corev1.Namespace) error {
					*obj = *namespace
					return nil
				})
			})

			It("should mark the project as 'not stale' because the namespace has the skip-stale-check annotation", func() {
				nowFunc := func() metav1.Time {
					return metav1.Time{Time: time.Date(100, 1, 1, 0, 0, 0, 0, time.UTC)}
				}
				defer test.WithVar(&NowFunc, nowFunc)()

				namespace.Annotations = map[string]string{v1beta1constants.ProjectSkipStaleCheck: "true"}

				expectNonStaleMarking(k8sGardenRuntimeClient, project)

				_, result := reconciler.Reconcile(ctx, request)
				Expect(result).To(Succeed())
			})

			It("should mark the project as 'not stale' because it is younger than the configured MinimumLifetimeDays", func() {
				nowFunc := func() metav1.Time {
					return metav1.Time{Time: time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC)}
				}
				defer test.WithVar(&NowFunc, nowFunc)()

				project.CreationTimestamp = metav1.Time{Time: time.Date(1, 1, minimumLifetimeDays-1, 0, 0, 0, 0, time.UTC)}

				expectNonStaleMarking(k8sGardenRuntimeClient, project)

				_, result := reconciler.Reconcile(ctx, request)
				Expect(result).To(Succeed())
			})

			It("should mark the project as 'not stale' because the last activity was before the MinimumLifetimeDays", func() {
				nowFunc := func() metav1.Time {
					return metav1.Time{Time: time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC)}
				}
				defer test.WithVar(&NowFunc, nowFunc)()

				project.Status.LastActivityTimestamp = &metav1.Time{Time: time.Date(1, 1, minimumLifetimeDays-1, 0, 0, 0, 0, time.UTC)}

				expectNonStaleMarking(k8sGardenRuntimeClient, project)

				_, result := reconciler.Reconcile(ctx, request)
				Expect(result).To(Succeed())
			})

			Context("project older than the configured MinimumLifetimeDays", func() {
				var nowFunc func() metav1.Time

				BeforeEach(func() {
					nowFunc = func() metav1.Time {
						return metav1.Time{Time: time.Date(1, 1, minimumLifetimeDays+1, 1, 0, 0, 0, time.UTC)}
					}
					project.CreationTimestamp = metav1.Time{Time: time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC)}
				})

				Describe("project should be marked as not stale", func() {
					It("has shoots", func() {
						defer test.WithVar(&NowFunc, nowFunc)()

						k8sGardenRuntimeClient.EXPECT().List(ctx, partialShootMetaList, client.InNamespace(namespaceName), client.Limit(1)).DoAndReturn(func(ctx context.Context, list *metav1.PartialObjectMetadataList, opts ...client.ListOption) error {
							(&metav1.PartialObjectMetadataList{Items: []metav1.PartialObjectMetadata{*partialObjectMetadata}}).DeepCopyInto(list)
							return nil
						})

						expectNonStaleMarking(k8sGardenRuntimeClient, project)

						_, result := reconciler.Reconcile(ctx, request)
						Expect(result).To(Succeed())
					})

					It("has plants", func() {
						defer test.WithVar(&NowFunc, nowFunc)()

						k8sGardenRuntimeClient.EXPECT().List(ctx, partialShootMetaList, client.InNamespace(namespaceName), client.Limit(1))
						k8sGardenRuntimeClient.EXPECT().List(ctx, partialPlantMetaList, client.InNamespace(namespaceName), client.Limit(1)).DoAndReturn(func(ctx context.Context, list *metav1.PartialObjectMetadataList, opts ...client.ListOption) error {
							(&metav1.PartialObjectMetadataList{Items: []metav1.PartialObjectMetadata{*partialObjectMetadata}}).DeepCopyInto(list)
							return nil
						})

						expectNonStaleMarking(k8sGardenRuntimeClient, project)

						_, result := reconciler.Reconcile(ctx, request)
						Expect(result).To(Succeed())
					})

					It("has backupentries", func() {
						defer test.WithVar(&NowFunc, nowFunc)()

						k8sGardenRuntimeClient.EXPECT().List(ctx, partialShootMetaList, client.InNamespace(namespaceName), client.Limit(1))
						k8sGardenRuntimeClient.EXPECT().List(ctx, partialPlantMetaList, client.InNamespace(namespaceName), client.Limit(1))
						k8sGardenRuntimeClient.EXPECT().List(ctx, partialBackupEntryMetaList, client.InNamespace(namespaceName), client.Limit(1)).DoAndReturn(func(ctx context.Context, list *metav1.PartialObjectMetadataList, opts ...client.ListOption) error {
							(&metav1.PartialObjectMetadataList{Items: []metav1.PartialObjectMetadata{*partialObjectMetadata}}).DeepCopyInto(list)
							return nil
						})

						expectNonStaleMarking(k8sGardenRuntimeClient, project)

						_, result := reconciler.Reconcile(ctx, request)
						Expect(result).To(Succeed())
					})

					It("has secrets that are used by shoots in the same namespace", func() {
						defer test.WithVar(&NowFunc, nowFunc)()

						k8sGardenRuntimeClient.EXPECT().List(ctx, partialShootMetaList, client.InNamespace(namespaceName), client.Limit(1))
						k8sGardenRuntimeClient.EXPECT().List(ctx, partialPlantMetaList, client.InNamespace(namespaceName), client.Limit(1))
						k8sGardenRuntimeClient.EXPECT().List(ctx, partialBackupEntryMetaList, client.InNamespace(namespaceName), client.Limit(1))
						k8sGardenRuntimeClient.EXPECT().List(ctx, gomock.AssignableToTypeOf(&corev1.SecretList{}), client.InNamespace(namespaceName)).DoAndReturn(func(ctx context.Context, list *corev1.SecretList, opts ...client.ListOption) error {
							(&corev1.SecretList{Items: []corev1.Secret{*secret}}).DeepCopyInto(list)
							return nil
						})
						k8sGardenRuntimeClient.EXPECT().List(ctx, gomock.AssignableToTypeOf(&gardencorev1beta1.SecretBindingList{})).DoAndReturn(func(ctx context.Context, list *gardencorev1beta1.SecretBindingList, opts ...client.ListOption) error {
							(&gardencorev1beta1.SecretBindingList{Items: []gardencorev1beta1.SecretBinding{*secretBinding}}).DeepCopyInto(list)
							return nil
						})
						k8sGardenRuntimeClient.EXPECT().List(ctx, gomock.AssignableToTypeOf(&gardencorev1beta1.ShootList{}), client.InNamespace(namespaceName)).DoAndReturn(func(ctx context.Context, list *gardencorev1beta1.ShootList, opts ...client.ListOption) error {
							(&gardencorev1beta1.ShootList{Items: []gardencorev1beta1.Shoot{*shoot}}).DeepCopyInto(list)
							return nil
						})

						expectNonStaleMarking(k8sGardenRuntimeClient, project)

						_, result := reconciler.Reconcile(ctx, request)
						Expect(result).To(Succeed())
					})

					It("has secrets that are used by shoots in another namespace", func() {
						defer test.WithVar(&NowFunc, nowFunc)()

						otherNamespace := namespaceName + "other"
						secretBinding.Namespace = otherNamespace
						shoot.Namespace = otherNamespace

						k8sGardenRuntimeClient.EXPECT().List(ctx, partialShootMetaList, client.InNamespace(namespaceName), client.Limit(1))
						k8sGardenRuntimeClient.EXPECT().List(ctx, partialPlantMetaList, client.InNamespace(namespaceName), client.Limit(1))
						k8sGardenRuntimeClient.EXPECT().List(ctx, partialBackupEntryMetaList, client.InNamespace(namespaceName), client.Limit(1))
						k8sGardenRuntimeClient.EXPECT().List(ctx, gomock.AssignableToTypeOf(&corev1.SecretList{}), client.InNamespace(namespaceName)).DoAndReturn(func(ctx context.Context, list *corev1.SecretList, opts ...client.ListOption) error {
							(&corev1.SecretList{Items: []corev1.Secret{*secret}}).DeepCopyInto(list)
							return nil
						})
						k8sGardenRuntimeClient.EXPECT().List(ctx, gomock.AssignableToTypeOf(&gardencorev1beta1.SecretBindingList{})).DoAndReturn(func(ctx context.Context, list *gardencorev1beta1.SecretBindingList, opts ...client.ListOption) error {
							(&gardencorev1beta1.SecretBindingList{Items: []gardencorev1beta1.SecretBinding{*secretBinding}}).DeepCopyInto(list)
							return nil
						})
						k8sGardenRuntimeClient.EXPECT().List(ctx, gomock.AssignableToTypeOf(&gardencorev1beta1.ShootList{}), client.InNamespace(otherNamespace)).DoAndReturn(func(ctx context.Context, list *gardencorev1beta1.ShootList, opts ...client.ListOption) error {
							(&gardencorev1beta1.ShootList{Items: []gardencorev1beta1.Shoot{*shoot}}).DeepCopyInto(list)
							return nil
						})

						expectNonStaleMarking(k8sGardenRuntimeClient, project)

						_, result := reconciler.Reconcile(ctx, request)
						Expect(result).To(Succeed())
					})

					It("has quotas that are used by shoots in the same namespace", func() {
						defer test.WithVar(&NowFunc, nowFunc)()

						k8sGardenRuntimeClient.EXPECT().List(ctx, partialShootMetaList, client.InNamespace(namespaceName), client.Limit(1))
						k8sGardenRuntimeClient.EXPECT().List(ctx, partialPlantMetaList, client.InNamespace(namespaceName), client.Limit(1))
						k8sGardenRuntimeClient.EXPECT().List(ctx, partialBackupEntryMetaList, client.InNamespace(namespaceName), client.Limit(1))
						k8sGardenRuntimeClient.EXPECT().List(ctx, gomock.AssignableToTypeOf(&corev1.SecretList{}), client.InNamespace(namespaceName))
						k8sGardenRuntimeClient.EXPECT().List(ctx, partialQuotaMetaList, client.InNamespace(namespaceName)).DoAndReturn(func(ctx context.Context, list *metav1.PartialObjectMetadataList, opts ...client.ListOption) error {
							(&metav1.PartialObjectMetadataList{Items: []metav1.PartialObjectMetadata{*quotaMeta}}).DeepCopyInto(list)
							return nil
						})
						k8sGardenRuntimeClient.EXPECT().List(ctx, gomock.AssignableToTypeOf(&gardencorev1beta1.SecretBindingList{})).DoAndReturn(func(ctx context.Context, list *gardencorev1beta1.SecretBindingList, opts ...client.ListOption) error {
							(&gardencorev1beta1.SecretBindingList{Items: []gardencorev1beta1.SecretBinding{*secretBinding}}).DeepCopyInto(list)
							return nil
						})
						k8sGardenRuntimeClient.EXPECT().List(ctx, gomock.AssignableToTypeOf(&gardencorev1beta1.ShootList{}), client.InNamespace(namespaceName)).DoAndReturn(func(ctx context.Context, list *gardencorev1beta1.ShootList, opts ...client.ListOption) error {
							(&gardencorev1beta1.ShootList{Items: []gardencorev1beta1.Shoot{*shoot}}).DeepCopyInto(list)
							return nil
						})

						expectNonStaleMarking(k8sGardenRuntimeClient, project)

						_, result := reconciler.Reconcile(ctx, request)
						Expect(result).To(Succeed())
					})

					It("has quotas that are used by shoots in another namespace", func() {
						defer test.WithVar(&NowFunc, nowFunc)()

						otherNamespace := namespaceName + "other"
						secretBinding.Namespace = otherNamespace
						shoot.Namespace = otherNamespace

						k8sGardenRuntimeClient.EXPECT().List(ctx, partialShootMetaList, client.InNamespace(namespaceName), client.Limit(1))
						k8sGardenRuntimeClient.EXPECT().List(ctx, partialPlantMetaList, client.InNamespace(namespaceName), client.Limit(1))
						k8sGardenRuntimeClient.EXPECT().List(ctx, partialBackupEntryMetaList, client.InNamespace(namespaceName), client.Limit(1))
						k8sGardenRuntimeClient.EXPECT().List(ctx, gomock.AssignableToTypeOf(&corev1.SecretList{}), client.InNamespace(namespaceName))
						k8sGardenRuntimeClient.EXPECT().List(ctx, partialQuotaMetaList, client.InNamespace(namespaceName)).DoAndReturn(func(ctx context.Context, list *metav1.PartialObjectMetadataList, opts ...client.ListOption) error {
							(&metav1.PartialObjectMetadataList{Items: []metav1.PartialObjectMetadata{*quotaMeta}}).DeepCopyInto(list)
							return nil
						})
						k8sGardenRuntimeClient.EXPECT().List(ctx, gomock.AssignableToTypeOf(&gardencorev1beta1.SecretBindingList{})).DoAndReturn(func(ctx context.Context, list *gardencorev1beta1.SecretBindingList, opts ...client.ListOption) error {
							(&gardencorev1beta1.SecretBindingList{Items: []gardencorev1beta1.SecretBinding{*secretBinding}}).DeepCopyInto(list)
							return nil
						})
						k8sGardenRuntimeClient.EXPECT().List(ctx, gomock.AssignableToTypeOf(&gardencorev1beta1.ShootList{}), client.InNamespace(otherNamespace)).DoAndReturn(func(ctx context.Context, list *gardencorev1beta1.ShootList, opts ...client.ListOption) error {
							(&gardencorev1beta1.ShootList{Items: []gardencorev1beta1.Shoot{*shoot}}).DeepCopyInto(list)
							return nil
						})

						expectNonStaleMarking(k8sGardenRuntimeClient, project)

						_, result := reconciler.Reconcile(ctx, request)
						Expect(result).To(Succeed())
					})

				})

				Describe("project should be marked as stale", func() {
					It("has secrets that are unused", func() {
						defer test.WithVar(&NowFunc, nowFunc)()

						k8sGardenRuntimeClient.EXPECT().List(ctx, partialShootMetaList, client.InNamespace(namespaceName), client.Limit(1))
						k8sGardenRuntimeClient.EXPECT().List(ctx, partialPlantMetaList, client.InNamespace(namespaceName), client.Limit(1))
						k8sGardenRuntimeClient.EXPECT().List(ctx, partialBackupEntryMetaList, client.InNamespace(namespaceName), client.Limit(1))
						k8sGardenRuntimeClient.EXPECT().List(ctx, gomock.AssignableToTypeOf(&corev1.SecretList{}), client.InNamespace(namespaceName)).DoAndReturn(func(ctx context.Context, list *corev1.SecretList, opts ...client.ListOption) error {
							(&corev1.SecretList{Items: []corev1.Secret{*secret}}).DeepCopyInto(list)
							return nil
						})
						k8sGardenRuntimeClient.EXPECT().List(ctx, gomock.AssignableToTypeOf(&gardencorev1beta1.SecretBindingList{})).DoAndReturn(func(ctx context.Context, list *gardencorev1beta1.SecretBindingList, opts ...client.ListOption) error {
							(&gardencorev1beta1.SecretBindingList{Items: []gardencorev1beta1.SecretBinding{*secretBinding}}).DeepCopyInto(list)
							return nil
						})
						k8sGardenRuntimeClient.EXPECT().List(ctx, gomock.AssignableToTypeOf(&gardencorev1beta1.ShootList{}), client.InNamespace(namespaceName))
						k8sGardenRuntimeClient.EXPECT().List(ctx, partialQuotaMetaList, client.InNamespace(namespaceName))

						expectStaleMarking(k8sGardenRuntimeClient, project, nil, nil, nowFunc)

						_, result := reconciler.Reconcile(ctx, request)
						Expect(result).To(Succeed())
					})

					It("has quotas that are unused", func() {
						defer test.WithVar(&NowFunc, nowFunc)()

						k8sGardenRuntimeClient.EXPECT().List(ctx, partialShootMetaList, client.InNamespace(namespaceName), client.Limit(1))
						k8sGardenRuntimeClient.EXPECT().List(ctx, partialPlantMetaList, client.InNamespace(namespaceName), client.Limit(1))
						k8sGardenRuntimeClient.EXPECT().List(ctx, partialBackupEntryMetaList, client.InNamespace(namespaceName), client.Limit(1))
						k8sGardenRuntimeClient.EXPECT().List(ctx, gomock.AssignableToTypeOf(&corev1.SecretList{}), client.InNamespace(namespaceName))
						k8sGardenRuntimeClient.EXPECT().List(ctx, partialQuotaMetaList, client.InNamespace(namespaceName)).DoAndReturn(func(ctx context.Context, list *metav1.PartialObjectMetadataList, opts ...client.ListOption) error {
							(&metav1.PartialObjectMetadataList{Items: []metav1.PartialObjectMetadata{*quotaMeta}}).DeepCopyInto(list)
							return nil
						})
						k8sGardenRuntimeClient.EXPECT().List(ctx, gomock.AssignableToTypeOf(&gardencorev1beta1.SecretBindingList{})).DoAndReturn(func(ctx context.Context, list *gardencorev1beta1.SecretBindingList, opts ...client.ListOption) error {
							(&gardencorev1beta1.SecretBindingList{Items: []gardencorev1beta1.SecretBinding{*secretBinding}}).DeepCopyInto(list)
							return nil
						})
						k8sGardenRuntimeClient.EXPECT().List(ctx, gomock.AssignableToTypeOf(&gardencorev1beta1.ShootList{}), client.InNamespace(namespaceName))

						expectStaleMarking(k8sGardenRuntimeClient, project, nil, nil, nowFunc)

						_, result := reconciler.Reconcile(ctx, request)
						Expect(result).To(Succeed())
					})

					It("it is not used", func() {
						defer test.WithVar(&NowFunc, nowFunc)()

						k8sGardenRuntimeClient.EXPECT().List(ctx, partialShootMetaList, client.InNamespace(namespaceName), client.Limit(1))
						k8sGardenRuntimeClient.EXPECT().List(ctx, partialPlantMetaList, client.InNamespace(namespaceName), client.Limit(1))
						k8sGardenRuntimeClient.EXPECT().List(ctx, partialBackupEntryMetaList, client.InNamespace(namespaceName), client.Limit(1))
						k8sGardenRuntimeClient.EXPECT().List(ctx, gomock.AssignableToTypeOf(&corev1.SecretList{}), client.InNamespace(namespaceName))
						k8sGardenRuntimeClient.EXPECT().List(ctx, partialQuotaMetaList, client.InNamespace(namespaceName))

						expectStaleMarking(k8sGardenRuntimeClient, project, nil, nil, nowFunc)

						_, result := reconciler.Reconcile(ctx, request)
						Expect(result).To(Succeed())
					})

					It("should not set the auto delete timestamp because stale grace period is not exceeded", func() {
						defer test.WithVar(&NowFunc, nowFunc)()

						staleSinceTimestamp := metav1.Time{Time: nowFunc().Add(-24*time.Hour*time.Duration(staleGracePeriodDays) + time.Hour)}
						project.Status.StaleSinceTimestamp = &staleSinceTimestamp

						k8sGardenRuntimeClient.EXPECT().List(ctx, partialShootMetaList, client.InNamespace(namespaceName), client.Limit(1))
						k8sGardenRuntimeClient.EXPECT().List(ctx, partialPlantMetaList, client.InNamespace(namespaceName), client.Limit(1))
						k8sGardenRuntimeClient.EXPECT().List(ctx, partialBackupEntryMetaList, client.InNamespace(namespaceName), client.Limit(1))
						k8sGardenRuntimeClient.EXPECT().List(ctx, gomock.AssignableToTypeOf(&corev1.SecretList{}), client.InNamespace(namespaceName))
						k8sGardenRuntimeClient.EXPECT().List(ctx, partialQuotaMetaList, client.InNamespace(namespaceName))

						expectStaleMarking(k8sGardenRuntimeClient, project, &staleSinceTimestamp, nil, nowFunc)

						_, result := reconciler.Reconcile(ctx, request)
						Expect(result).To(Succeed())
					})

					It("should set the auto delete timestamp because stale grace period is exceeded", func() {
						defer test.WithVar(&NowFunc, nowFunc)()

						var (
							staleSinceTimestamp      = metav1.Time{Time: nowFunc().Add(-24 * time.Hour * time.Duration(staleGracePeriodDays))}
							staleAutoDeleteTimestamp = metav1.Time{Time: staleSinceTimestamp.Add(24 * time.Hour * time.Duration(staleExpirationTimeDays))}
						)
						project.Status.StaleSinceTimestamp = &staleSinceTimestamp

						k8sGardenRuntimeClient.EXPECT().List(ctx, partialShootMetaList, client.InNamespace(namespaceName), client.Limit(1))
						k8sGardenRuntimeClient.EXPECT().List(ctx, partialPlantMetaList, client.InNamespace(namespaceName), client.Limit(1))
						k8sGardenRuntimeClient.EXPECT().List(ctx, partialBackupEntryMetaList, client.InNamespace(namespaceName), client.Limit(1))
						k8sGardenRuntimeClient.EXPECT().List(ctx, gomock.AssignableToTypeOf(&corev1.SecretList{}), client.InNamespace(namespaceName))
						k8sGardenRuntimeClient.EXPECT().List(ctx, partialQuotaMetaList, client.InNamespace(namespaceName))

						expectStaleMarking(k8sGardenRuntimeClient, project, &staleSinceTimestamp, &staleAutoDeleteTimestamp, nowFunc)

						_, result := reconciler.Reconcile(ctx, request)
						Expect(result).To(Succeed())
					})

					It("should delete the project if the auto delete timestamp is exceeded", func() {
						defer test.WithVar(&NowFunc, nowFunc)()

						var (
							staleSinceTimestamp      = metav1.Time{Time: nowFunc().Add(-24 * time.Hour * 3 * time.Duration(staleExpirationTimeDays))}
							staleAutoDeleteTimestamp = nowFunc()
						)

						project.Status.StaleSinceTimestamp = &staleSinceTimestamp
						project.Status.StaleAutoDeleteTimestamp = &staleAutoDeleteTimestamp

						k8sGardenRuntimeClient.EXPECT().List(ctx, partialShootMetaList, client.InNamespace(namespaceName), client.Limit(1))
						k8sGardenRuntimeClient.EXPECT().List(ctx, partialPlantMetaList, client.InNamespace(namespaceName), client.Limit(1))
						k8sGardenRuntimeClient.EXPECT().List(ctx, partialBackupEntryMetaList, client.InNamespace(namespaceName), client.Limit(1))
						k8sGardenRuntimeClient.EXPECT().List(ctx, gomock.AssignableToTypeOf(&corev1.SecretList{}), client.InNamespace(namespaceName))
						k8sGardenRuntimeClient.EXPECT().List(ctx, partialQuotaMetaList, client.InNamespace(namespaceName))

						expectStaleMarking(k8sGardenRuntimeClient, project, &staleSinceTimestamp, &staleAutoDeleteTimestamp, nowFunc)

						gutil.TimeNow = func() time.Time {
							return time.Date(1, 1, minimumLifetimeDays+1, 1, 0, 0, 0, time.UTC)
						}

						projectCopy := project.DeepCopy()
						projectCopy.Annotations = map[string]string{
							gutil.ConfirmationDeletion:         "true",
							v1beta1constants.GardenerTimestamp: gutil.TimeNow().UTC().String(),
						}
						k8sGardenRuntimeClient.EXPECT().Patch(ctx, projectCopy, gomock.Any())
						k8sGardenRuntimeClient.EXPECT().Delete(ctx, projectCopy)

						_, result := reconciler.Reconcile(ctx, request)
						Expect(result).To(Succeed())
					})
				})
			})
		})
	})
})

func expectNonStaleMarking(k8sGardenRuntimeClient *mockclient.MockClient, project *gardencorev1beta1.Project) {
	k8sGardenRuntimeClient.EXPECT().Status().Return(k8sGardenRuntimeClient)

	projectPatched := project.DeepCopy()
	projectPatched.Status.StaleSinceTimestamp = nil
	projectPatched.Status.StaleAutoDeleteTimestamp = nil

	test.EXPECTPatch(context.TODO(), k8sGardenRuntimeClient, projectPatched, project, types.StrategicMergePatchType)
}

func expectStaleMarking(k8sGardenRuntimeClient *mockclient.MockClient, project *gardencorev1beta1.Project, staleSinceTimestamp, staleAutoDeleteTimestamp *metav1.Time, nowFunc func() metav1.Time) {
	k8sGardenRuntimeClient.EXPECT().Status().Return(k8sGardenRuntimeClient)

	projectPatched := project.DeepCopy()
	if staleSinceTimestamp == nil {
		now := nowFunc()
		projectPatched.Status.StaleSinceTimestamp = &now
	} else {
		projectPatched.Status.StaleSinceTimestamp = staleSinceTimestamp
	}
	projectPatched.Status.StaleAutoDeleteTimestamp = staleAutoDeleteTimestamp

	test.EXPECTPatch(context.TODO(), k8sGardenRuntimeClient, projectPatched, project, types.StrategicMergePatchType)
}
