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

package health_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
)

var _ = Describe("StatefulSet", func() {
	DescribeTable("CheckStatefulSet",
		func(statefulSet *appsv1.StatefulSet, matcher types.GomegaMatcher) {
			err := health.CheckStatefulSet(statefulSet)
			Expect(err).To(matcher)
		},
		Entry("healthy", &appsv1.StatefulSet{
			Spec:   appsv1.StatefulSetSpec{Replicas: pointer.Int32(1)},
			Status: appsv1.StatefulSetStatus{CurrentReplicas: 1, ReadyReplicas: 1},
		}, BeNil()),
		Entry("healthy with nil replicas", &appsv1.StatefulSet{
			Status: appsv1.StatefulSetStatus{ReadyReplicas: 1},
		}, BeNil()),
		Entry("not observed at latest version", &appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{Generation: 1},
		}, HaveOccurred()),
		Entry("not enough ready replicas", &appsv1.StatefulSet{
			Spec:   appsv1.StatefulSetSpec{Replicas: pointer.Int32(2)},
			Status: appsv1.StatefulSetStatus{ReadyReplicas: 1},
		}, HaveOccurred()),
	)

	Describe("IsStatefulSetProgressing", func() {
		var (
			statefulSet *appsv1.StatefulSet
		)

		BeforeEach(func() {
			statefulSet = &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Generation: 42,
				},
				Spec: appsv1.StatefulSetSpec{
					Replicas: pointer.Int32(3),
				},
				Status: appsv1.StatefulSetStatus{
					ObservedGeneration: 42,
					UpdatedReplicas:    3,
				},
			}
		})

		It("should return false if it is fully rolled out", func() {
			progressing, reason := health.IsStatefulSetProgressing(statefulSet)
			Expect(progressing).To(BeFalse())
			Expect(reason).To(Equal("StatefulSet is fully rolled out"))
		})

		It("should return true if observedGeneration is outdated", func() {
			statefulSet.Status.ObservedGeneration--

			progressing, reason := health.IsStatefulSetProgressing(statefulSet)
			Expect(progressing).To(BeTrue())
			Expect(reason).To(Equal("observed generation outdated (41/42)"))
		})

		It("should return true if replicas still need to be updated", func() {
			statefulSet.Status.UpdatedReplicas--

			progressing, reason := health.IsStatefulSetProgressing(statefulSet)
			Expect(progressing).To(BeTrue())
			Expect(reason).To(Equal("2 of 3 replica(s) have been updated"))
		})

		It("should return true if replica still needs to be updated (spec.replicas=null)", func() {
			statefulSet.Spec.Replicas = nil
			statefulSet.Status.UpdatedReplicas = 0

			progressing, reason := health.IsStatefulSetProgressing(statefulSet)
			Expect(progressing).To(BeTrue())
			Expect(reason).To(Equal("0 of 1 replica(s) have been updated"))
		})
	})
})
