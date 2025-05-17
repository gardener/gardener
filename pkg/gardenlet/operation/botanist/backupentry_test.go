// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	mockbackupentry "github.com/gardener/gardener/pkg/component/garden/backupentry/mock"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	. "github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
	"github.com/gardener/gardener/pkg/gardenlet/operation/seed"
	shootpkg "github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
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
				Backup: &gardencorev1beta1.Backup{
					Provider: "gcp",
				},
			},
		})
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#DestroySourceBackupEntry", func() {
		It("should set force-deletion annotation and destroy the SourceBackupEntry component", func() {
			sourceBackupEntry.EXPECT().SetForceDeletionAnnotation(ctx)
			sourceBackupEntry.EXPECT().Destroy(ctx)

			Expect(botanist.DestroySourceBackupEntry(ctx)).To(Succeed())
		})
	})
})
