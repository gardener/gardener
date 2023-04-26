// Copyright 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package stale_test

import (
	"context"
	"time"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/clock/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/controllermanager/apis/config"
	. "github.com/gardener/gardener/pkg/controllermanager/controller/project/stale"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/test"
)

var _ = Describe("Reconciler", func() {
	var (
		ctx       = context.TODO()
		fakeClock *testing.FakeClock

		k8sGardenRuntimeClient *mockclient.MockClient
		mockStatusWriter       *mockclient.MockStatusWriter

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
		partialBackupEntryMetaList = &metav1.PartialObjectMetadataList{TypeMeta: metav1.TypeMeta{APIVersion: "core.gardener.cloud/v1beta1", Kind: "BackupEntryList"}}
		partialQuotaMetaList       = &metav1.PartialObjectMetadataList{TypeMeta: metav1.TypeMeta{APIVersion: "core.gardener.cloud/v1beta1", Kind: "QuotaList"}}

		project               *gardencorev1beta1.Project
		namespace             *corev1.Namespace
		partialObjectMetadata *metav1.PartialObjectMetadata
		shoot                 *gardencorev1beta1.Shoot
		quotaMeta             *metav1.PartialObjectMetadata
		secret                *corev1.Secret
		secretBinding         *gardencorev1beta1.SecretBinding
		cfg                   config.ProjectControllerConfiguration
		request               reconcile.Request

		reconciler reconcile.Reconciler
	)

	BeforeEach(func() {
		ctrl := gomock.NewController(GinkgoT())
		k8sGardenRuntimeClient = mockclient.NewMockClient(ctrl)
		mockStatusWriter = mockclient.NewMockStatusWriter(ctrl)
		fakeClock = testing.NewFakeClock(time.Now())

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
			Quotas:     []corev1.ObjectReference{{}, {Namespace: namespaceName, Name: quotaName}},
		}
		cfg = config.ProjectControllerConfiguration{
			MinimumLifetimeDays:     &minimumLifetimeDays,
			StaleGracePeriodDays:    &staleGracePeriodDays,
			StaleExpirationTimeDays: &staleExpirationTimeDays,
			StaleSyncPeriod:         &staleSyncPeriod,
		}
		request = reconcile.Request{NamespacedName: types.NamespacedName{Name: project.Name}}

		reconciler = &Reconciler{
			Client: k8sGardenRuntimeClient,
			Config: cfg,
			Clock:  fakeClock,
		}

		k8sGardenRuntimeClient.EXPECT().Get(gomock.Any(), kubernetesutils.Key(project.Name), gomock.AssignableToTypeOf(&gardencorev1beta1.Project{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *gardencorev1beta1.Project, _ ...client.GetOption) error {
			*obj = *project
			return nil
		})
	})

	Describe("#Reconcile", func() {
		Context("early exit", func() {
			It("should do nothing because the project has no namespace", func() {
				project.Spec.Namespace = nil

				_, result := reconciler.Reconcile(ctx, request)
				Expect(result).To(Succeed())
			})
		})

		BeforeEach(func() {
			k8sGardenRuntimeClient.EXPECT().Get(gomock.Any(), kubernetesutils.Key(namespaceName), gomock.AssignableToTypeOf(&corev1.Namespace{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *corev1.Namespace, _ ...client.GetOption) error {
				*obj = *namespace
				return nil
			}).AnyTimes()
		})

		It("should mark the project as 'not stale' because the namespace has the skip-stale-check annotation", func() {
			fakeClock.SetTime(time.Date(100, 1, 1, 0, 0, 0, 0, time.UTC))

			namespace.Annotations = map[string]string{v1beta1constants.ProjectSkipStaleCheck: "true"}

			expectNonStaleMarking(k8sGardenRuntimeClient, mockStatusWriter, project)

			_, result := reconciler.Reconcile(ctx, request)
			Expect(result).To(Succeed())
		})

		It("should mark the project as 'not stale' because it is younger than the configured MinimumLifetimeDays", func() {
			fakeClock.SetTime(time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC))

			project.CreationTimestamp = metav1.Time{Time: time.Date(1, 1, minimumLifetimeDays-1, 0, 0, 0, 0, time.UTC)}

			expectNonStaleMarking(k8sGardenRuntimeClient, mockStatusWriter, project)

			_, result := reconciler.Reconcile(ctx, request)
			Expect(result).To(Succeed())
		})

		It("should mark the project as 'not stale' because the last activity was before the MinimumLifetimeDays", func() {
			fakeClock.SetTime(time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC))

			project.Status.LastActivityTimestamp = &metav1.Time{Time: time.Date(1, 1, minimumLifetimeDays-1, 0, 0, 0, 0, time.UTC)}

			expectNonStaleMarking(k8sGardenRuntimeClient, mockStatusWriter, project)

			_, result := reconciler.Reconcile(ctx, request)
			Expect(result).To(Succeed())
		})

		Context("project older than the configured MinimumLifetimeDays", func() {
			BeforeEach(func() {
				fakeClock.SetTime(time.Date(1, 1, minimumLifetimeDays+1, 1, 0, 0, 0, time.UTC))
				project.CreationTimestamp = metav1.Time{Time: time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC)}
			})

			Describe("project should be marked as not stale", func() {
				It("has shoots", func() {
					k8sGardenRuntimeClient.EXPECT().List(gomock.Any(), partialShootMetaList, client.InNamespace(namespaceName), client.Limit(1)).DoAndReturn(func(ctx context.Context, list *metav1.PartialObjectMetadataList, opts ...client.ListOption) error {
						(&metav1.PartialObjectMetadataList{Items: []metav1.PartialObjectMetadata{*partialObjectMetadata}}).DeepCopyInto(list)
						return nil
					})

					expectNonStaleMarking(k8sGardenRuntimeClient, mockStatusWriter, project)

					_, result := reconciler.Reconcile(ctx, request)
					Expect(result).To(Succeed())
				})

				It("has backupentries", func() {
					k8sGardenRuntimeClient.EXPECT().List(gomock.Any(), partialShootMetaList, client.InNamespace(namespaceName), client.Limit(1))
					k8sGardenRuntimeClient.EXPECT().List(gomock.Any(), partialBackupEntryMetaList, client.InNamespace(namespaceName), client.Limit(1)).DoAndReturn(func(ctx context.Context, list *metav1.PartialObjectMetadataList, opts ...client.ListOption) error {
						(&metav1.PartialObjectMetadataList{Items: []metav1.PartialObjectMetadata{*partialObjectMetadata}}).DeepCopyInto(list)
						return nil
					})

					expectNonStaleMarking(k8sGardenRuntimeClient, mockStatusWriter, project)

					_, result := reconciler.Reconcile(ctx, request)
					Expect(result).To(Succeed())
				})

				It("has secrets that are used by shoots in the same namespace", func() {
					k8sGardenRuntimeClient.EXPECT().List(gomock.Any(), partialShootMetaList, client.InNamespace(namespaceName), client.Limit(1))
					k8sGardenRuntimeClient.EXPECT().List(gomock.Any(), partialBackupEntryMetaList, client.InNamespace(namespaceName), client.Limit(1))
					k8sGardenRuntimeClient.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&corev1.SecretList{}), client.InNamespace(namespaceName)).DoAndReturn(func(ctx context.Context, list *corev1.SecretList, opts ...client.ListOption) error {
						(&corev1.SecretList{Items: []corev1.Secret{*secret}}).DeepCopyInto(list)
						return nil
					})
					k8sGardenRuntimeClient.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.SecretBindingList{})).DoAndReturn(func(ctx context.Context, list *gardencorev1beta1.SecretBindingList, opts ...client.ListOption) error {
						(&gardencorev1beta1.SecretBindingList{Items: []gardencorev1beta1.SecretBinding{*secretBinding}}).DeepCopyInto(list)
						return nil
					})
					k8sGardenRuntimeClient.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.ShootList{}), client.InNamespace(namespaceName)).DoAndReturn(func(ctx context.Context, list *gardencorev1beta1.ShootList, opts ...client.ListOption) error {
						(&gardencorev1beta1.ShootList{Items: []gardencorev1beta1.Shoot{*shoot}}).DeepCopyInto(list)
						return nil
					})

					expectNonStaleMarking(k8sGardenRuntimeClient, mockStatusWriter, project)

					_, result := reconciler.Reconcile(ctx, request)
					Expect(result).To(Succeed())
				})

				It("has secrets that are used by shoots in another namespace", func() {
					otherNamespace := namespaceName + "other"
					secretBinding.Namespace = otherNamespace
					shoot.Namespace = otherNamespace

					k8sGardenRuntimeClient.EXPECT().List(gomock.Any(), partialShootMetaList, client.InNamespace(namespaceName), client.Limit(1))
					k8sGardenRuntimeClient.EXPECT().List(gomock.Any(), partialBackupEntryMetaList, client.InNamespace(namespaceName), client.Limit(1))
					k8sGardenRuntimeClient.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&corev1.SecretList{}), client.InNamespace(namespaceName)).DoAndReturn(func(ctx context.Context, list *corev1.SecretList, opts ...client.ListOption) error {
						(&corev1.SecretList{Items: []corev1.Secret{*secret}}).DeepCopyInto(list)
						return nil
					})
					k8sGardenRuntimeClient.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.SecretBindingList{})).DoAndReturn(func(ctx context.Context, list *gardencorev1beta1.SecretBindingList, opts ...client.ListOption) error {
						(&gardencorev1beta1.SecretBindingList{Items: []gardencorev1beta1.SecretBinding{*secretBinding}}).DeepCopyInto(list)
						return nil
					})
					k8sGardenRuntimeClient.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.ShootList{}), client.InNamespace(otherNamespace)).DoAndReturn(func(ctx context.Context, list *gardencorev1beta1.ShootList, opts ...client.ListOption) error {
						(&gardencorev1beta1.ShootList{Items: []gardencorev1beta1.Shoot{*shoot}}).DeepCopyInto(list)
						return nil
					})

					expectNonStaleMarking(k8sGardenRuntimeClient, mockStatusWriter, project)

					_, result := reconciler.Reconcile(ctx, request)
					Expect(result).To(Succeed())
				})

				It("has quotas that are used by shoots in the same namespace", func() {
					k8sGardenRuntimeClient.EXPECT().List(gomock.Any(), partialShootMetaList, client.InNamespace(namespaceName), client.Limit(1))
					k8sGardenRuntimeClient.EXPECT().List(gomock.Any(), partialBackupEntryMetaList, client.InNamespace(namespaceName), client.Limit(1))
					k8sGardenRuntimeClient.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&corev1.SecretList{}), client.InNamespace(namespaceName))
					k8sGardenRuntimeClient.EXPECT().List(gomock.Any(), partialQuotaMetaList, client.InNamespace(namespaceName)).DoAndReturn(func(ctx context.Context, list *metav1.PartialObjectMetadataList, opts ...client.ListOption) error {
						(&metav1.PartialObjectMetadataList{Items: []metav1.PartialObjectMetadata{*quotaMeta}}).DeepCopyInto(list)
						return nil
					})
					k8sGardenRuntimeClient.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.SecretBindingList{})).DoAndReturn(func(ctx context.Context, list *gardencorev1beta1.SecretBindingList, opts ...client.ListOption) error {
						(&gardencorev1beta1.SecretBindingList{Items: []gardencorev1beta1.SecretBinding{*secretBinding}}).DeepCopyInto(list)
						return nil
					})
					k8sGardenRuntimeClient.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.ShootList{}), client.InNamespace(namespaceName)).DoAndReturn(func(ctx context.Context, list *gardencorev1beta1.ShootList, opts ...client.ListOption) error {
						(&gardencorev1beta1.ShootList{Items: []gardencorev1beta1.Shoot{*shoot}}).DeepCopyInto(list)
						return nil
					})

					expectNonStaleMarking(k8sGardenRuntimeClient, mockStatusWriter, project)

					_, result := reconciler.Reconcile(ctx, request)
					Expect(result).To(Succeed())
				})

				It("has quotas that are used by shoots in another namespace", func() {
					otherNamespace := namespaceName + "other"
					secretBinding.Namespace = otherNamespace
					shoot.Namespace = otherNamespace

					k8sGardenRuntimeClient.EXPECT().List(gomock.Any(), partialShootMetaList, client.InNamespace(namespaceName), client.Limit(1))
					k8sGardenRuntimeClient.EXPECT().List(gomock.Any(), partialBackupEntryMetaList, client.InNamespace(namespaceName), client.Limit(1))
					k8sGardenRuntimeClient.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&corev1.SecretList{}), client.InNamespace(namespaceName))
					k8sGardenRuntimeClient.EXPECT().List(gomock.Any(), partialQuotaMetaList, client.InNamespace(namespaceName)).DoAndReturn(func(ctx context.Context, list *metav1.PartialObjectMetadataList, opts ...client.ListOption) error {
						(&metav1.PartialObjectMetadataList{Items: []metav1.PartialObjectMetadata{*quotaMeta}}).DeepCopyInto(list)
						return nil
					})
					k8sGardenRuntimeClient.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.SecretBindingList{})).DoAndReturn(func(ctx context.Context, list *gardencorev1beta1.SecretBindingList, opts ...client.ListOption) error {
						(&gardencorev1beta1.SecretBindingList{Items: []gardencorev1beta1.SecretBinding{*secretBinding}}).DeepCopyInto(list)
						return nil
					})
					k8sGardenRuntimeClient.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.ShootList{}), client.InNamespace(otherNamespace)).DoAndReturn(func(ctx context.Context, list *gardencorev1beta1.ShootList, opts ...client.ListOption) error {
						(&gardencorev1beta1.ShootList{Items: []gardencorev1beta1.Shoot{*shoot}}).DeepCopyInto(list)
						return nil
					})

					expectNonStaleMarking(k8sGardenRuntimeClient, mockStatusWriter, project)

					_, result := reconciler.Reconcile(ctx, request)
					Expect(result).To(Succeed())
				})

			})

			Describe("project should be marked as stale", func() {
				It("has secrets that are unused", func() {
					k8sGardenRuntimeClient.EXPECT().List(gomock.Any(), partialShootMetaList, client.InNamespace(namespaceName), client.Limit(1))
					k8sGardenRuntimeClient.EXPECT().List(gomock.Any(), partialBackupEntryMetaList, client.InNamespace(namespaceName), client.Limit(1))
					k8sGardenRuntimeClient.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&corev1.SecretList{}), client.InNamespace(namespaceName)).DoAndReturn(func(ctx context.Context, list *corev1.SecretList, opts ...client.ListOption) error {
						(&corev1.SecretList{Items: []corev1.Secret{*secret}}).DeepCopyInto(list)
						return nil
					})
					k8sGardenRuntimeClient.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.SecretBindingList{})).DoAndReturn(func(ctx context.Context, list *gardencorev1beta1.SecretBindingList, opts ...client.ListOption) error {
						(&gardencorev1beta1.SecretBindingList{Items: []gardencorev1beta1.SecretBinding{*secretBinding}}).DeepCopyInto(list)
						return nil
					})
					k8sGardenRuntimeClient.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.ShootList{}), client.InNamespace(namespaceName))
					k8sGardenRuntimeClient.EXPECT().List(gomock.Any(), partialQuotaMetaList, client.InNamespace(namespaceName))

					expectStaleMarking(k8sGardenRuntimeClient, mockStatusWriter, project, nil, nil, fakeClock)

					_, result := reconciler.Reconcile(ctx, request)
					Expect(result).To(Succeed())
				})

				It("has secrets that have owner references to a shoot", func() {
					otherNamespace := namespaceName + "other"
					secretBinding.Namespace = otherNamespace
					shoot.Namespace = otherNamespace

					secretWithOwnerRef := secret.DeepCopy()
					secretWithOwnerRef.OwnerReferences = []metav1.OwnerReference{
						{
							APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
							Kind:       "Shoot",
						},
					}

					k8sGardenRuntimeClient.EXPECT().List(gomock.Any(), partialShootMetaList, client.InNamespace(namespaceName), client.Limit(1))
					k8sGardenRuntimeClient.EXPECT().List(gomock.Any(), partialBackupEntryMetaList, client.InNamespace(namespaceName), client.Limit(1))
					k8sGardenRuntimeClient.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&corev1.SecretList{}), client.InNamespace(namespaceName)).DoAndReturn(func(ctx context.Context, list *corev1.SecretList, opts ...client.ListOption) error {
						(&corev1.SecretList{Items: []corev1.Secret{*secretWithOwnerRef}}).DeepCopyInto(list)
						return nil
					})
					k8sGardenRuntimeClient.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.SecretBindingList{})).DoAndReturn(func(ctx context.Context, list *gardencorev1beta1.SecretBindingList, opts ...client.ListOption) error {
						(&gardencorev1beta1.SecretBindingList{Items: []gardencorev1beta1.SecretBinding{*secretBinding}}).DeepCopyInto(list)
						return nil
					}).AnyTimes()
					k8sGardenRuntimeClient.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.ShootList{}), client.InNamespace(otherNamespace)).DoAndReturn(func(ctx context.Context, list *gardencorev1beta1.ShootList, opts ...client.ListOption) error {
						(&gardencorev1beta1.ShootList{Items: []gardencorev1beta1.Shoot{*shoot}}).DeepCopyInto(list)
						return nil
					}).AnyTimes()
					k8sGardenRuntimeClient.EXPECT().List(gomock.Any(), partialQuotaMetaList, client.InNamespace(namespaceName))

					expectStaleMarking(k8sGardenRuntimeClient, mockStatusWriter, project, nil, nil, fakeClock)

					_, result := reconciler.Reconcile(ctx, request)
					Expect(result).To(Succeed())
				})

				It("has quotas that are unused", func() {
					k8sGardenRuntimeClient.EXPECT().List(gomock.Any(), partialShootMetaList, client.InNamespace(namespaceName), client.Limit(1))
					k8sGardenRuntimeClient.EXPECT().List(gomock.Any(), partialBackupEntryMetaList, client.InNamespace(namespaceName), client.Limit(1))
					k8sGardenRuntimeClient.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&corev1.SecretList{}), client.InNamespace(namespaceName))
					k8sGardenRuntimeClient.EXPECT().List(gomock.Any(), partialQuotaMetaList, client.InNamespace(namespaceName)).DoAndReturn(func(ctx context.Context, list *metav1.PartialObjectMetadataList, opts ...client.ListOption) error {
						(&metav1.PartialObjectMetadataList{Items: []metav1.PartialObjectMetadata{*quotaMeta}}).DeepCopyInto(list)
						return nil
					})
					k8sGardenRuntimeClient.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.SecretBindingList{})).DoAndReturn(func(ctx context.Context, list *gardencorev1beta1.SecretBindingList, opts ...client.ListOption) error {
						(&gardencorev1beta1.SecretBindingList{Items: []gardencorev1beta1.SecretBinding{*secretBinding}}).DeepCopyInto(list)
						return nil
					})
					k8sGardenRuntimeClient.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.ShootList{}), client.InNamespace(namespaceName))

					expectStaleMarking(k8sGardenRuntimeClient, mockStatusWriter, project, nil, nil, fakeClock)

					_, result := reconciler.Reconcile(ctx, request)
					Expect(result).To(Succeed())
				})

				It("it is not used", func() {
					k8sGardenRuntimeClient.EXPECT().List(gomock.Any(), partialShootMetaList, client.InNamespace(namespaceName), client.Limit(1))
					k8sGardenRuntimeClient.EXPECT().List(gomock.Any(), partialBackupEntryMetaList, client.InNamespace(namespaceName), client.Limit(1))
					k8sGardenRuntimeClient.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&corev1.SecretList{}), client.InNamespace(namespaceName))
					k8sGardenRuntimeClient.EXPECT().List(gomock.Any(), partialQuotaMetaList, client.InNamespace(namespaceName))

					expectStaleMarking(k8sGardenRuntimeClient, mockStatusWriter, project, nil, nil, fakeClock)

					_, result := reconciler.Reconcile(ctx, request)
					Expect(result).To(Succeed())
				})

				It("should not set the auto delete timestamp because stale grace period is not exceeded", func() {
					staleSinceTimestamp := metav1.Time{Time: fakeClock.Now().Add(-24*time.Hour*time.Duration(staleGracePeriodDays) + time.Hour)}
					project.Status.StaleSinceTimestamp = &staleSinceTimestamp

					k8sGardenRuntimeClient.EXPECT().List(gomock.Any(), partialShootMetaList, client.InNamespace(namespaceName), client.Limit(1))
					k8sGardenRuntimeClient.EXPECT().List(gomock.Any(), partialBackupEntryMetaList, client.InNamespace(namespaceName), client.Limit(1))
					k8sGardenRuntimeClient.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&corev1.SecretList{}), client.InNamespace(namespaceName))
					k8sGardenRuntimeClient.EXPECT().List(gomock.Any(), partialQuotaMetaList, client.InNamespace(namespaceName))

					expectStaleMarking(k8sGardenRuntimeClient, mockStatusWriter, project, &staleSinceTimestamp, nil, fakeClock)

					_, result := reconciler.Reconcile(ctx, request)
					Expect(result).To(Succeed())
				})

				It("should set the auto delete timestamp because stale grace period is exceeded", func() {
					var (
						staleSinceTimestamp      = metav1.Time{Time: fakeClock.Now().Add(-24 * time.Hour * time.Duration(staleGracePeriodDays))}
						staleAutoDeleteTimestamp = metav1.Time{Time: staleSinceTimestamp.Add(24 * time.Hour * time.Duration(staleExpirationTimeDays))}
					)
					project.Status.StaleSinceTimestamp = &staleSinceTimestamp

					k8sGardenRuntimeClient.EXPECT().List(gomock.Any(), partialShootMetaList, client.InNamespace(namespaceName), client.Limit(1))
					k8sGardenRuntimeClient.EXPECT().List(gomock.Any(), partialBackupEntryMetaList, client.InNamespace(namespaceName), client.Limit(1))
					k8sGardenRuntimeClient.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&corev1.SecretList{}), client.InNamespace(namespaceName))
					k8sGardenRuntimeClient.EXPECT().List(gomock.Any(), partialQuotaMetaList, client.InNamespace(namespaceName))

					expectStaleMarking(k8sGardenRuntimeClient, mockStatusWriter, project, &staleSinceTimestamp, &staleAutoDeleteTimestamp, fakeClock)

					_, result := reconciler.Reconcile(ctx, request)
					Expect(result).To(Succeed())
				})

				It("should delete the project if the auto delete timestamp is exceeded", func() {
					var (
						staleSinceTimestamp      = metav1.Time{Time: fakeClock.Now().Add(-24 * time.Hour * 3 * time.Duration(staleExpirationTimeDays))}
						staleAutoDeleteTimestamp = metav1.Time{Time: fakeClock.Now()}
					)

					project.Status.StaleSinceTimestamp = &staleSinceTimestamp
					project.Status.StaleAutoDeleteTimestamp = &staleAutoDeleteTimestamp

					k8sGardenRuntimeClient.EXPECT().List(gomock.Any(), partialShootMetaList, client.InNamespace(namespaceName), client.Limit(1))
					k8sGardenRuntimeClient.EXPECT().List(gomock.Any(), partialBackupEntryMetaList, client.InNamespace(namespaceName), client.Limit(1))
					k8sGardenRuntimeClient.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&corev1.SecretList{}), client.InNamespace(namespaceName))
					k8sGardenRuntimeClient.EXPECT().List(gomock.Any(), partialQuotaMetaList, client.InNamespace(namespaceName))

					expectStaleMarking(k8sGardenRuntimeClient, mockStatusWriter, project, &staleSinceTimestamp, &staleAutoDeleteTimestamp, fakeClock)

					defer test.WithVar(&gardenerutils.TimeNow, func() time.Time {
						return time.Date(1, 1, minimumLifetimeDays+1, 1, 0, 0, 0, time.UTC)
					})()

					projectCopy := project.DeepCopy()
					projectCopy.Annotations = map[string]string{
						gardenerutils.ConfirmationDeletion: "true",
						v1beta1constants.GardenerTimestamp: gardenerutils.TimeNow().UTC().Format(time.RFC3339Nano),
					}
					k8sGardenRuntimeClient.EXPECT().Patch(gomock.Any(), projectCopy, gomock.Any())
					k8sGardenRuntimeClient.EXPECT().Delete(gomock.Any(), projectCopy)

					_, result := reconciler.Reconcile(ctx, request)
					Expect(result).To(Succeed())
				})
			})
		})
	})
})

func expectNonStaleMarking(k8sGardenRuntimeClient *mockclient.MockClient, mockStatusWriter *mockclient.MockStatusWriter, project *gardencorev1beta1.Project) {
	k8sGardenRuntimeClient.EXPECT().Status().Return(mockStatusWriter)

	projectPatched := project.DeepCopy()
	projectPatched.Status.StaleSinceTimestamp = nil
	projectPatched.Status.StaleAutoDeleteTimestamp = nil

	test.EXPECTStatusPatch(gomock.Any(), mockStatusWriter, projectPatched, project, types.MergePatchType)
}

func expectStaleMarking(k8sGardenRuntimeClient *mockclient.MockClient, mockStatusWriter *mockclient.MockStatusWriter, project *gardencorev1beta1.Project, staleSinceTimestamp, staleAutoDeleteTimestamp *metav1.Time, fakeClock *testing.FakeClock) {
	k8sGardenRuntimeClient.EXPECT().Status().Return(mockStatusWriter)

	projectPatched := project.DeepCopy()
	if staleSinceTimestamp == nil {
		projectPatched.Status.StaleSinceTimestamp = &metav1.Time{Time: fakeClock.Now()}
	} else {
		projectPatched.Status.StaleSinceTimestamp = staleSinceTimestamp
	}
	projectPatched.Status.StaleAutoDeleteTimestamp = staleAutoDeleteTimestamp

	test.EXPECTStatusPatch(gomock.Any(), mockStatusWriter, projectPatched, project, types.MergePatchType)
}
