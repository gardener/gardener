// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

var _ = Describe("Daemonset", func() {
	oneUnavailable := intstr.FromInt(1)

	DescribeTable("#CheckDaemonSet",
		func(daemonSet *appsv1.DaemonSet, matcher types.GomegaMatcher) {
			err := health.CheckDaemonSet(daemonSet)
			Expect(err).To(matcher)
		},
		Entry("not observed at latest version", &appsv1.DaemonSet{
			ObjectMeta: metav1.ObjectMeta{Generation: 1},
		}, HaveOccurred()),
		Entry("not enough scheduled", &appsv1.DaemonSet{
			Status: appsv1.DaemonSetStatus{DesiredNumberScheduled: 1},
		}, HaveOccurred()),
		Entry("misscheduled pods", &appsv1.DaemonSet{
			Status: appsv1.DaemonSetStatus{NumberMisscheduled: 1},
		}, HaveOccurred()),
		Entry("too many unavailable pods", &appsv1.DaemonSet{
			Spec: appsv1.DaemonSetSpec{UpdateStrategy: appsv1.DaemonSetUpdateStrategy{
				Type: appsv1.RollingUpdateDaemonSetStrategyType,
				RollingUpdate: &appsv1.RollingUpdateDaemonSet{
					MaxUnavailable: &oneUnavailable,
				},
			}},
			Status: appsv1.DaemonSetStatus{
				DesiredNumberScheduled: 2,
				CurrentNumberScheduled: 2,
				NumberUnavailable:      2,
				NumberReady:            0,
			},
		}, HaveOccurred()),
		Entry("too less ready pods", &appsv1.DaemonSet{
			Status: appsv1.DaemonSetStatus{
				DesiredNumberScheduled: 1,
				CurrentNumberScheduled: 1,
			},
		}, HaveOccurred()),
		Entry("healthy", &appsv1.DaemonSet{
			Status: appsv1.DaemonSetStatus{
				DesiredNumberScheduled: 1,
				CurrentNumberScheduled: 1,
				NumberReady:            1,
			},
		}, BeNil()),
	)
})
