// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package zerodowntimevalidator

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	. "github.com/gardener/gardener/test/e2e/gardener"
	shootupdatesuite "github.com/gardener/gardener/test/utils/shoots/update"
	"github.com/gardener/gardener/test/utils/shoots/update/highavailability"
)

const jobName = "update"

// Job represents the zero-downtime validator job.
type Job struct {
	job *batchv1.Job
}

// ItShouldDeployJob deploys the zero-downtime validator job to ensure no API server downtime while upgrading Gardener.
func (j *Job) ItShouldDeployJob(s *ShootContext) {
	GinkgoHelper()

	It("Deploy zero-downtime validator job to ensure no API server downtime while upgrading Gardener", func(ctx SpecContext) {
		By("Fetch kube-apiserver auth token")
		deployment := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.DeploymentNameKubeAPIServer, Namespace: s.Shoot.Status.TechnicalID}}
		Eventually(s.SeedKomega.Get(deployment)).Should(Succeed())
		authToken := deployment.Spec.Template.Spec.Containers[0].ReadinessProbe.HTTPGet.HTTPHeaders[0].Value

		By("Deploy job with name " + jobName)
		Eventually(ctx, func() error {
			var err error
			j.job, err = highavailability.DeployZeroDownTimeValidatorJob(ctx, s.SeedClient, jobName, s.Shoot.Status.TechnicalID, authToken)
			return err
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))
}

// ItShouldWaitForJobToBeReady waits until the zero-downtime validator job is ready.
func (j *Job) ItShouldWaitForJobToBeReady(s *ShootContext) {
	GinkgoHelper()

	It("Wait until zero-downtime validator job is ready", func(ctx SpecContext) {
		shootupdatesuite.WaitForJobToBeReady(ctx, s.SeedClient, j.job)
	}, SpecTimeout(5*time.Minute))
}

// ItShouldEnsureThereWasNoDowntime ensures there was no downtime while upgrading shoot by checking the job status.
func (j *Job) ItShouldEnsureThereWasNoDowntime(s *ShootContext) {
	GinkgoHelper()

	It("Ensure there was no downtime while upgrading shoot", func(ctx SpecContext) {
		j.initJobIfNeeded(s)
		Eventually(ctx, s.SeedKomega.Get(j.job)).Should(Succeed())
		Expect(j.job.Status.Failed).To(BeZero())
	}, SpecTimeout(time.Minute))
}

// AfterAllDeleteJob registers an 'AfterAll' node for deleting the zero-downtime validator job.
func (j *Job) AfterAllDeleteJob(s *ShootContext) {
	GinkgoHelper()

	AfterAll(func(ctx SpecContext) {
		j.initJobIfNeeded(s)
		Eventually(ctx, func() error {
			return s.SeedClient.Delete(ctx, j.job, client.PropagationPolicy(metav1.DeletePropagationForeground))
		}).Should(Or(Succeed(), BeNotFoundError()))
	}, NodeTimeout(time.Minute))
}

func (j *Job) initJobIfNeeded(s *ShootContext) {
	if j.job == nil {
		j.job = highavailability.EmptyZeroDownTimeValidatorJob(jobName, s.Shoot.Status.TechnicalID)
	}
}
