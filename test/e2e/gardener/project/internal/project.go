// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package internal

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
func ItShouldCreateProject(s *ProjectContext) {
	GinkgoHelper()

	It("Create Project", func(ctx SpecContext) {
		Eventually(ctx, func() error {
			if err := s.GardenClient.Create(ctx, s.Project); !apierrors.IsAlreadyExists(err) {
				return err
			}

			return StopTrying("project already exists")
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))
}

// ItShouldDeleteProject deletes the project
func ItShouldDeleteProject(s *ProjectContext) {
	GinkgoHelper()

	It("Delete Project", func(ctx SpecContext) {
		Eventually(ctx, func(g Gomega) {
			g.Expect(gardenerutils.ConfirmDeletion(ctx, s.GardenClient, s.Project)).To(Succeed())
			g.Expect(s.GardenClient.Delete(ctx, s.Project)).To(Succeed())
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))
}

// ItShouldWaitForProjectToBeDeleted waits for the project to be gone
func ItShouldWaitForProjectToBeDeleted(s *ProjectContext) {
	GinkgoHelper()

	It("Wait for Project to be deleted", func(ctx SpecContext) {
		Eventually(ctx, func() error {
			err := s.GardenKomega.Get(s.Project)()
			if err == nil {
				s.Log.Info("Waiting for deletion", "phase", s.Project.Status.Phase)
			}
			return err
		}).WithPolling(30 * time.Second).Should(BeNotFoundError())

		s.Log.Info("Project has been deleted")
	}, SpecTimeout(5*time.Minute))
}

// ItShouldWaitForProjectToBeReconciledAndReady waits for the project to be reconciled successfully and ready.
func ItShouldWaitForProjectToBeReconciledAndReady(s *ProjectContext) {
	GinkgoHelper()

	It("Wait for Project to be reconciled", func(ctx SpecContext) {
		Eventually(ctx, func(g Gomega) {
			g.Expect(s.GardenKomega.Get(s.Project)()).To(Succeed())
			g.Expect(s.Project.Status.ObservedGeneration).To(Equal(s.Project.Generation))
			g.Expect(s.Project.Status.Phase).To(Equal(gardencorev1beta1.ProjectReady))
		}).WithPolling(5 * time.Second).Should(Succeed())

		s.Log.Info("Project has been reconciled and is ready")
	}, SpecTimeout(5*time.Minute))
}
