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
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
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
	kubecoreinformers "k8s.io/client-go/informers"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("ProjectStaleControl", func() {
	Describe("projectStaleReconciler", func() {
		var (
			ctx = context.TODO()

			ctrl                         *gomock.Controller
			k8sGardenRuntimeClient       *mockclient.MockClient
			k8sGardenRuntimeStatusWriter *mockclient.MockStatusWriter
			gardenCoreInformerFactory    gardencoreinformers.SharedInformerFactory
			kubeCoreInformerFactory      kubecoreinformers.SharedInformerFactory

			oldTimenowFunc func() time.Time

			projectName       = "foo"
			namespaceName     = "garden-foo"
			secretName        = "secret"
			secretBindingName = "secretbinding"
			quotaName         = "quota"

			minimumLifetimeDays     = 5
			staleGracePeriodDays    = 10
			staleExpirationTimeDays = 15
			staleSyncPeriod         = metav1.Duration{Duration: time.Second}

			project       *gardencorev1beta1.Project
			namespace     *corev1.Namespace
			shoot         *gardencorev1beta1.Shoot
			plant         *gardencorev1beta1.Plant
			backupEntry   *gardencorev1beta1.BackupEntry
			quota         *gardencorev1beta1.Quota
			secret        *corev1.Secret
			secretBinding *gardencorev1beta1.SecretBinding
			cfg           *config.ProjectControllerConfiguration
			request       reconcile.Request

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
			k8sGardenRuntimeStatusWriter = mockclient.NewMockStatusWriter(ctrl)
			gardenCoreInformerFactory = gardencoreinformers.NewSharedInformerFactory(nil, 0)
			kubeCoreInformerFactory = kubecoreinformers.NewSharedInformerFactory(nil, 0)

			logger.Logger = logger.NewNopLogger()

			project = &gardencorev1beta1.Project{
				ObjectMeta: metav1.ObjectMeta{Name: projectName},
				Spec:       gardencorev1beta1.ProjectSpec{Namespace: &namespaceName},
			}
			namespace = &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{Name: namespaceName},
			}
			shoot = &gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{Namespace: namespaceName},
				Spec:       gardencorev1beta1.ShootSpec{SecretBindingName: secretBindingName},
			}
			plant = &gardencorev1beta1.Plant{
				ObjectMeta: metav1.ObjectMeta{Namespace: namespaceName},
			}
			backupEntry = &gardencorev1beta1.BackupEntry{
				ObjectMeta: metav1.ObjectMeta{Namespace: namespaceName},
			}
			quota = &gardencorev1beta1.Quota{
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

			reconciler = NewProjectStaleReconciler(
				logger.NewNopLogger(),
				cfg,
				k8sGardenRuntimeClient,
				kubeCoreInformerFactory.Core().V1().Secrets().Lister(),
			)

			k8sGardenRuntimeClient.EXPECT().Get(ctx, kutil.Key(project.Name), gomock.AssignableToTypeOf(&gardencorev1beta1.Project{}))
			Expect(kubeCoreInformerFactory.Core().V1().Namespaces().Informer().GetStore().Add(namespace)).To(Succeed())
		})

		Describe("#ReconcileStaleProject", func() {
			It("should do nothing because the project has no namespace", func() {
				project.Spec.Namespace = nil

				_, result := reconciler.Reconcile(ctx, request)
				Expect(result).To(Succeed())
			})

			It("should mark the project as 'not stale' because the namespace has the skip-stale-check annotation", func() {
				nowFunc := func() metav1.Time {
					return metav1.Time{Time: time.Date(100, 1, 1, 0, 0, 0, 0, time.UTC)}
				}
				defer test.WithVar(&NowFunc, nowFunc)

				namespace.Annotations = map[string]string{v1beta1constants.ProjectSkipStaleCheck: "true"}
				Expect(kubeCoreInformerFactory.Core().V1().Namespaces().Informer().GetStore().Update(namespace)).To(Succeed())

				expectNonStaleMarking(k8sGardenRuntimeClient, k8sGardenRuntimeStatusWriter, project, nowFunc)

				_, result := reconciler.Reconcile(ctx, request)
				Expect(result).To(Succeed())
			})

			It("should mark the project as 'not stale' because it is younger than the configured MinimumLifetimeDays", func() {
				nowFunc := func() metav1.Time {
					return metav1.Time{Time: time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC)}
				}
				defer test.WithVar(&NowFunc, nowFunc)

				project.CreationTimestamp = metav1.Time{Time: time.Date(1, 1, minimumLifetimeDays-1, 0, 0, 0, 0, time.UTC)}

				expectNonStaleMarking(k8sGardenRuntimeClient, k8sGardenRuntimeStatusWriter, project, nowFunc)

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
						defer test.WithVar(&NowFunc, nowFunc)

						Expect(gardenCoreInformerFactory.Core().V1beta1().Shoots().Informer().GetStore().Add(shoot)).To(Succeed())

						expectNonStaleMarking(k8sGardenRuntimeClient, k8sGardenRuntimeStatusWriter, project, nowFunc)

						_, result := reconciler.Reconcile(ctx, request)
						Expect(result).To(Succeed())
					})

					It("has plants", func() {
						defer test.WithVar(&NowFunc, nowFunc)

						Expect(gardenCoreInformerFactory.Core().V1beta1().Plants().Informer().GetStore().Add(plant)).To(Succeed())

						expectNonStaleMarking(k8sGardenRuntimeClient, k8sGardenRuntimeStatusWriter, project, nowFunc)

						_, result := reconciler.Reconcile(ctx, request)
						Expect(result).To(Succeed())
					})

					It("has backupentries", func() {
						defer test.WithVar(&NowFunc, nowFunc)

						Expect(gardenCoreInformerFactory.Core().V1beta1().BackupEntries().Informer().GetStore().Add(backupEntry)).To(Succeed())

						expectNonStaleMarking(k8sGardenRuntimeClient, k8sGardenRuntimeStatusWriter, project, nowFunc)

						_, result := reconciler.Reconcile(ctx, request)
						Expect(result).To(Succeed())
					})

					It("has secrets that are used by shoots in the same namespace", func() {
						defer test.WithVar(&NowFunc, nowFunc)

						Expect(kubeCoreInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(secret)).To(Succeed())
						Expect(gardenCoreInformerFactory.Core().V1beta1().SecretBindings().Informer().GetStore().Add(secretBinding)).To(Succeed())
						Expect(gardenCoreInformerFactory.Core().V1beta1().Shoots().Informer().GetStore().Add(shoot)).To(Succeed())

						expectNonStaleMarking(k8sGardenRuntimeClient, k8sGardenRuntimeStatusWriter, project, nowFunc)

						_, result := reconciler.Reconcile(ctx, request)
						Expect(result).To(Succeed())
					})

					It("has secrets that are used by shoots in another namespace", func() {
						defer test.WithVar(&NowFunc, nowFunc)

						otherNamespace := namespaceName + "other"
						secretBinding.Namespace = otherNamespace
						shoot.Namespace = otherNamespace

						Expect(kubeCoreInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(secret)).To(Succeed())
						Expect(gardenCoreInformerFactory.Core().V1beta1().SecretBindings().Informer().GetStore().Add(secretBinding)).To(Succeed())
						Expect(gardenCoreInformerFactory.Core().V1beta1().Shoots().Informer().GetStore().Add(shoot)).To(Succeed())

						expectNonStaleMarking(k8sGardenRuntimeClient, k8sGardenRuntimeStatusWriter, project, nowFunc)

						_, result := reconciler.Reconcile(ctx, request)
						Expect(result).To(Succeed())
					})

					It("has quotas that are used by shoots in the same namespace", func() {
						defer test.WithVar(&NowFunc, nowFunc)

						Expect(gardenCoreInformerFactory.Core().V1beta1().Quotas().Informer().GetStore().Add(quota)).To(Succeed())
						Expect(gardenCoreInformerFactory.Core().V1beta1().SecretBindings().Informer().GetStore().Add(secretBinding)).To(Succeed())
						Expect(gardenCoreInformerFactory.Core().V1beta1().Shoots().Informer().GetStore().Add(shoot)).To(Succeed())

						expectNonStaleMarking(k8sGardenRuntimeClient, k8sGardenRuntimeStatusWriter, project, nowFunc)

						_, result := reconciler.Reconcile(ctx, request)
						Expect(result).To(Succeed())
					})

					It("has quotas that are used by shoots in another namespace", func() {
						defer test.WithVar(&NowFunc, nowFunc)

						otherNamespace := namespaceName + "other"
						secretBinding.Namespace = otherNamespace
						shoot.Namespace = otherNamespace

						Expect(gardenCoreInformerFactory.Core().V1beta1().Quotas().Informer().GetStore().Add(quota)).To(Succeed())
						Expect(gardenCoreInformerFactory.Core().V1beta1().SecretBindings().Informer().GetStore().Add(secretBinding)).To(Succeed())
						Expect(gardenCoreInformerFactory.Core().V1beta1().Shoots().Informer().GetStore().Add(shoot)).To(Succeed())

						expectNonStaleMarking(k8sGardenRuntimeClient, k8sGardenRuntimeStatusWriter, project, nowFunc)

						_, result := reconciler.Reconcile(ctx, request)
						Expect(result).To(Succeed())
					})

				})

				Describe("project should be marked as stale", func() {
					It("has secrets that are unused", func() {
						defer test.WithVar(&NowFunc, nowFunc)

						Expect(kubeCoreInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(secret)).To(Succeed())
						Expect(gardenCoreInformerFactory.Core().V1beta1().SecretBindings().Informer().GetStore().Add(secretBinding)).To(Succeed())

						expectStaleMarking(k8sGardenRuntimeClient, k8sGardenRuntimeStatusWriter, project, nil, nil, nowFunc)

						_, result := reconciler.Reconcile(ctx, request)
						Expect(result).To(Succeed())
					})

					It("has quotas that are unused", func() {
						defer test.WithVar(&NowFunc, nowFunc)

						Expect(gardenCoreInformerFactory.Core().V1beta1().Quotas().Informer().GetStore().Add(quota)).To(Succeed())
						Expect(gardenCoreInformerFactory.Core().V1beta1().SecretBindings().Informer().GetStore().Add(secretBinding)).To(Succeed())

						expectStaleMarking(k8sGardenRuntimeClient, k8sGardenRuntimeStatusWriter, project, nil, nil, nowFunc)

						_, result := reconciler.Reconcile(ctx, request)
						Expect(result).To(Succeed())
					})

					It("it is not used", func() {
						defer test.WithVar(&NowFunc, nowFunc)

						expectStaleMarking(k8sGardenRuntimeClient, k8sGardenRuntimeStatusWriter, project, nil, nil, nowFunc)

						_, result := reconciler.Reconcile(ctx, request)
						Expect(result).To(Succeed())
					})

					It("should not set the auto delete timestamp because stale grace period is not exceeded", func() {
						defer test.WithVar(&NowFunc, nowFunc)

						staleSinceTimestamp := metav1.Time{Time: nowFunc().Add(-24*time.Hour*time.Duration(staleGracePeriodDays) + time.Hour)}
						project.Status.StaleSinceTimestamp = &staleSinceTimestamp

						expectStaleMarking(k8sGardenRuntimeClient, k8sGardenRuntimeStatusWriter, project, &staleSinceTimestamp, nil, nowFunc)

						_, result := reconciler.Reconcile(ctx, request)
						Expect(result).To(Succeed())
					})

					It("should set the auto delete timestamp because stale grace period is exceeded", func() {
						defer test.WithVar(&NowFunc, nowFunc)

						var (
							staleSinceTimestamp      = metav1.Time{Time: nowFunc().Add(-24 * time.Hour * time.Duration(staleGracePeriodDays))}
							staleAutoDeleteTimestamp = metav1.Time{Time: staleSinceTimestamp.Add(24 * time.Hour * time.Duration(staleExpirationTimeDays))}
						)
						project.Status.StaleSinceTimestamp = &staleSinceTimestamp

						expectStaleMarking(k8sGardenRuntimeClient, k8sGardenRuntimeStatusWriter, project, &staleSinceTimestamp, &staleAutoDeleteTimestamp, nowFunc)

						_, result := reconciler.Reconcile(ctx, request)
						Expect(result).To(Succeed())
					})

					It("should extend the project's stale grace period", func() {
						defer test.WithVar(&NowFunc, nowFunc)

						var (
							staleSinceTimestamp      = metav1.Time{Time: nowFunc().Add(-24 * time.Hour * time.Duration(staleGracePeriodDays))}
							staleAutoDeleteTimestamp = metav1.Time{Time: staleSinceTimestamp.Add(24 * time.Hour * time.Duration(staleExpirationTimeDays))}
							oldTime                  = metav1.Time{Time: staleSinceTimestamp.Add(-24*time.Hour*time.Duration(staleExpirationTimeDays) + time.Hour)}
						)

						project.Status.StaleSinceTimestamp = &oldTime
						project.Status.StaleAutoDeleteTimestamp = &oldTime

						expectStaleMarking(k8sGardenRuntimeClient, k8sGardenRuntimeStatusWriter, project, &staleSinceTimestamp, &staleAutoDeleteTimestamp, nowFunc)

						_, result := reconciler.Reconcile(ctx, request)
						Expect(result).To(Succeed())
					})

					It("should delete the project if the auto delete timestamp is exceeded", func() {
						defer test.WithVar(&NowFunc, nowFunc)

						var (
							staleSinceTimestamp      = metav1.Time{Time: nowFunc().Add(-24 * time.Hour * 3 * time.Duration(staleExpirationTimeDays))}
							staleAutoDeleteTimestamp = nowFunc()
						)
						project.Status.StaleSinceTimestamp = &staleSinceTimestamp
						project.Status.StaleAutoDeleteTimestamp = &staleAutoDeleteTimestamp

						expectStaleMarking(k8sGardenRuntimeClient, k8sGardenRuntimeStatusWriter, project, &staleSinceTimestamp, &staleAutoDeleteTimestamp, nowFunc)

						gutil.TimeNow = func() time.Time {
							return time.Date(1, 1, minimumLifetimeDays+1, 1, 0, 0, 0, time.UTC)
						}

						k8sGardenRuntimeClient.EXPECT().Get(context.TODO(), kutil.Key(projectName), project)
						project.Annotations = map[string]string{
							gutil.ConfirmationDeletion:         "true",
							v1beta1constants.GardenerTimestamp: gutil.TimeNow().UTC().String(),
						}
						k8sGardenRuntimeClient.EXPECT().Update(context.TODO(), project)
						k8sGardenRuntimeClient.EXPECT().Delete(context.TODO(), project)

						_, result := reconciler.Reconcile(ctx, request)
						Expect(result).To(Succeed())
					})
				})
			})
		})
	})
})

func expectNonStaleMarking(k8sGardenRuntimeClient *mockclient.MockClient, k8sGardenRuntimeStatusWriter *mockclient.MockStatusWriter, project *gardencorev1beta1.Project, nowFunc func() metav1.Time) {
	k8sGardenRuntimeClient.EXPECT().Get(context.TODO(), kutil.Key(project.Name), project).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *gardencorev1beta1.Project) error {
		someTime := nowFunc()
		obj.Status.StaleSinceTimestamp = &someTime
		obj.Status.StaleAutoDeleteTimestamp = &someTime
		return nil
	})

	project.Status.StaleSinceTimestamp = nil
	project.Status.StaleAutoDeleteTimestamp = nil

	k8sGardenRuntimeClient.EXPECT().Status().Return(k8sGardenRuntimeStatusWriter)
	k8sGardenRuntimeStatusWriter.EXPECT().Update(context.TODO(), project)
}

func expectStaleMarking(k8sGardenRuntimeClient *mockclient.MockClient, k8sGardenRuntimeStatusWriter *mockclient.MockStatusWriter, project *gardencorev1beta1.Project, staleSinceTimestamp, staleAutoDeleteTimestamp *metav1.Time, nowFunc func() metav1.Time) {
	k8sGardenRuntimeClient.EXPECT().Get(context.TODO(), kutil.Key(project.Name), project).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *gardencorev1beta1.Project) error {
		project.DeepCopyInto(obj)
		return nil
	})

	if staleSinceTimestamp == nil {
		now := nowFunc()
		project.Status.StaleSinceTimestamp = &now
	} else {
		project.Status.StaleSinceTimestamp = staleSinceTimestamp
	}

	project.Status.StaleAutoDeleteTimestamp = staleAutoDeleteTimestamp

	k8sGardenRuntimeClient.EXPECT().Status().Return(k8sGardenRuntimeStatusWriter)
	k8sGardenRuntimeStatusWriter.EXPECT().Update(context.TODO(), project)
}
