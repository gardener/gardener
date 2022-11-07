// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	. "github.com/gardener/gardener/pkg/gardenlet/controller/backupentry/backupentry"
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

	Describe("#SeedNamePredicate", func() {
		var p predicate.Predicate

		BeforeEach(func() {
			p = reconciler.SeedNamePredicate()
		})

		It("should return false if the specified object is not a BackupEntry", func() {
			Expect(p.Create(event.CreateEvent{Object: &corev1.Secret{}})).To(BeFalse())
			Expect(p.Update(event.UpdateEvent{ObjectNew: &corev1.Secret{}})).To(BeFalse())
			Expect(p.Delete(event.DeleteEvent{Object: &corev1.Secret{}})).To(BeFalse())
			Expect(p.Generic(event.GenericEvent{Object: &corev1.Secret{}})).To(BeFalse())
		})

		DescribeTable("filter BackupEntry by seedName",
			func(specSeedName, statusSeedName *string, match gomegatypes.GomegaMatcher) {
				backupEntry.Spec.SeedName = specSeedName
				backupEntry.Status.SeedName = statusSeedName

				Expect(p.Create(event.CreateEvent{Object: backupEntry})).To(match)
				Expect(p.Update(event.UpdateEvent{ObjectNew: backupEntry})).To(match)
				Expect(p.Delete(event.DeleteEvent{Object: backupEntry})).To(match)
				Expect(p.Generic(event.GenericEvent{Object: backupEntry})).To(match)
			},

			Entry("BackupEntry.Spec.SeedName and BackupEntry.Status.SeedName are nil", nil, nil, BeFalse()),
			Entry("BackupEntry.Spec.SeedName does not match and BackupEntry.Status.SeedName is nil", pointer.String("otherSeed"), nil, BeFalse()),
			Entry("BackupEntry.Spec.SeedName and BackupEntry.Status.SeedName do not match", pointer.String("otherSeed"), pointer.String("otherSeed"), BeFalse()),
			Entry("BackupEntry.Spec.SeedName is nil but BackupEntry.Status.SeedName matches", nil, pointer.String("seed"), BeFalse()),
			Entry("BackupEntry.Spec.SeedName matches and BackupEntry.Status.SeedName is nil", pointer.String("seed"), nil, BeTrue()),
			Entry("BackupEntry.Spec.SeedName does not match but BackupEntry.Status.SeedName matches", pointer.String("otherSeed"), pointer.String("seed"), BeTrue()),
		)
	})

	Describe("#MapExtensionBackupEntryToBackupEntry", func() {
		var (
			ctx                  = context.TODO()
			log                  = logr.Discard()
			extensionBackupEntry *extensionsv1alpha1.BackupEntry
			project              *gardencorev1beta1.Project
			fakeClient           client.Client
		)

		BeforeEach(func() {
			extensionBackupEntry = &extensionsv1alpha1.BackupEntry{
				ObjectMeta: metav1.ObjectMeta{
					Name: entryName,
				},
			}

			project = &gardencorev1beta1.Project{
				ObjectMeta: metav1.ObjectMeta{
					Name: projectName,
				},
				Spec: gardencorev1beta1.ProjectSpec{
					Namespace: &projectNamespace,
				},
			}

			fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).Build()
			reconciler = &Reconciler{
				SeedName:     "seed",
				GardenClient: fakeClient,
			}
		})

		It("should return a request with the core.gardener.cloud/v1beta1.BackupEntry name and namespace", func() {
			Expect(fakeClient.Create(ctx, project)).To(Succeed())
			Expect(reconciler.MapExtensionBackupEntryToCoreBackupEntry(ctx, log, nil, extensionBackupEntry)).To(ConsistOf(
				reconcile.Request{NamespacedName: types.NamespacedName{Name: backupEntry.Name, Namespace: backupEntry.Namespace}},
			))
		})

		It("should return nil when project is not found", func() {
			Expect(reconciler.MapExtensionBackupEntryToCoreBackupEntry(ctx, log, nil, extensionBackupEntry)).To(BeNil())
		})
	})
})
