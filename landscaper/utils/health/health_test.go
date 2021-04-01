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
	landscaperv1alpha1 "github.com/gardener/landscaper/apis/core/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/gardener/gardener/landscaper/utils/health"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("health", func() {
	var installation *landscaperv1alpha1.Installation

	BeforeEach(func() {
		installation = &landscaperv1alpha1.Installation{
			ObjectMeta: metav1.ObjectMeta{
				Generation: 1,
			},
			Status: landscaperv1alpha1.InstallationStatus{
				Phase:              landscaperv1alpha1.ComponentPhaseSucceeded,
				ObservedGeneration: 1,
				Conditions: []landscaperv1alpha1.Condition{
					{
						Type:   landscaperv1alpha1.EnsureSubInstallationsCondition,
						Status: landscaperv1alpha1.ConditionTrue,
					},
					{
						Type:   landscaperv1alpha1.ReconcileExecutionCondition,
						Status: landscaperv1alpha1.ConditionTrue,
					},
					{
						Type:   landscaperv1alpha1.ValidateImportsCondition,
						Status: landscaperv1alpha1.ConditionFalse,
					},
					{
						Type:   landscaperv1alpha1.CreateImportsCondition,
						Status: landscaperv1alpha1.ConditionTrue,
					},
					{
						Type:   landscaperv1alpha1.CreateExportsCondition,
						Status: landscaperv1alpha1.ConditionTrue,
					},
					{
						Type:   landscaperv1alpha1.EnsureExecutionsCondition,
						Status: landscaperv1alpha1.ConditionTrue,
					},
					{
						Type:   landscaperv1alpha1.ValidateExportCondition,
						Status: landscaperv1alpha1.ConditionTrue,
					},
				},
			},
		}
	})

	Describe("#CheckInstallation", func() {
		It("installation is healthy", func() {
			Expect(CheckInstallation(installation)).NotTo(HaveOccurred())
		})

		It("installation not healthy - observed generation outdated", func() {
			installation.Status.ObservedGeneration = 0
			Expect(CheckInstallation(installation)).To(HaveOccurred())
		})

		It("installation not healthy - phase is not succeeded", func() {
			installation.Status.Phase = landscaperv1alpha1.ComponentPhaseFailed
			Expect(CheckInstallation(installation)).To(HaveOccurred())
		})

		It("installation not healthy - not all conditions are healthy", func() {
			installation.Status.Conditions[0].Status = landscaperv1alpha1.ConditionFalse
			Expect(CheckInstallation(installation)).To(HaveOccurred())
		})

		It("installation healthy - even though there is an additional condition", func() {
			installation.Status.Conditions[0].Type = landscaperv1alpha1.ConditionType("abc")
			Expect(CheckInstallation(installation)).ToNot(HaveOccurred())
		})

		It("installation not healthy - operation annotation not yet picked up", func() {
			installation.ObjectMeta.Annotations = map[string]string{
				landscaperv1alpha1.OperationAnnotation: "xzy",
			}
			Expect(CheckInstallation(installation)).To(HaveOccurred())
		})

		It("installation not healthy - last error is set", func() {
			installation.Status.LastError = &landscaperv1alpha1.Error{}
			Expect(CheckInstallation(installation)).To(HaveOccurred())
		})
	})
})
