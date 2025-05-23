// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

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
