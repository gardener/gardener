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
