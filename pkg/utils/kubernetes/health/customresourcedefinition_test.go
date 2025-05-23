// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package health_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
)

var _ = Describe("Customresourcedefinition", func() {
	Context("CheckCustomResourceDefinition", func() {
		DescribeTable("crds",
			func(crd *apiextensionsv1.CustomResourceDefinition, matcher types.GomegaMatcher) {
				err := health.CheckCustomResourceDefinition(crd)
				Expect(err).To(matcher)
			},
			Entry("terminating", &apiextensionsv1.CustomResourceDefinition{
				Status: apiextensionsv1.CustomResourceDefinitionStatus{
					Conditions: []apiextensionsv1.CustomResourceDefinitionCondition{
						{
							Type:   apiextensionsv1.NamesAccepted,
							Status: apiextensionsv1.ConditionTrue,
						},
						{
							Type:   apiextensionsv1.Established,
							Status: apiextensionsv1.ConditionTrue,
						},
						{
							Type:   apiextensionsv1.Terminating,
							Status: apiextensionsv1.ConditionTrue,
						},
					},
				},
			}, HaveOccurred()),
			Entry("with conflicting name", &apiextensionsv1.CustomResourceDefinition{
				Status: apiextensionsv1.CustomResourceDefinitionStatus{
					Conditions: []apiextensionsv1.CustomResourceDefinitionCondition{
						{
							Type:   apiextensionsv1.NamesAccepted,
							Status: apiextensionsv1.ConditionFalse,
						},
						{
							Type:   apiextensionsv1.Established,
							Status: apiextensionsv1.ConditionFalse,
						},
					},
				},
			}, HaveOccurred()),
			Entry("healthy", &apiextensionsv1.CustomResourceDefinition{
				Status: apiextensionsv1.CustomResourceDefinitionStatus{
					Conditions: []apiextensionsv1.CustomResourceDefinitionCondition{
						{
							Type:   apiextensionsv1.NamesAccepted,
							Status: apiextensionsv1.ConditionTrue,
						},
						{
							Type:   apiextensionsv1.Established,
							Status: apiextensionsv1.ConditionTrue,
						},
					},
				},
			}, BeNil()),
		)
	})
})
