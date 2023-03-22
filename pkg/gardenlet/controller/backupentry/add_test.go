// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package backupentry_test

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	. "github.com/gardener/gardener/pkg/gardenlet/controller/backupentry"
)

var _ = Describe("Add", func() {
	var (
		reconciler       *Reconciler
		backupEntry      *gardencorev1beta1.BackupEntry
		projectName      = "dev"
		shootName        = "shoot"
		projectNamespace string
		shootTechnicalID string
		entryName        string
	)

	BeforeEach(func() {
		shootTechnicalID = fmt.Sprintf("shoot--%s--%s", projectName, shootName)
		entryName = shootTechnicalID + "--shootUID"
		projectNamespace = "garden-" + projectName

		reconciler = &Reconciler{
			SeedName: "seed",
		}

		backupEntry = &gardencorev1beta1.BackupEntry{
			ObjectMeta: metav1.ObjectMeta{
				Name:      entryName,
				Namespace: projectNamespace,
			},
			Spec: gardencorev1beta1.BackupEntrySpec{
				SeedName: pointer.String("seed"),
			},
		}
	})

	Describe("#MapExtensionBackupEntryToBackupEntry", func() {
		var (
			ctx                  = context.TODO()
			log                  = logr.Discard()
			extensionBackupEntry *extensionsv1alpha1.BackupEntry
			cluster              *extensionsv1alpha1.Cluster
			fakeClient           client.Client
		)

		BeforeEach(func() {
			extensionBackupEntry = &extensionsv1alpha1.BackupEntry{
				ObjectMeta: metav1.ObjectMeta{
					Name: entryName,
				},
			}

			cluster = &extensionsv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: shootTechnicalID,
				},
				Spec: extensionsv1alpha1.ClusterSpec{
					Shoot: runtime.RawExtension{
						Object: &gardencorev1beta1.Shoot{
							TypeMeta: metav1.TypeMeta{
								APIVersion: "core.gardener.cloud/v1beta1",
								Kind:       "Shoot",
							},
							ObjectMeta: metav1.ObjectMeta{
								Name:      shootName,
								Namespace: projectNamespace,
							},
						},
					},
				},
			}

			testScheme := runtime.NewScheme()
			Expect(extensionsv1alpha1.AddToScheme(testScheme)).To(Succeed())

			fakeClient = fakeclient.NewClientBuilder().WithScheme(testScheme).Build()
			reconciler = &Reconciler{
				SeedName:   "seed",
				SeedClient: fakeClient,
			}
		})

		It("should return a request with the core.gardener.cloud/v1beta1.BackupEntry name and namespace", func() {
			Expect(fakeClient.Create(ctx, cluster)).To(Succeed())
			Expect(reconciler.MapExtensionBackupEntryToCoreBackupEntry(ctx, log, nil, extensionBackupEntry)).To(ConsistOf(
				reconcile.Request{NamespacedName: types.NamespacedName{Name: backupEntry.Name, Namespace: backupEntry.Namespace}},
			))
		})

		It("should return nil if the object has a deletion timestamp", func() {
			extensionBackupEntry.DeletionTimestamp = &metav1.Time{Time: time.Now()}

			Expect(reconciler.MapExtensionBackupEntryToCoreBackupEntry(ctx, log, nil, extensionBackupEntry)).To(BeNil())
		})

		It("should return nil when cluster is not found", func() {
			Expect(reconciler.MapExtensionBackupEntryToCoreBackupEntry(ctx, log, nil, extensionBackupEntry)).To(BeNil())
		})

		It("should return nil when shoot is not present in the cluster", func() {
			cluster.Spec.Shoot.Object = nil
			Expect(fakeClient.Create(ctx, cluster)).To(Succeed())

			Expect(reconciler.MapExtensionBackupEntryToCoreBackupEntry(ctx, log, nil, extensionBackupEntry)).To(BeNil())
		})
	})
})
