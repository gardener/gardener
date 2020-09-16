// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package general

import (
	"testing"

	resourcesv1alpha1 "github.com/gardener/gardener-resource-manager/pkg/apis/resources/v1alpha1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestHealth(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Health Suite")
}

var _ = Describe("health", func() {
	Context("CheckMachineDeployment", func() {
		DescribeTable("machine deployments",
			func(managedResource *resourcesv1alpha1.ManagedResource, matcher types.GomegaMatcher) {
				err := checkManagedResourceIsHealthy(managedResource)
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
})
