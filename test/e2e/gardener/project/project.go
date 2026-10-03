// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package project

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	. "github.com/gardener/gardener/test/e2e/gardener"
)

// ItShouldCreateProject creates the project
func ItShouldCreateProject(tc *ProjectContext) {
	GinkgoHelper()

	It("Create Project", func(ctx SpecContext) {
		Eventually(ctx, func() error {
			if err := tc.GardenClient.Create(ctx, tc.Project); !apierrors.IsAlreadyExists(err) {
				return err
			}

			return StopTrying("project already exists")
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))
}

// ItShouldDeleteProject deletes the project
func ItShouldDeleteProject(tc *ProjectContext) {
	GinkgoHelper()

	It("Delete Project", func(ctx SpecContext) {
		Eventually(ctx, func(g Gomega) {
			g.Expect(gardenerutils.ConfirmDeletion(ctx, tc.GardenClient, tc.Project)).To(Succeed())
			g.Expect(tc.GardenClient.Delete(ctx, tc.Project)).To(Succeed())
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))
}

// ItShouldWaitForProjectToBeDeleted waits for the project to be gone
func ItShouldWaitForProjectToBeDeleted(tc *ProjectContext) {
	GinkgoHelper()

	It("Wait for Project to be deleted", func(ctx SpecContext) {
		Eventually(ctx, func() error {
			err := tc.GardenKomega.Get(tc.Project)()
			if err == nil {
				tc.Log.Info("Waiting for deletion", "phase", tc.Project.Status.Phase)
			}
			return err
		}).WithPolling(30 * time.Second).Should(BeNotFoundError())

		tc.Log.Info("Project has been deleted")
	}, SpecTimeout(5*time.Minute))
}

// ItShouldWaitForProjectToBeReconciledAndReady waits for the project to be reconciled successfully and ready.
func ItShouldWaitForProjectToBeReconciledAndReady(tc *ProjectContext) {
	GinkgoHelper()

	It("Wait for Project to be reconciled", func(ctx SpecContext) {
		Eventually(ctx, func(g Gomega) {
			g.Expect(tc.GardenKomega.Get(tc.Project)()).To(Succeed())
			g.Expect(tc.Project.Status.ObservedGeneration).To(Equal(tc.Project.Generation))
			g.Expect(tc.Project.Status.Phase).To(Equal(gardencorev1beta1.ProjectReady))
		}).WithPolling(5 * time.Second).Should(Succeed())

		tc.Log.Info("Project has been reconciled and is ready")
	}, SpecTimeout(5*time.Minute))
}
