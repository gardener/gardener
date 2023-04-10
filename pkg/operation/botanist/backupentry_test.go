// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package botanist_test

import (
	"context"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/operation"
	. "github.com/gardener/gardener/pkg/operation/botanist"
	mockbackupentry "github.com/gardener/gardener/pkg/operation/botanist/component/backupentry/mock"
	"github.com/gardener/gardener/pkg/operation/seed"
	shootpkg "github.com/gardener/gardener/pkg/operation/shoot"
)

var _ = Describe("BackupEntry", func() {

	var (
		ctrl *gomock.Controller
		ctx  = context.TODO()

		botanist          *Botanist
		sourceBackupEntry *mockbackupentry.MockInterface
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())

		sourceBackupEntry = mockbackupentry.NewMockInterface(ctrl)
		botanist = &Botanist{
			Operation: &operation.Operation{
				Shoot: &shootpkg.Shoot{
					Components: &shootpkg.Components{
						SourceBackupEntry: sourceBackupEntry,
					},
				},
			},
		}

		botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "bar",
				Namespace: "foo",
			},
			Status: gardencorev1beta1.ShootStatus{
				LastOperation: &gardencorev1beta1.LastOperation{
					Type: gardencorev1beta1.LastOperationTypeRestore,
				},
			},
		})
		botanist.Seed = &seed.Seed{}
		botanist.Seed.SetInfo(&gardencorev1beta1.Seed{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "seed",
				Namespace: "garden",
			},
			Spec: gardencorev1beta1.SeedSpec{
				Backup: &gardencorev1beta1.SeedBackup{
					Provider: "gcp",
				},
			},
		})
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#DestroySourceBackupEntry", func() {
		It("shouldn't destroy the SourceBackupEntry component when Seed backup is not enabled", func() {
			botanist.Seed.SetInfo(&gardencorev1beta1.Seed{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "seed",
					Namespace: "garden",
				},
				Spec: gardencorev1beta1.SeedSpec{
					Backup: nil,
				},
			})

			Expect(botanist.DestroySourceBackupEntry(ctx)).To(Succeed())
		})

		It("shouldn't destroy the SourceBackupEntry component when Shoot is not in restore phase", func() {
			botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "bar",
					Namespace: "foo",
				},
				Status: gardencorev1beta1.ShootStatus{
					LastOperation: &gardencorev1beta1.LastOperation{
						Type: gardencorev1beta1.LastOperationTypeReconcile,
					},
				},
			})

			Expect(botanist.DestroySourceBackupEntry(ctx)).To(Succeed())
		})

		It("should set force-deletion annotation and destroy the SourceBackupEntry component", func() {
			sourceBackupEntry.EXPECT().SetForceDeletionAnnotation(ctx)
			sourceBackupEntry.EXPECT().Destroy(ctx)

			Expect(botanist.DestroySourceBackupEntry(ctx)).To(Succeed())
		})
	})
})
