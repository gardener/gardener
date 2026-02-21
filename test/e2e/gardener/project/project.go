// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package project

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	. "github.com/gardener/gardener/test/e2e/gardener"
)

// CreateProject creates the project.
func CreateProject(ctx context.Context, s *ProjectContext) {
	GinkgoHelper()

	Eventually(ctx, func() error {
		if err := s.GardenClient.Create(ctx, s.Project); !apierrors.IsAlreadyExists(err) {
			return err
		}

		return StopTrying("project already exists")
	}).Should(Succeed())
}

// DeleteProject deletes the project.
func DeleteProject(ctx context.Context, s *ProjectContext) {
	GinkgoHelper()

	Eventually(ctx, func(g Gomega) {
		g.Expect(gardenerutils.ConfirmDeletion(ctx, s.GardenClient, s.Project)).To(Succeed())
		g.Expect(s.GardenClient.Delete(ctx, s.Project)).To(Succeed())
	}).Should(Succeed())
}

// WaitForProjectToBeDeleted waits for the project to be gone.
func WaitForProjectToBeDeleted(ctx context.Context, s *ProjectContext) {
	GinkgoHelper()

	Eventually(ctx, func() error {
		err := s.GardenKomega.Get(s.Project)()
		if err == nil {
			s.Log.Info("Waiting for deletion", "phase", s.Project.Status.Phase)
		}
		return err
	}).WithPolling(30 * time.Second).Should(BeNotFoundError())

	s.Log.Info("Project has been deleted")
}

// WaitForProjectToBeReconciledAndReady waits for the project to be reconciled successfully and ready.
func WaitForProjectToBeReconciledAndReady(ctx context.Context, s *ProjectContext) {
	GinkgoHelper()

	Eventually(ctx, func(g Gomega) {
		g.Expect(s.GardenKomega.Get(s.Project)()).To(Succeed())
		g.Expect(s.Project.Status.ObservedGeneration).To(Equal(s.Project.Generation))
		g.Expect(s.Project.Status.Phase).To(Equal(gardencorev1beta1.ProjectReady))
	}).WithPolling(5 * time.Second).Should(Succeed())

	s.Log.Info("Project has been reconciled and is ready")
}
