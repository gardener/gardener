// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package health_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"

	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
)

var _ = Describe("CheckJob", func() {
	It("should not return an error if JobFailed is missing", func() {
		job := &batchv1.Job{}
		Expect(health.CheckJob(job)).To(Succeed())
	})

	It("should not return an error if JobFailed is False", func() {
		job := &batchv1.Job{Status: batchv1.JobStatus{
			Conditions: []batchv1.JobCondition{{
				Type:   batchv1.JobFailed,
				Status: corev1.ConditionFalse,
			}},
		}}
		Expect(health.CheckJob(job)).To(Succeed())
	})

	It("should return an error if JobFailed is True", func() {
		job := &batchv1.Job{Status: batchv1.JobStatus{
			Conditions: []batchv1.JobCondition{{
				Type:   batchv1.JobFailed,
				Status: corev1.ConditionTrue,
			}},
		}}
		Expect(health.CheckJob(job)).To(MatchError(ContainSubstring(`condition "Failed"`)))
	})
})
