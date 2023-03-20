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
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"

	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
)

var _ = Describe("Apiservice", func() {
	DescribeTable("#CheckAPIService",
		func(apiService *apiregistrationv1.APIService, matcher types.GomegaMatcher) {
			err := health.CheckAPIService(apiService)
			Expect(err).To(matcher)
		},
		Entry("Available=True", &apiregistrationv1.APIService{
			Status: apiregistrationv1.APIServiceStatus{Conditions: []apiregistrationv1.APIServiceCondition{{Type: apiregistrationv1.Available, Status: apiregistrationv1.ConditionTrue}}},
		}, BeNil()),
		Entry("Available=False", &apiregistrationv1.APIService{
			Status: apiregistrationv1.APIServiceStatus{Conditions: []apiregistrationv1.APIServiceCondition{{Type: apiregistrationv1.Available, Status: apiregistrationv1.ConditionFalse}}},
		}, HaveOccurred()),
		Entry("Available=Unknown", &apiregistrationv1.APIService{
			Status: apiregistrationv1.APIServiceStatus{Conditions: []apiregistrationv1.APIServiceCondition{{Type: apiregistrationv1.Available, Status: apiregistrationv1.ConditionUnknown}}},
		}, HaveOccurred()),
		Entry("Available condition missing", &apiregistrationv1.APIService{}, HaveOccurred()),
	)
})
