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
