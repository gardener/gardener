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

package podschedulername_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"

	. "github.com/gardener/gardener/pkg/resourcemanager/webhook/podschedulername"
)

var _ = Describe("Handler", func() {
	var (
		ctx = context.TODO()

		handler *Handler
		pod     *corev1.Pod
	)

	BeforeEach(func() {
		handler = &Handler{SchedulerName: "foo-scheduler"}
		pod = &corev1.Pod{}
	})

	Describe("#Default", func() {
		It("should not patch the scheduler name when the pod specifies custom scheduler", func() {
			schedulerName := "bar-scheduler"
			pod.Spec.SchedulerName = schedulerName

			Expect(handler.Default(ctx, pod)).To(Succeed())
			Expect(pod.Spec.SchedulerName).To(Equal(schedulerName))
		})

		It("should patch the scheduler name when the pod scheduler is not specified", func() {
			pod.Spec.SchedulerName = ""

			Expect(handler.Default(ctx, pod)).To(Succeed())
			Expect(pod.Spec.SchedulerName).To(Equal(handler.SchedulerName))
		})
	})
})
