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
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Managedresource", func() {
	DescribeTable("#CheckManagedResource",
		func(managedResource *resourcesv1alpha1.ManagedResource, matcher types.GomegaMatcher) {
			err := health.CheckManagedResource(managedResource)
			Expect(err).To(matcher)
		},
		Entry("healthy", &resourcesv1alpha1.ManagedResource{
			Status: resourcesv1alpha1.ManagedResourceStatus{Conditions: []resourcesv1alpha1.ManagedResourceCondition{
				{
					Type:   resourcesv1alpha1.ResourcesHealthy,
					Status: resourcesv1alpha1.ConditionTrue,
				},
				{
					Type:   resourcesv1alpha1.ResourcesApplied,
					Status: resourcesv1alpha1.ConditionTrue,
				},
			}},
		}, BeNil()),
		Entry("unhealthy without available", &resourcesv1alpha1.ManagedResource{}, HaveOccurred()),
		Entry("unhealthy with false available", &resourcesv1alpha1.ManagedResource{
			Status: resourcesv1alpha1.ManagedResourceStatus{Conditions: []resourcesv1alpha1.ManagedResourceCondition{
				{
					Type:   resourcesv1alpha1.ResourcesHealthy,
					Status: resourcesv1alpha1.ConditionTrue,
				},
				{
					Type:   resourcesv1alpha1.ResourcesApplied,
					Status: resourcesv1alpha1.ConditionFalse,
				},
			}},
		}, HaveOccurred()),
		Entry("not observed at latest version", &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{Generation: 1},
		}, HaveOccurred()),
	)
})
